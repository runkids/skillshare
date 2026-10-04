import { useState } from 'react';
import { Pencil, Plus, PowerOff, Trash2 } from 'lucide-react';
import { mcpApi, mcpOffTargets, mcpTargets, type MCPMutation, type MCPServer } from '../../api/mcp';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import ConfirmDialog from '../ConfirmDialog';
import { RailLayout } from '../StatusRail';
import { SkillContextMenu, type ContextMenuItem } from '../TargetMenu';
import { useToast } from '../Toast';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { ProjectTargets } from './MCPProjectSettings';
import MCPCheckNote from './MCPCheckNote';
import MCPImportDialog from './MCPImportDialog';
import MCPRemoveDialog from './MCPRemoveDialog';
import MCPServerDialog from './MCPServerDialog';
import MCPServerList from './MCPServerList';
import MCPSyncBox, { MCPRailActions } from './MCPSyncBox';
import MCPUnmanagedNote from './MCPUnmanagedNote';
import { TargetPill } from './TargetPicker';
import { problemsByServer, useMCPCheck } from './useMCPCheck';
import { buildMatrix, describeEndpoint, describeError, projectOf, switchTargets, targetLabel, writes } from './mcpView';

type MCPList = Awaited<ReturnType<typeof mcpApi.list>>;

interface Props {
  data: MCPList;
  root: string;
  offered: readonly string[];
  onChanged: () => void;
  onRemoved: () => void;
}

function projectViewModel(data: MCPList, root: string, offered: readonly string[]) {
  const project = data.source.projects?.[root] ?? {};
  const globals = data.source.servers;
  const defaults = data.source.targets ?? [];
  const targets = project.targets ?? defaults;
  const servers = project.servers ?? {};
  // A global server no Agent receives is active nowhere, so a project has nothing to turn off.
  const shownGlobals = Object.keys(globals).filter((n) => servers[n] || globals[n].targets?.length !== 0).sort();
  const roots = Object.keys(data.source.projects ?? {});
  const changes = (data.plan?.changes ?? []).filter((c) => projectOf(roots, c) === root);
  const name = shortenHome(root);
  const unmanaged = data.unmanaged.filter((u) => u.project === root);
  // A switch for a global server belongs to the list above; everything else is the project's own.
  const own = Object.fromEntries(Object.entries(servers).filter(([n, s]) => !(s.disabled && globals[n])));
  // A switch that names no targets follows the project.
  const targetsOf = (n: string) => own[n]?.targets ?? targets;
  const offTargets = mcpOffTargets;
  const switchable = (server: MCPServer) => switchTargets(server, defaults, targets);
  const ownRows = buildMatrix(own, data.plan && { ...data.plan, changes: changes.filter((c) => own[c.name]) });
  const shown = mcpTargets.filter((x) => offered.includes(x) || ownRows.some((row) => targetsOf(row.name).includes(x)));
  return { project, globals, defaults, targets, servers, shownGlobals, roots, changes, name, unmanaged, own, targetsOf, offTargets, switchable, ownRows, shown };
}

type ProjectModel = ReturnType<typeof projectViewModel>;

function ProjectSettings({ model, offered, pickTargets, onPickTargets, busy, save }: { model: ProjectModel; offered: readonly string[]; pickTargets: boolean; onPickTargets: () => void; busy: boolean; save: (mutation: MCPMutation) => Promise<boolean> }) {
  const t = useT();
  const { project, targets, defaults, name } = model;
  return (
    <div className="ss-box flex flex-col gap-3.5">
      <dl className="ss-kv !grid-cols-[auto_minmax(0,1fr)] items-center">
        <dt>{t('mcp.targets')}</dt>
        <dd><TargetPill selected={targets} text={project.targets ? `${targets.length}/${offered.length}` : t('mcp.projects.inherit')} expanded={pickTargets} label={t('mcp.chooseAgents', { name })} onClick={() => onPickTargets()} /></dd>
        {/* In the value column, so the expanded control lines up under the pill that opened it. */}
        {pickTargets && <dd className="col-start-2 !font-normal"><ProjectTargets value={project.targets} defaults={defaults} offered={offered} disabled={busy} onChange={(next) => void save({ replace: true, settings: { targets: next } })} /></dd>}
      </dl>
    </div>
  );
}

function ProjectServerEditor({ data, model, root, offered, editing, addingOff, addMode, onMode, onClose, onSaved }: { data: MCPList; model: ProjectModel; root: string; offered: readonly string[]; editing: string; addingOff: boolean; addMode: 'form' | 'paste'; onMode: (mode: 'form' | 'paste') => void; onClose: () => void; onSaved: () => void }) {
  const { servers, targets, own } = model;
  return (
    editing === '' && addMode === 'paste' ? (
      <MCPImportDialog
        source="paste"
        project={root}
        servers={servers}
        defaultTargets={targets}
        availableTargets={offered}
        paths={data.paths}
        detected={data.detected}
        onMode={onMode}
        onClose={onClose}
        onImported={onSaved}
      />
    ) : (
      <MCPServerDialog
        project={root}
        off={editing === '' && addingOff}
        initial={editing ? { name: editing, server: own[editing] } : undefined}
        defaultTargets={targets}
        existingNames={Object.keys(servers)}
        availableTargets={offered}
        onMode={editing === '' ? onMode : undefined}
        onClose={onClose}
        onSaved={onSaved}
      />
    )
  );
}

/** One root under mcp.projects: its defaults, the global servers it turns off, and servers of its own. */
export default function MCPProjectView({ data, root, offered, onChanged, onRemoved }: Props) {
  const t = useT();
  const { toast } = useToast();
  const [pickTargets, setPickTargets] = useState(false);
  const [editing, setEditing] = useState<string | null>(null); // '' adds a new server
  const [addMode, setAddMode] = useState<'form' | 'paste'>('form');
  const [addingOff, setAddingOff] = useState(false); // the new entry is a switch, not a server
  const [removing, setRemoving] = useState('');
  const [dropping, setDropping] = useState(false);
  const [importFrom, setImportFrom] = useState(''); // an Agent file of this project to import from
  const [busy, setBusy] = useState(false);
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);
  const check = useMCPCheck();

  const model = projectViewModel(data, root, offered);
  const { globals, defaults, targets, servers, shownGlobals, roots, changes, name, unmanaged, own, targetsOf, offTargets, switchable, ownRows, shown } = model;

  const save = async (mutation: MCPMutation) => {
    setBusy(true);
    try {
      await mcpApi.save({ ...mutation, project: root });
      onChanged();
      return true;
    } catch (e) {
      toast(describeError(t, (e as Error).message), 'error');
      return false;
    } finally {
      setBusy(false);
    }
  };

  /** Agents where this global server can be turned off from here. */

  // The switch names no targets: sync works out where it goes from the project's targets at
  // that moment. A stored list went stale as soon as those changed.
  const toggleGlobal = (n: string, on: boolean) => save(on ? { name: n, remove: true } : { name: n, replace: true, server: { disabled: true } });

  const toggleOwn = (n: string, target: string, on: boolean) => {
    const next = mcpTargets.filter((x) => (x === target ? on : targetsOf(n).includes(x)));
    // A switch-only entry needs an Agent to turn the server off for; a server may have none.
    if (next.length === 0 && own[n].disabled) return toast(t('mcp.needTarget'), 'warning');
    void save({ name: n, replace: true, server: { ...own[n], targets: next } }).then((saved) => { if (saved && next.length === 0) toast(t('mcp.noTargetsToast', { name: n }), 'info'); });
  };

  const openMenu = (e: React.MouseEvent<HTMLButtonElement>, n: string) => {
    const r = e.currentTarget.getBoundingClientRect();
    setMenu({ x: r.left, y: r.bottom + 4, items: [
      { key: 'edit', label: t('mcp.edit'), icon: <Pencil size={14} />, onSelect: () => setEditing(n) },
      { key: 'remove', label: t('mcp.remove'), icon: <Trash2 size={14} />, danger: true, onSelect: () => setRemoving(n) },
    ] });
  };

  const drop = async () => {
    setBusy(true);
    try {
      await mcpApi.save({ project: root, remove: true });
      onRemoved();
    } catch (e) {
      toast(describeError(t, (e as Error).message), 'error');
      setBusy(false);
    }
  };

  return (
    <div>
      <RailLayout pageScroll rail={data.plan
        ? <MCPSyncBox changes={changes} roots={roots} plan={data.plan} check={ownRows.length > 0 ? check : undefined} />
        : ownRows.length > 0 && <div className="ss-box"><MCPRailActions check={check} /></div>}>
        <ProjectSettings model={model} offered={offered} pickTargets={pickTargets} onPickTargets={() => setPickTargets(!pickTargets)} busy={busy} save={save} />

        {ownRows.length > 0 && <div className="mt-3 empty:hidden"><MCPCheckNote report={check.report} checkedAt={check.checkedAt} error={check.error} running={check.running} onRun={() => void check.run()} project={root} /></div>}
        <div className="mt-3 empty:hidden"><MCPUnmanagedNote entries={unmanaged} onImport={setImportFrom} /></div>

        <section className="mt-3 flex flex-col">
          <div className="ss-sec"><h2>{t('mcp.projects.globalServers')}</h2><span className="ss-cnt">{shownGlobals.length}</span><span className="text-[13px] text-ink-2">{t('mcp.projects.globalHint')}</span></div>
          {shownGlobals.length > 0 ? (
            <div className="ss-list">
              {shownGlobals.map((n) => {
                const server = globals[n];
                const entry = servers[n];
                const off = Boolean(entry?.disabled);
                const to = switchable(server);
                // Off shows as Agent logos. A sentence is kept for the switch that cannot be used, which needs a reason.
                const written = (entry?.targets ?? to).filter((x) => mcpOffTargets.includes(x));
                const reason = entry && !off ? t('mcp.projects.overridden')
                  : off ? (written.length === 0 ? t('mcp.projects.noSwitchHere') : '')
                  : to.length > 0 ? ''
                  // Some Agent could turn it off, just none this project uses.
                  : switchTargets(server, defaults, mcpOffTargets).length > 0 ? t('mcp.projects.noSwitchHere') : t('mcp.projects.noSwitch');
                // What the page has to admit: the badge says off, yet an Agent without a switch keeps
                // loading the server here, and a list saved with the entry does not follow the project.
                const stillOn = off ? targets.filter((x) => (server.targets ?? defaults).includes(x) && !written.includes(x)) : [];
                const stale = off && entry.targets && [...entry.targets].sort().join() !== [...to].sort().join();
                return (
                  <div key={n} className="ss-r !items-stretch !flex-col !gap-2 !py-3">
                    <span className="flex min-w-0 items-center gap-2">
                      <span title={n} className="max-w-[45%] shrink-0 truncate font-mono font-semibold">{n}</span>
                      {off && <span className="ss-tag warn">{t('mcp.projects.offBadge')}</span>}
                      {changes.some((c) => c.name === n && writes(c)) && <span className="ss-tag warn">{t('plugins.pending')}</span>}
                      <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3">{describeEndpoint(server)}</span>
                      {off && written.length > 0 && (
                        <span className="ss-stack" role="img" aria-label={t('mcp.projects.offIn', { targets: written.map(targetLabel).join(', ') })} title={t('mcp.projects.offIn', { targets: written.map(targetLabel).join(', ') })}>
                          {written.map((x) => <span key={x} className="ss-at"><AgentIcon target={x} size={13} /></span>)}
                        </span>
                      )}
                      <button type="button" role="switch" aria-checked={!off} aria-label={n} aria-describedby={`mcp-sw-${n}`} className={`ss-sw ${off ? '' : 'on'} disabled:opacity-50`} disabled={busy || (!off && to.length === 0) || Boolean(entry && !off)} onClick={() => void toggleGlobal(n, off)}><i /></button>
                    </span>
                    <span id={`mcp-sw-${n}`} className="flex flex-col text-xs empty:hidden">
                      {reason && <span className="text-ink-3">{reason}</span>}
                      {stillOn.length > 0 && <span className="text-warn">{t('mcp.projects.stillOn', { targets: stillOn.map(targetLabel).join(', ') })}</span>}
                      {stale && <span className="text-warn">{t('mcp.projects.staleSwitch')} <button type="button" className="underline disabled:opacity-50" disabled={busy} onClick={() => void toggleGlobal(n, false)}>{t('mcp.projects.matchProject')}</button></span>}
                    </span>
                  </div>
                );
              })}
            </div>
          ) : <p className="text-[13px] text-ink-3">{t('mcp.empty')}</p>}
        </section>

        <section className="mt-3 flex flex-col">
          {/* Two named actions rather than one button that then asks which it was. */}
          <div className="ss-sec !items-center"><h2>{t('mcp.projects.onlyHere')}</h2><span className="ss-cnt">{ownRows.length}</span>
            <Button className="ml-auto" size="sm" variant="ghost" onClick={() => { setAddingOff(true); setAddMode('form'); setEditing(''); }}><PowerOff size={14} />{t('mcp.addOff')}</Button>
            <Button size="sm" variant="secondary" onClick={() => { setAddingOff(false); setAddMode('form'); setEditing(''); }}><Plus size={14} />{t('mcp.addServer')}</Button>
          </div>
          {ownRows.length > 0
            ? <MCPServerList rows={ownRows} targets={shown} targetsOf={targetsOf} offTargets={offTargets} onToggle={toggleOwn} onMenu={openMenu} disabled={busy} problems={problemsByServer(check.report, root)} />
            : <p className="text-[13px] text-ink-3">{t('mcp.projects.noOnlyHere')}</p>}
        </section>
        <Button className="flush mt-6 self-start" size="sm" variant="ghost" onClick={() => setDropping(true)}><Trash2 size={14} />{t('projects.mcp.stop')}</Button>
      </RailLayout>

      {editing !== null && (<ProjectServerEditor data={data} model={model} root={root} offered={offered} editing={editing} addingOff={addingOff} addMode={addMode} onMode={setAddMode} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); onChanged(); toast(t('mcp.toast.saved'), 'success'); }} />
      )}
      {importFrom && (
        <MCPImportDialog
          source="target"
          project={root}
          importSources={data.importSources?.[root]}
          defaultPath={unmanaged.find((u) => u.target === importFrom)?.path}
          servers={servers}
          defaultTargets={targets}
          availableTargets={offered}
          paths={Object.fromEntries(unmanaged.map((u) => [u.target, u.path]))}
          detected={unmanaged.map((u) => u.target)}
          defaultFrom={importFrom}
          onClose={() => setImportFrom('')}
          onImported={() => { setImportFrom(''); onChanged(); }}
        />
      )}
      {removing && <MCPRemoveDialog name={removing} project={root} inScope={(c) => projectOf(roots, c) === root} onClose={() => setRemoving('')} onSaved={(unmanaged) => { const n = removing; setRemoving(''); onChanged(); toast(t(unmanaged ? 'mcp.toast.unmanaged' : 'mcp.toast.removed', { name: n }), 'success'); }} />}
      <ConfirmDialog open={dropping} variant="danger" loading={busy} title={t('projects.mcp.stopTitle', { name })} message={t('projects.mcp.stopMessage')} confirmText={t('projects.mcp.stop')} onCancel={() => setDropping(false)} onConfirm={() => void drop()} />
      <SkillContextMenu open={!!menu} anchorPoint={menu ?? undefined} items={menu?.items ?? []} onClose={() => setMenu(null)} />
    </div>
  );
}
