import { useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { AlertCircle, Archive, Copy, Download, Eye, Info, Pencil, Plus, Trash2, Webhook } from 'lucide-react';
import { hooksApi, type HookChange, type HookEntry, type HookInventory } from '../../api/hooks';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import Button from '../Button';
import EmptyState from '../EmptyState';
import { RailLayout, RailRow, RailSection } from '../StatusRail';
import SourcePathButton from '../SourcePathButton';
import Tooltip from '../Tooltip';
import { SkillContextMenu, type ContextMenuItem } from '../TargetMenu';
import { useToast } from '../Toast';
import HookDialog from './HookDialog';
import { HooksConfigDialog } from './HooksConfigView';
import HooksImportDialog from './HooksImportDialog';
import HooksList from './HooksList';
import HooksRemoveDialog from './HooksRemoveDialog';
import HooksRestoreDialog from './HooksRestoreDialog';
import HooksSyncBox, { HooksSyncDialog } from './HooksSyncBox';
import HooksUnmanagedNote from './HooksUnmanagedNote';
import { blockedHint, hookAccounts, keyLabel, hookNote, scopeBackups, scopeChanges, scopeEntries, scopePaths, scopePlan, scopeUnmanaged, writes } from './hooksView';

const copy = (text: string) => void navigator.clipboard?.writeText(text);

interface Props {
  data: HookInventory;
  /** A root under hooks.projects. Left out, this is the global source; the two never show each other's hooks. */
  project?: string;
  /** Lays out what sits above the list; receives the buttons (backups, import, add). */
  header: (actions: ReactNode) => ReactNode;
}

/** The hooks of one scope with everything you do to them, on the Hooks page and in a project's tab. */
export default function HooksScope({ data, project, header }: Props) {
  const t = useT();
  const { toast } = useToast();
  const cache = useQueryClient();
  const [editing, setEditing] = useState<string | null>(null); // '' adds a new hook
  const [importing, setImporting] = useState(false);
  const [removing, setRemoving] = useState('');
  const [takingOver, setTakingOver] = useState('');
  const [viewing, setViewing] = useState('');
  const [backupsOpen, setBackupsOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);

  const refresh = () => {
    void cache.invalidateQueries({ queryKey: queryKeys.hooks });
    void cache.invalidateQueries({ queryKey: queryKeys.config });
  };
  const done = (message: string) => {
    setEditing(null); setImporting(false); setRemoving(''); setBackupsOpen(false);
    refresh();
    toast(message, 'success');
  };

  const entries = scopeEntries(data, project);
  const names = Object.keys(entries);
  const plan = scopePlan(data, project);
  // Global sync writes the whole plan, projects included, so the global rail shows all of it; a project's rail stays its root.
  const railPlan = project ? plan : data.plan;
  const paths = scopePaths(data, project);
  const targets = data.targets;
  const accounts = project ? {} : hookAccounts(targets);
  const unmanaged = scopeUnmanaged(data, project);
  const backups = scopeBackups(data, project);
  // A conflict of one of this scope's own hooks opens the same take-over preview as the row menu.
  const ownConflicts = new Set(scopeChanges(data, project).filter((c) => c.action === 'conflict' && c.name in entries).map((c) => `${c.root ?? ''}\0${c.name}`));
  const canTakeOver = (c: HookChange) => c.action === 'conflict' && ownConflicts.has(`${c.root ?? ''}\0${c.name}`);

  // Each save sends the revision it previewed, so the switch holds the others until it settles.
  const toggle = async (name: string, enabled: boolean) => {
    const entry: HookEntry = { ...entries[name] };
    if (enabled) delete entry.enabled; else entry.enabled = false;
    setBusy(true);
    try {
      await hooksApi.save({ ...(project && { project }), name, entry });
      toast(t(enabled ? 'hooks.toast.enabled' : 'hooks.toast.disabled', { name }), 'success');
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setBusy(false);
      refresh();
    }
  };

  const openMenu = (e: React.MouseEvent<HTMLButtonElement>, name: string) => {
    const r = e.currentTarget.getBoundingClientRect();
    setMenu({
      x: r.left, y: r.bottom + 4,
      items: [
        { key: 'edit', label: t('mcp.edit'), icon: <Pencil size={14} />, onSelect: () => setEditing(name) },
        { key: 'view', label: t('hooks.viewConfig'), icon: <Eye size={14} />, onSelect: () => setViewing(name) },
        ...(scopeChanges(data, project).some((c) => c.name === name && c.action === 'conflict')
          ? [{ key: 'takeover', label: t('hooks.takeoverMenu'), icon: <Download size={14} />, onSelect: () => setTakingOver(name) }] : []),
        { key: 'remove', label: t('mcp.remove'), icon: <Trash2 size={14} />, danger: true, onSelect: () => setRemoving(name) },
      ],
    });
  };

  const add = () => setEditing('');
  const actions = (
    <span className="flex flex-wrap items-center justify-end gap-2.5">
      {!project && <SourcePathButton path={data.source.path} configPath={data.source.configPath} section="hooks" />}
      {backups.length > 0 ? <Button variant="ghost" onClick={() => setBackupsOpen(true)}><Archive size={15} />{t('hooks.backupsButton')}</Button> : null}
      <Button variant="secondary" onClick={() => setImporting(true)}><Download size={15} />{t('hooks.import')}</Button>
      <Button variant="primary" onClick={add}><Plus size={15} />{t('hooks.add')}</Button>
    </span>
  );

  return (
    <>
      {header(actions)}
      {data.previewError && <div className="ss-note bad mb-4"><AlertCircle size={16} /><span className="flex-1">{data.previewError}</span></div>}
      <RailLayout className="max-[900px]:grid-cols-1 max-[900px]:[&>*]:static max-[900px]:[&>*]:max-h-none [&>aside]:static [&>aside]:max-h-none" pageScroll rail={<>
        {/* Removing the last hook leaves native changes pending, so the box also shows without a source entry. */}
        {railPlan && (names.length > 0 || railPlan.changes.some((c) => writes(c) || c.action === 'conflict')) && <HooksSyncBox plan={railPlan} project={project} canTakeOver={canTakeOver} onTakeover={setTakingOver} />}
        <HooksUnmanagedNote entries={unmanaged} onImport={() => setImporting(true)} />
        {!project && (
          <RailSection title={t('hooks.targets')} count={targets.length} action={<Tooltip content={t('hooks.targetsInfo')}><Info size={14} className="text-ink-3" aria-label={t('hooks.targetsInfo')} /></Tooltip>}>
            <div className="flex flex-col">
              {targets.map((tg) => (
                <RailRow
                  key={tg.name}
                  target={tg.agent ?? tg.name}
                  label={keyLabel(accounts, tg.name)}
                  path={paths[tg.name] && shortenHome(paths[tg.name])}
                  detail={<>
                    <span>{hookNote(t, tg.agent ?? tg.name, tg.note)}</span>
                    {paths[tg.name] && <button type="button" className="flex items-center gap-1.5 text-xs font-semibold text-ink-2 hover:text-ink" onClick={() => { copy(paths[tg.name]); toast(t('mcp.copied'), 'success'); }}><Copy size={13} />{t('mcp.copyPath')}</button>}
                  </>}
                />
              ))}
            </div>
          </RailSection>
        )}
      </>}>
        {plan?.blocked && <div className="ss-note warn"><AlertCircle size={16} /><span className="flex-1">{blockedHint(t, railPlan ?? plan)}</span></div>}
        {names.length > 0 ? (
          <HooksList accounts={accounts} entries={entries} plan={plan} disabled={busy} onToggle={(n, on) => void toggle(n, on)} onMenu={openMenu} />
        ) : (
          <EmptyState
            icon={Webhook}
            title={t('hooks.empty')}
            description={t('hooks.emptyHint')}
            action={<div className="flex gap-2">
              <Button variant="secondary" onClick={() => setImporting(true)}><Download size={15} />{t('hooks.import')}</Button>
              <Button variant="primary" onClick={add}><Plus size={15} />{t('hooks.add')}</Button>
            </div>}
          />
        )}
      </RailLayout>

      {editing !== null && (
        <HookDialog accounts={accounts} initial={editing ? { name: editing, entry: entries[editing] } : undefined} existingNames={names} project={project} unmanaged={unmanaged} onClose={() => setEditing(null)} onSaved={(synced) => done(t(synced ? 'hooks.toast.savedSynced' : 'hooks.toast.saved'))} />
      )}
      {importing && <HooksImportDialog data={data} project={project} onClose={() => setImporting(false)} onImported={(count) => done(t('hooks.toast.imported', { count }))} />}
      {removing && <HooksRemoveDialog name={removing} project={project} canUnmanage={!entries[removing]?.bindings.git} onClose={() => setRemoving('')} onSaved={(unmanaged) => done(t(unmanaged ? 'hooks.toast.unmanaged' : 'hooks.toast.removed', { name: removing }))} />}
      {viewing && entries[viewing] && <HooksConfigDialog mutation={{ ...(project && { project }), name: viewing, entry: entries[viewing] }} sourcePath={data.source.path} onClose={() => setViewing('')} />}
      {takingOver && entries[takingOver] && <HooksSyncDialog project={project} takeover={{ name: takingOver, entry: entries[takingOver] }} onClose={() => { setTakingOver(''); refresh(); }} />}
      {backupsOpen && <HooksRestoreDialog backups={backups} onClose={() => setBackupsOpen(false)} onRestored={() => done(t('hooks.toast.restored'))} />}
      <SkillContextMenu open={!!menu} anchorPoint={menu ?? undefined} items={menu?.items ?? []} onClose={() => setMenu(null)} />
    </>
  );
}
