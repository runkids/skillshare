import { useState } from 'react';
import { Folder, Globe, Pencil, Plus, PowerOff, Trash2, Workflow } from 'lucide-react';
import {
  isShared,
  mcpApi,
  mcpOffTargets,
  mcpTargets,
  type MCPMutation,
  type MCPServer,
} from '../../api/mcp';
import Button from '../Button';
import ConfirmDialog from '../ConfirmDialog';
import Reveal from '../Reveal';
import { RailLayout, RailRow, RailSection } from '../StatusRail';
import { SkillContextMenu, type ContextMenuItem } from '../TargetMenu';
import Tooltip from '../Tooltip';
import { useToast } from '../Toast';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { ProjectTargets } from './MCPProjectSettings';
import MCPCheckNote from './MCPCheckNote';
import MCPFileActions from './MCPFileActions';
import MCPFlowDialog from './MCPFlowDialog';
import MCPImportDialog from './MCPImportDialog';
import MCPRemoveDialog from './MCPRemoveDialog';
import MCPServerDialog from './MCPServerDialog';
import { copyableServers, isServeCommand, type AddMode } from './mcpServerDraft';
import MCPServerList from './MCPServerList';
import MCPOverrideDialog from './MCPOverrideDialog';
import MCPSharedServers from './MCPSharedServers';
import ProjectSectionHead from './ProjectSectionHead';
import ProjectServerRow from './ProjectServerRow';
import MCPSyncBox, { MCPRailActions } from './MCPSyncBox';
import MCPUnmanagedNote from './MCPUnmanagedNote';
import { TargetPill } from './TargetPicker';
import { problemsByServer, useMCPCheck } from './useMCPCheck';
import {
  buildMatrix,
  describeError,
  projectFileTargets,
  projectOf,
  sharedUsers,
  switchTargets,
  targetLabel,
  writes,
} from './mcpView';

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
  // A shared server never reaches the global files, so there is nothing to turn off here.
  const globals = Object.fromEntries(
    Object.entries(data.source.servers).filter(([, s]) => !isShared(s)),
  );
  const users = sharedUsers(data.source);
  const use = project.use ?? [];
  const defaults = data.source.targets ?? [];
  const targets = project.targets ?? defaults;
  const servers = project.servers ?? {};
  // A global server no Agent receives is active nowhere, so a project has nothing to turn off.
  const shownGlobals = Object.keys(globals)
    .filter((n) => servers[n] || globals[n].targets?.length !== 0)
    .sort();
  const roots = Object.keys(data.source.projects ?? {});
  const changes = (data.plan?.changes ?? []).filter((c) => projectOf(roots, c) === root);
  const name = shortenHome(root);
  const unmanaged = data.unmanaged.filter((u) => u.project === root);
  // A switch for a global server belongs to the list above; everything else is the project's own.
  const own = Object.fromEntries(
    Object.entries(servers).filter(([n, s]) => !(s.disabled && globals[n])),
  );
  // A switch that names no targets follows the project.
  const targetsOf = (n: string) => own[n]?.targets ?? targets;
  const offTargets = mcpOffTargets;
  const switchable = (server: MCPServer) => switchTargets(server, defaults, targets);
  const ownRows = buildMatrix(
    own,
    data.plan && { ...data.plan, changes: changes.filter((c) => own[c.name]) },
  );
  const shown = mcpTargets.filter(
    (x) => offered.includes(x) || ownRows.some((row) => targetsOf(row.name).includes(x)),
  );
  return {
    project,
    globals,
    users,
    use,
    defaults,
    targets,
    servers,
    shownGlobals,
    roots,
    changes,
    name,
    unmanaged,
    own,
    targetsOf,
    offTargets,
    switchable,
    ownRows,
    shown,
  };
}

type ProjectModel = ReturnType<typeof projectViewModel>;

// The rail lists this many Agent files before the rest fold behind a button.
const FILES_SHOWN = 5;

function ProjectSettings({
  model,
  offered,
  pickTargets,
  onPickTargets,
  busy,
  save,
}: {
  model: ProjectModel;
  offered: readonly string[];
  pickTargets: boolean;
  onPickTargets: () => void;
  busy: boolean;
  save: (mutation: MCPMutation) => Promise<boolean>;
}) {
  const t = useT();
  const { project, targets, defaults, name } = model;
  return (
    <div className="ss-box !p-0">
      {/* Laid out like the global MCP defaults: the setting and its pill on one row, its toggles under it. */}
      <div className="ss-setrow">
        <div className="l">
          <b>{t('mcp.targets')}</b>
          <span>{t('mcp.defaults.targetsHint')}</span>
        </div>
        <TargetPill
          selected={targets}
          text={project.targets ? `${targets.length}/${offered.length}` : t('mcp.projects.inherit')}
          expanded={pickTargets}
          label={t('mcp.chooseAgents', { name })}
          onClick={() => onPickTargets()}
        />
      </div>
      <Reveal open={pickTargets}>
        <div className="px-[18px] pb-4">
          <ProjectTargets
            value={project.targets}
            defaults={defaults}
            offered={offered}
            disabled={busy}
            onChange={(next) => save({ replace: true, settings: { targets: next } })}
          />
        </div>
      </Reveal>
    </div>
  );
}

function ProjectServerEditor({
  data,
  model,
  root,
  offered,
  editing,
  addingOff,
  addMode,
  onMode,
  onClose,
  onSaved,
}: {
  data: MCPList;
  model: ProjectModel;
  root: string;
  offered: readonly string[];
  editing: string;
  addingOff: boolean;
  addMode: AddMode;
  onMode: (mode: AddMode) => void;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { servers, targets, own, use } = model;
  const server = editing ? own[editing] : undefined;
  const serve = editing === '' ? addMode === 'serve' : isServeCommand(server);
  const copyFrom = copyableServers(data.source.projects, root);
  return editing === '' && addMode === 'paste' ? (
    <MCPImportDialog
      source="paste"
      project={root}
      servers={servers}
      defaultTargets={targets}
      availableTargets={offered}
      paths={data.paths}
      detected={data.detected}
      onMode={onMode}
      copyable={copyFrom.length > 0}
      onClose={onClose}
      onImported={onSaved}
    />
  ) : (
    <MCPServerDialog
      key={addMode}
      project={root}
      off={editing === '' && addingOff}
      serve={serve}
      copyFrom={copyFrom}
      copy={editing === '' && addMode === 'copy'}
      initial={editing && server ? { name: editing, server } : undefined}
      defaultTargets={targets}
      // A shared server this project uses holds its name here too.
      existingNames={[...Object.keys(servers), ...use]}
      availableTargets={offered}
      onMode={editing === '' ? onMode : undefined}
      onClose={onClose}
      onSaved={onSaved}
    />
  );
}

/** One root under mcp.projects: its defaults, the global servers it turns off, and servers of its own. */
export default function MCPProjectView({ data, root, offered, onChanged, onRemoved }: Props) {
  const t = useT();
  const { toast } = useToast();
  const [pickTargets, setPickTargets] = useState(false);
  const [editing, setEditing] = useState<string | null>(null); // '' adds a new server
  const [addMode, setAddMode] = useState<AddMode>('form');
  const [addingOff, setAddingOff] = useState(false); // the new entry is a switch, not a server
  const [removing, setRemoving] = useState('');
  const [overriding, setOverriding] = useState(''); // a shared server this project changes
  const [dropping, setDropping] = useState(false);
  const [importFrom, setImportFrom] = useState(''); // an Agent file of this project to import from
  const [busy, setBusy] = useState(false);
  const [showFlow, setShowFlow] = useState(false);
  const [allFiles, setAllFiles] = useState(false);
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);
  const check = useMCPCheck();

  const model = projectViewModel(data, root, offered);
  const {
    globals,
    users,
    use,
    defaults,
    targets,
    servers,
    shownGlobals,
    roots,
    changes,
    name,
    unmanaged,
    own,
    targetsOf,
    offTargets,
    switchable,
    ownRows,
    shown,
  } = model;
  const paths = data.projectPaths[root] ?? {};
  const existing = data.projectExisting[root] ?? [];
  const files = projectFileTargets(targets, own, existing, paths);

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
  const toggleGlobal = (n: string, on: boolean) =>
    save(on ? { name: n, remove: true } : { name: n, replace: true, server: { disabled: true } });

  // One name at a time, so a second switch, or another tab, never undoes the first.
  const toggleShared = (n: string, on: boolean) => save({ name: n, using: on });

  const toggleOwn = (n: string, target: string, on: boolean) => {
    const next = mcpTargets.filter((x) => (x === target ? on : targetsOf(n).includes(x)));
    // A switch-only entry needs an Agent to turn the server off for; a server may have none.
    if (next.length === 0 && own[n].disabled) return toast(t('mcp.needTarget'), 'warning');
    void save({ name: n, replace: true, server: { ...own[n], targets: next } }).then((saved) => {
      if (saved && next.length === 0) toast(t('mcp.noTargetsToast', { name: n }), 'info');
    });
  };

  const openMenu = (e: React.MouseEvent<HTMLButtonElement>, n: string) => {
    const r = e.currentTarget.getBoundingClientRect();
    setMenu({
      x: r.left,
      y: r.bottom + 4,
      items: [
        {
          key: 'edit',
          label: t('mcp.edit'),
          icon: <Pencil size={14} />,
          onSelect: () => setEditing(n),
        },
        {
          key: 'remove',
          label: t('mcp.remove'),
          icon: <Trash2 size={14} />,
          danger: true,
          onSelect: () => setRemoving(n),
        },
      ],
    });
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
      <RailLayout
        pageScroll
        rail={
          <>
            {data.plan ? (
              <MCPSyncBox
                changes={changes}
                roots={roots}
                plan={data.plan}
                byServer
                check={ownRows.length > 0 ? check : undefined}
              />
            ) : (
              ownRows.length > 0 && (
                <div className="ss-box">
                  <MCPRailActions check={check} />
                </div>
              )
            )}
            {files.length > 0 && (
              <RailSection title={t('layout.nav.agents')} count={files.length}>
                {(allFiles ? files : files.slice(0, FILES_SHOWN)).map((target) => (
                  <RailRow
                    key={target}
                    target={target}
                    label={targetLabel(target)}
                    right={
                      <MCPFileActions
                        target={target}
                        path={paths[target]}
                        local={data.local}
                        openable={data.local && existing.includes(target)}
                        project={root}
                      />
                    }
                  />
                ))}
                {files.length > FILES_SHOWN && (
                  <Button
                    variant="link"
                    className="self-start"
                    onClick={() => setAllFiles(!allFiles)}
                  >
                    {allFiles
                      ? t('mcp.projects.showFewer', { count: FILES_SHOWN })
                      : t('mcp.projects.showRest', { count: files.length - FILES_SHOWN })}
                  </Button>
                )}
              </RailSection>
            )}
          </>
        }
      >
        <ProjectSettings
          model={model}
          offered={offered}
          pickTargets={pickTargets}
          onPickTargets={() => setPickTargets(!pickTargets)}
          busy={busy}
          save={save}
        />

        {ownRows.length > 0 && (
          <div className="mt-3 empty:hidden">
            <MCPCheckNote
              report={check.report}
              checkedAt={check.checkedAt}
              error={check.error}
              running={check.running}
              onRun={() => void check.run()}
              project={root}
            />
          </div>
        )}
        <div className="mt-3 empty:hidden">
          <MCPUnmanagedNote entries={unmanaged} onImport={setImportFrom} />
        </div>

        <section className="mt-3 flex flex-col">
          <ProjectSectionHead
            icon={<Globe size={16} />}
            title={t('mcp.projects.globalServers')}
            count={shownGlobals.length}
            chip={t('mcp.projects.chipOn')}
            hint={t('mcp.projects.globalHint')}
          >
            <Button size="sm" variant="ghost" onClick={() => setShowFlow(true)}>
              <Workflow size={14} />
              {t('mcp.projects.flow')}
            </Button>
          </ProjectSectionHead>
          {shownGlobals.length > 0 ? (
            <div className="ss-list">
              {shownGlobals.map((n) => {
                const server = globals[n];
                const entry = servers[n];
                const off = Boolean(entry?.disabled);
                const to = switchable(server);
                // Agent logos say where it applies. A sentence is kept for the switch that cannot be used, which needs a reason.
                const written = (entry?.targets ?? to).filter((x) => mcpOffTargets.includes(x));
                const reason =
                  entry && !off
                    ? t('mcp.projects.overridden')
                    : off
                      ? written.length === 0
                        ? t('mcp.projects.noSwitchHere')
                        : ''
                      : to.length > 0
                        ? ''
                        : // Some Agent could turn it off, just none this project uses.
                          switchTargets(server, defaults, mcpOffTargets).length > 0
                          ? t('mcp.projects.noSwitchHere')
                          : t('mcp.projects.noSwitch');
                // What the page has to admit: the badge says off, yet an Agent without a switch keeps
                // loading the server here, and a list saved with the entry does not follow the project.
                const stillOn = off
                  ? targets.filter(
                      (x) => (server.targets ?? defaults).includes(x) && !written.includes(x),
                    )
                  : [];
                const stale =
                  off &&
                  entry.targets &&
                  [...entry.targets].sort().join() !== [...to].sort().join();
                // Off shows where the switch turns it off; on shows every Agent the global server loads in.
                const icons = off ? written : entry ? [] : (server.targets ?? defaults);
                const iconsLabel = t(off ? 'mcp.projects.offIn' : 'mcp.projects.onIn', {
                  targets: icons.map(targetLabel).join(', '),
                });
                return (
                  <ProjectServerRow
                    key={n}
                    name={n}
                    server={server}
                    tags={
                      <>
                        {off && <span className="ss-tag warn">{t('mcp.projects.offBadge')}</span>}
                        {changes.some((c) => c.name === n && writes(c)) && (
                          <span className="ss-tag warn">{t('plugins.pending')}</span>
                        )}
                      </>
                    }
                    icons={icons}
                    iconsLabel={iconsLabel}
                    checked={!off}
                    disabled={busy || (!off && to.length === 0) || Boolean(entry && !off)}
                    onToggle={() => void toggleGlobal(n, off)}
                  >
                    {reason && <span className="text-ink-3">{reason}</span>}
                    {stillOn.length > 0 && (
                      <span className="text-warn">
                        {t('mcp.projects.stillOn', {
                          targets: stillOn.map(targetLabel).join(', '),
                        })}
                      </span>
                    )}
                    {stale && (
                      <span className="text-warn">
                        {t('mcp.projects.staleSwitch')}{' '}
                        <button
                          type="button"
                          className="underline disabled:opacity-50"
                          disabled={busy}
                          onClick={() => void toggleGlobal(n, false)}
                        >
                          {t('mcp.projects.matchProject')}
                        </button>
                      </span>
                    )}
                  </ProjectServerRow>
                );
              })}
            </div>
          ) : (
            <p className="text-[13px] text-ink-3">{t('mcp.empty')}</p>
          )}
        </section>

        <MCPSharedServers
          root={root}
          source={data.source}
          users={users}
          use={use}
          own={own}
          changes={changes}
          busy={busy}
          onToggle={(n, on) => void toggleShared(n, on)}
          onOverride={setOverriding}
        />

        <section className="mt-3 flex flex-col">
          {/* Two named actions rather than one button that then asks which it was. */}
          <ProjectSectionHead
            icon={<Folder size={16} />}
            title={t('mcp.projects.onlyHere')}
            count={ownRows.length}
            chip={t('mcp.projects.chipLocal')}
            hint={t('mcp.projects.onlyHereHint')}
          >
            <Tooltip content={t('mcp.addOffHint')} side="top">
              <Button
                size="sm"
                variant="ghost"
                className="text-ink-2"
                onClick={() => {
                  setAddingOff(true);
                  setAddMode('form');
                  setEditing('');
                }}
              >
                <PowerOff size={14} />
                {t('mcp.addOff')}
              </Button>
            </Tooltip>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setAddingOff(false);
                setAddMode('form');
                setEditing('');
              }}
            >
              <Plus size={14} />
              {t('mcp.addServer')}
            </Button>
          </ProjectSectionHead>
          {ownRows.length > 0 ? (
            <MCPServerList
              rows={ownRows}
              targets={shown}
              targetsOf={targetsOf}
              offTargets={offTargets}
              onToggle={toggleOwn}
              onMenu={openMenu}
              disabled={busy}
              problems={problemsByServer(check.report, root)}
            />
          ) : (
            <p className="text-[13px] text-ink-3">{t('mcp.projects.noOnlyHere')}</p>
          )}
        </section>
        <Button
          className="flush mt-6 self-start"
          size="sm"
          variant="ghost"
          onClick={() => setDropping(true)}
        >
          <Trash2 size={14} />
          {t('projects.mcp.stop')}
        </Button>
      </RailLayout>

      {showFlow && (
        <MCPFlowDialog
          root={root}
          global={{
            on: shownGlobals.filter((n) => !servers[n]).length,
            total: shownGlobals.length,
          }}
          shared={{ on: use.filter((n) => users[n]).length, total: Object.keys(users).length }}
          own={Object.values(own).filter((x) => !x.disabled).length}
          files={files}
          paths={paths}
          onClose={() => setShowFlow(false)}
        />
      )}
      {editing !== null && (
        <ProjectServerEditor
          data={data}
          model={model}
          root={root}
          offered={offered}
          editing={editing}
          addingOff={addingOff}
          addMode={addMode}
          onMode={setAddMode}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            onChanged();
            toast(t('mcp.toast.saved'), 'success');
          }}
        />
      )}
      {overriding && (
        <MCPOverrideDialog
          name={overriding}
          root={root}
          source={data.source}
          offered={offered}
          paths={paths}
          onClose={() => setOverriding('')}
          onSaved={onChanged}
        />
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
          onImported={() => {
            setImportFrom('');
            onChanged();
          }}
        />
      )}
      {removing && (
        <MCPRemoveDialog
          name={removing}
          project={root}
          inScope={(c) => projectOf(roots, c) === root}
          onClose={() => setRemoving('')}
          onSaved={(unmanaged) => {
            const n = removing;
            setRemoving('');
            onChanged();
            toast(
              t(unmanaged ? 'mcp.toast.unmanaged' : 'mcp.toast.removed', { name: n }),
              'success',
            );
          }}
        />
      )}
      <ConfirmDialog
        open={dropping}
        variant="danger"
        loading={busy}
        title={t('projects.mcp.stopTitle', { name })}
        message={t('projects.mcp.stopMessage')}
        confirmText={t('projects.mcp.stop')}
        onCancel={() => setDropping(false)}
        onConfirm={() => void drop()}
      />
      <SkillContextMenu
        open={!!menu}
        anchorPoint={menu ?? undefined}
        items={menu?.items ?? []}
        onClose={() => setMenu(null)}
      />
    </div>
  );
}
