import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Ellipsis, FileText, Plus, RefreshCw, Trash2 } from 'lucide-react';
import { api } from '../../api/client';
import type { InstructionsWarning, SharedInstructionsFile } from '../../api/client';
import Button from '../Button';
import ConfirmDialog from '../ConfirmDialog';
import { PageSkeleton } from '../Skeleton';
import { SkillContextMenu } from '../TargetMenu';
import { useToast } from '../Toast';
import { useT, plural } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import AddLocationDialog from './AddLocationDialog';
import InstructionsEditorDialog from './InstructionsEditorDialog';
import LocationRows from './LocationRows';
import NewSharedDialog from './NewSharedDialog';
import {
  instructionsErrorMessage, instructionsWarningMessage, locationLabel, projectSourcePath, saveCopiesSummary, sharedSyncFeedback, staleLocations,
} from './instructionsView';
import { invalidate } from '../../lib/queryEvents';

type Pending = { title: string; message: string; confirm: string; danger?: boolean; run: () => Promise<void> };

/** Extras › AGENTS.md (project): single-file extras kept in .skillshare/extras, each with the places in the project it is put. */
export default function ProjectSharedFiles({ creating, setCreating }: { creating: boolean; setCreating: (open: boolean) => void }) {
  const t = useT();
  const queryClient = useQueryClient();
  const { data, error, isPending } = useQuery({ queryKey: queryKeys.instructions.shared, queryFn: () => api.listSharedInstructions() });
  // An older server does not say; file links then work as before.
  const fileLinks = data?.file_links ?? true;

  return (
    <section className="flex flex-col gap-2.5" aria-label={t('instructions.projectShared.title')}>
      <div className="ss-sec">
        <h2>{t('instructions.projectShared.title')}</h2>
        <span className="text-[13px] text-ink-3">{t('instructions.projectShared.hint')}</span>
      </div>
      {isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{instructionsErrorMessage(error, t)}</span></div>
      ) : data.files.length === 0 ? (
        <div className="ss-empty !gap-1.5 !p-[26px]">
          <span className="text-[13.5px] font-semibold text-ink">{t('instructions.projectShared.empty.title')}</span>
          <span className="max-w-[460px] text-[12.5px] leading-normal text-ink-3">{t('instructions.projectShared.empty.description')}</span>
          <Button variant="secondary" size="sm" className="mt-2" onClick={() => setCreating(true)}><Plus size={14} />{t('instructions.projectShared.new')}</Button>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {data.files.map((f) => <SharedFileCard key={f.name} file={f} fileLinks={fileLinks} />)}
        </div>
      )}

      {creating && (
        <NewSharedDialog
          project
          targets={[]}
          onClose={() => setCreating(false)}
          onCreated={async () => {
            void invalidate(queryClient, 'instructionsChanged');
            await queryClient.refetchQueries({ queryKey: queryKeys.instructions.shared });
            setCreating(false);
          }}
        />
      )}
    </section>
  );
}

function SharedFileCard({ file, fileLinks }: { file: SharedInstructionsFile; fileLinks: boolean }) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const name = file.name;
  const locations = file.locations ?? [];
  const stale = staleLocations(locations);
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState(false);
  const [adding, setAdding] = useState(false);
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);
  const [pending, setPending] = useState<Pending | null>(null);
  // Fetched only for the editor, so the page does not load every file's text.
  const content = useQuery({ queryKey: queryKeys.instructions.sharedContent(name), queryFn: () => api.getSharedInstructionsContent(name), enabled: editing });
  useEffect(() => {
    if (!editing || !content.error) return;
    toast(instructionsErrorMessage(content.error, t), 'error');
    setEditing(false);
  }, [editing, content.error, toast, t]);
  const list = (names: string[]) => names.join(t('instructions.shared.listSep'));
  const labels = (ls: typeof locations) => list(ls.map((l) => locationLabel(l, true)));

  const act = async (fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
    } catch (err) {
      toast(instructionsErrorMessage(err, t), 'error');
    } finally {
      setBusy(false);
      void invalidate(queryClient, 'instructionsChanged');
    }
  };
  const warn = (warnings?: InstructionsWarning[]) => warnings?.forEach((w) => toast(instructionsWarningMessage(w, t), 'warning'));

  // block: the location holds name as a content block, so only that block is collected or rewritten.
  const resolve = (label: string, path: string, action: 'collect' | 'reapply', block: boolean) => {
    const key = (part: string) => `instructions.resolve.${action}.${part}${block ? 'Block' : ''}`;
    const message = action === 'collect' && !block ? plural('instructions.resolve.collect.message', locations.length) : key('message');
    setPending({
      title: t(key('title'), { name, target: label }),
      message: t(message, { name, target: label, count: locations.length }),
      confirm: t(key('item'), { name }),
      run: async () => {
        await api.resolveSharedInstructions(name, { path }, action);
        toast(t(key('done'), { name, target: label }), 'success');
      },
    });
  };

  const sync = () => act(async () => {
    const res = await api.syncExtras({ name });
    const message = t('instructions.shared.synced', { name, targets: labels(stale.length ? stale : locations) });
    toast(...sharedSyncFeedback(res.extras, message, t));
  });

  const remove = () => setPending({
    title: t('instructions.delete.title', { name }),
    message: locations.length ? t(plural('instructions.projectShared.deleteMessage', locations.length), { name, count: locations.length }) : t('instructions.delete.unused', { name }),
    confirm: t('instructions.detail.delete.confirm'),
    danger: true,
    run: async () => {
      await api.deleteExtra(name);
      toast(t('instructions.detail.delete.done', { name }), 'success');
    },
  });

  return (
    <section className="ss-list" aria-label={name}>
      <div className="ss-gh !min-h-14 !gap-3 !py-2.5">
        <span className="ss-cat sm extra"><FileText size={14} /></span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex items-center gap-2">
            <span className="truncate font-mono font-semibold">{name}</span>
            <span className="shrink-0 text-[12.5px] text-ink-2">{t(plural('instructions.locations.count', locations.length), { count: locations.length })}</span>
          </span>
          <span className="truncate font-mono text-[12px] text-ink-3" title={file.path}>{projectSourcePath(file.path)}</span>
        </span>
        {stale.length > 0 && (
          <>
            <span className="text-[12.5px] text-warn">{t(plural('instructions.projectShared.needSync', stale.length), { count: stale.length })}</span>
            <Button variant="primary" size="sm" onClick={() => void sync()} loading={busy}>{t('extras.sync')}</Button>
          </>
        )}
        <Button variant="secondary" size="sm" disabled={busy} onClick={() => setEditing(true)}>{t('instructions.shared.edit')}</Button>
        <Button variant="secondary" size="sm" disabled={busy} onClick={() => setAdding(true)}><Plus size={14} />{t('instructions.locations.add')}</Button>
        <button type="button" className="ss-ib" aria-label={t('extras.moreActions', { name })} aria-haspopup="menu" aria-expanded={menu !== null}
          onClick={(e) => { const r = e.currentTarget.getBoundingClientRect(); setMenu({ x: r.right - 200, y: r.bottom + 4 }); }}>
          <Ellipsis size={16} />
        </button>
      </div>
      {locations.length > 0 ? (
        <LocationRows project name={name} locations={locations} fileLinks={fileLinks} busy={busy} act={act} warn={warn} onResolve={resolve} />
      ) : (
        <div className="ss-r text-[13px] text-ink-3">{t('instructions.projectShared.noLocations', { name })}</div>
      )}

      {adding && (
        <AddLocationDialog project name={name} file={file.file} fileLinks={fileLinks} onClose={() => setAdding(false)}
          onAdded={(path, warnings) => {
            warn(warnings);
            toast(t('instructions.locations.added', { path, name }), 'success');
            setAdding(false);
            void invalidate(queryClient, 'instructionsChanged');
          }} />
      )}
      {editing && content.data && (
        <InstructionsEditorDialog
          title={`${name} · ${file.file}`}
          path={file.path}
          content={content.data.content}
          onSave={async (next) => {
            const res = await api.putSharedInstructionsContent(name, next);
            void invalidate(queryClient, 'instructionsChanged');
            // Copies were rewritten; say which, and any problem, like the sync button.
            const copies = saveCopiesSummary(res.copies ?? []);
            copies.warnings.forEach((w) => toast(w, 'warning'));
            if (copies.errors.length) toast(copies.errors.join('; '), 'error');
            return copies.updated.length ? t('instructions.savedCopies', { path: projectSourcePath(file.path), targets: list(copies.updated) }) : undefined;
          }}
          onClose={() => setEditing(false)}
        />
      )}
      <SkillContextMenu open={menu !== null} anchorPoint={menu ?? undefined} onClose={() => setMenu(null)} items={[
        { key: 'sync', label: t('extras.sync'), icon: <RefreshCw size={14} />, onSelect: () => void sync() },
        { key: 'delete', label: t('instructions.shared.delete', { name }), icon: <Trash2 size={14} />, danger: true, onSelect: remove },
      ]} />
      <ConfirmDialog
        open={pending !== null}
        title={pending?.title ?? ''}
        message={pending?.message ?? ''}
        confirmText={pending?.confirm}
        variant={pending?.danger ? 'danger' : 'default'}
        loading={busy}
        onCancel={() => setPending(null)}
        onConfirm={async () => {
          const p = pending!;
          await act(p.run);
          setPending(null);
        }}
      />
    </section>
  );
}
