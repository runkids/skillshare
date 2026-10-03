import { useState } from 'react';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { BookOpen, Plus, Search, X } from 'lucide-react';
import { ApiError, api } from '../../api/client';
import type { MemoryNote } from '../../api/memory';
import { useT } from '../../i18n';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import Button from '../Button';
import ConfirmDialog from '../ConfirmDialog';
import DialogShell from '../DialogShell';
import EmptyState from '../EmptyState';
import { Input } from '../Input';
import { PageSkeleton } from '../Skeleton';
import { useToast } from '../Toast';
import InstructionsEditorDialog from '../instructions/InstructionsEditorDialog';
import { instructionsErrorMessage } from '../instructions/instructionsView';
import MemoryBrowser from './MemoryBrowser';
import MemoryGuidance from './MemoryGuidance';
import { Checkbox } from '../Checkbox';

/** creating: the New note dialog, opened from the page header. */
export default function MemoryNotes({ creating, setCreating }: { creating: boolean; setCreating: (open: boolean) => void }) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [editing, setEditing] = useState<MemoryNote | null>(null);
  const [deleting, setDeleting] = useState<MemoryNote | null>(null);
  const [moving, setMoving] = useState<MemoryNote | null>(null);
  const [newPath, setNewPath] = useState('');
  const [, setParams] = useSearchParams();
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [linkIndex, setLinkIndex] = useState(true);
  const [notice, setNotice] = useState('');
  const [backupPath, setBackupPath] = useState('');
  const { data, isPending, error } = useQuery({
    placeholderData: keepPreviousData, queryKey: queryKeys.memory.list(search), queryFn: () => api.listMemoryNotes(search), staleTime: staleTimes.extras,
  });
  const errorMessage = (error: unknown) => error instanceof ApiError && error.code === 'memory_conflict' ? t('memory.conflict')
    : error instanceof ApiError && error.code === 'memory_destination_exists' ? t('memory.destinationExists')
    : instructionsErrorMessage(error, t);
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.memory.all });
    void queryClient.invalidateQueries({ queryKey: queryKeys.extras });
    void queryClient.invalidateQueries({ queryKey: queryKeys.fileBackups.all });
  };
  const initialize = async () => {
    setBusy(true);
    try { await api.initMemory(); refresh(); } catch (err) { toast(errorMessage(err), 'error'); } finally { setBusy(false); }
  };
  const remove = async () => {
    if (!deleting || busy) return;
    setBusy(true);
    try {
      await api.deleteMemoryNote(deleting.path, deleting.version);
      setBackupPath(`${data?.root}/${deleting.path}`);
      setDeleting(null); setEditing(null); refresh(); toast(t('memory.deleted'), 'success');
    } catch (err) { toast(errorMessage(err), 'error'); } finally { setBusy(false); }
  };
  const move = async () => {
    if (!moving || busy) return;
    setBusy(true);
    try {
      const note = await api.moveMemoryNote(moving.path, newPath.trim(), moving.version);
      setBackupPath(`${data?.root}/${moving.path}`);
      setSearch(''); setMoving(null); refresh();
      setParams((prev) => { const next = new URLSearchParams(prev); next.set('note', note.path); return next; });
      toast(t('memory.moved'), 'success');
    } catch (err) { toast(errorMessage(err), 'error'); } finally { setBusy(false); }
  };
  const create = async () => {
    setBusy(true); setNotice('');
    try {
      let index = data?.index;
      if (!data?.initialized) {
        await api.initMemory();
        index = (await api.listMemoryNotes()).index;
      }
      const path = name.trim();
      const title = path.split('/').at(-1)?.replace(/\.md$/i, '') ?? path;
      const note = await api.writeMemoryNote(path, `# ${title}\n`, '');
      if (linkIndex && index?.version && path !== 'INDEX.md') {
        try { await api.linkMemoryIndex(path, index.version); setNotice(''); }
        catch (err) { setNotice(`${t('memory.indexPartial')}: ${errorMessage(err)}`); }
      }
      setCreating(false); setName(''); setEditing(note); refresh();
    } catch (err) { toast(errorMessage(err), 'error'); } finally { setBusy(false); }
  };
  const canMove = !!moving && newPath.trim() !== moving.path && newPath.trim().toLowerCase().endsWith('.md');
  if (isPending) return <PageSkeleton />;
  if (error) return <div className="ss-note bad"><span className="flex-1">{errorMessage(error)}</span></div>;
  if (!data) return null;
  const guidance = <MemoryGuidance initialized={data.initialized} instructions={data.instructions} />;

  return (
    <section className="flex flex-col gap-6" aria-label={t('memory.title')}>
      {notice && <div role="status" className="ss-note warn">{notice}</div>}
      {backupPath && <Link className="ss-more" to={`/backup?tab=files&path=${encodeURIComponent(backupPath)}`}>{t('memory.restoreNote')}</Link>}
      {(data.index?.broken_links.length ?? 0) > 0 && <div className="ss-note warn"><span>{t('memory.brokenLinks', { paths: data.index!.broken_links.join(', ') })}</span></div>}
      {data.notes.length === 0 && !search ? <>
        <EmptyState icon={BookOpen} title={t('memory.empty')} description={t('memory.emptyHint')} action={!data.initialized &&
          <Button variant="primary" disabled={busy} onClick={() => void initialize()}><Plus size={15} />{t('memory.init')}</Button>} />
        {guidance}
      </> : (
        <MemoryBrowser notes={data.notes} root={data.root} busy={busy} onEdit={setEditing} onDelete={setDeleting}
          onMove={(note) => { setMoving(note); setNewPath(note.path); }}
          search={<label className="ss-inp h-[34px] w-full">
            <Search size={15} className="shrink-0 text-ink-3" />
            <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t('memory.searchShort')} aria-label={t('memory.search')} />
          </label>}
          empty={<EmptyState icon={Search} title={t('memory.noMatches')} description={t('memory.emptyHint')} />}
          footer={guidance}
          index={data.index} onLinkIndex={async (path) => {
            if (!data.index?.version) return;
            try { await api.linkMemoryIndex(path, data.index.version); setNotice(''); refresh(); }
            catch (err) { setNotice(errorMessage(err)); }
          }} />
      )}
      {editing && <InstructionsEditorDialog
        key={editing.path} title={editing.path} path={`${data.root}/${editing.path}`} content={editing.content ?? ''}
        note={t('memory.editorHint')}
        onDelete={() => setDeleting(editing)}
        onReadLatest={async () => {
          const latest = await api.readMemoryNote(editing.path);
          setEditing(latest);
          return latest.content ?? '';
        }}
        onClose={() => setEditing(null)}
        onSave={async (content) => {
          const saved = await api.writeMemoryNote(editing.path, content, editing.version);
          setEditing(saved); refresh(); return t('memory.saved');
        }}
      />}
      <ConfirmDialog open={!!deleting} title={t('memory.deleteTitle', { path: deleting?.path ?? '' })}
        message={t('memory.deleteMessage')} confirmText={t('memory.delete')} variant="danger" loading={busy}
        onCancel={() => setDeleting(null)} onConfirm={() => void remove()} />
      <DialogShell open={!!moving} onClose={() => setMoving(null)} preventClose={busy} ariaLabel={t('memory.move')} padding="none">
        <div className="dh"><h2 className="ss-h2">{t('memory.move')}</h2><button type="button" className="ss-ib" aria-label={t('common.close')} onClick={() => setMoving(null)} disabled={busy}><X size={16} /></button></div>
        <div className="db flex flex-col gap-3">
          <Input autoFocus label={t('memory.newPath')} value={newPath} onChange={(e) => setNewPath(e.target.value)} disabled={busy}
            onKeyDown={(e) => { if (e.key === 'Enter' && canMove) void move(); }} />
          <p className="text-[12px] text-ink-3">{t('memory.moveHint')}</p>
        </div>
        <div className="df"><Button variant="ghost" onClick={() => setMoving(null)} disabled={busy}>{t('common.cancel')}</Button><Button variant="primary" onClick={() => void move()} loading={busy} disabled={!canMove}>{t('memory.moveConfirm')}</Button></div>
      </DialogShell>
      <DialogShell open={creating} onClose={() => { setCreating(false); setName(''); }} preventClose={busy} ariaLabel={t('memory.new')} padding="none">
        <div className="dh"><h2 className="ss-h2">{t('memory.new')}</h2><button type="button" className="ss-ib" aria-label={t('common.close')} onClick={() => setCreating(false)} disabled={busy}><X size={16} /></button></div>
        <div className="db flex flex-col gap-3"><Input autoFocus label={t('memory.fileName')} value={name} onChange={(e) => setName(e.target.value)} placeholder="projects/build.md" disabled={busy} /><p className="text-[12px] text-ink-3">{t('memory.fileHint')}</p>
          {data.index?.version && <Checkbox label={t('memory.linkFromIndex')} checked={linkIndex} onChange={setLinkIndex} disabled={busy} />}
        </div>
        <div className="df"><Button variant="ghost" onClick={() => setCreating(false)} disabled={busy}>{t('common.cancel')}</Button><Button variant="primary" onClick={() => void create()} loading={busy} disabled={!name.trim().toLowerCase().endsWith('.md')}>{t('memory.create')}</Button></div>
      </DialogShell>
    </section>
  );
}
