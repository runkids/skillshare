import { useState } from 'react';
import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ChevronDown, ChevronRight, FileText, Folder, FolderInput, History, Pencil, Trash2 } from 'lucide-react';
import type { Components } from 'react-markdown';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '../../api/client';
import type { MemoryNote, MemoryIndex } from '../../api/memory';
import { useT } from '../../i18n';
import { fileTree } from '../../lib/fileTree';
import { shortenHome } from '../../lib/paths';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import Button from '../Button';
import CodeView from '../CodeView';
import CopyButton from '../CopyButton';
import MarkdownView from '../MarkdownView';
import Spinner from '../Spinner';
import { BoxHeader } from '../instructions/ViewTabs';

/**
 * One card: the note tree (with the search box on top) on the left, the
 * selected note under the same header strip as an instruction file on the right.
 */
export default function MemoryBrowser({ notes, root, busy, onEdit, onMove, onDelete, index, onLinkIndex, search, empty }: {
  notes: MemoryNote[];
  root: string;
  busy: boolean;
  onEdit: (note: MemoryNote) => void;
  onMove: (note: MemoryNote) => void;
  onDelete: (note: MemoryNote) => void;
  index?: MemoryIndex;
  onLinkIndex: (path: string) => Promise<void>;
  search: ReactNode;
  /** Shown in place of the note when nothing matches. */
  empty: ReactNode;
}) {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const picked = params.get('note') ?? '';
  const [view, setView] = useState<'preview' | 'source'>('preview');
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const selected = notes.some((note) => note.path === picked) ? picked
    : notes.find((note) => note.path === 'INDEX.md')?.path ?? notes[0]?.path ?? '';
  const invalid = notes.find((note) => note.path === selected)?.invalid;
  const file = useQuery({ queryKey: queryKeys.memory.content(selected), queryFn: () => api.readMemoryNote(selected), enabled: !!selected && !invalid, staleTime: staleTimes.extras });
  const paths = notes.map((note) => note.path).sort();
  const noteLink = (path: string) => {
    const next = new URLSearchParams(params);
    next.set('note', path);
    return `?${next}`;
  };
  const flip = (path: string) => setCollapsed((prev) => {
    const next = new Set(prev);
    if (!next.delete(path)) next.add(path);
    return next;
  });
  const markdown: Components = {
    a: ({ href, id, children }) => {
      if (href && !/^(?:[a-z][a-z\d+.-]*:|\/\/|#)/i.test(href)) {
        const parts = selected.split('/').slice(0, -1);
        let decoded: string;
        try { decoded = decodeURIComponent(href.split(/[?#]/)[0]); } catch { decoded = ''; }
        for (const part of decoded.split('/')) {
          if (part === '..') parts.pop();
          else if (part && part !== '.') parts.push(part);
        }
        const path = parts.join('/');
        if (paths.includes(path)) return <Link to={noteLink(path)} onClick={() => {
          setCollapsed((prev) => new Set(Array.from(prev).filter((folder) => !path.startsWith(`${folder}/`))));
        }}>{children}</Link>;
      }
      return href?.startsWith('#') ? <a href={href} id={id}>{children}</a>
        : <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>;
    },
  };
  const unavailable = busy || !!invalid || !file.data || file.isFetching;
  const content = invalid || file.error ? undefined : file.data?.content;
  return (
    <div className="ss-box flex min-w-0 flex-col overflow-hidden !p-0 !shadow-none md:h-[calc(100dvh-230px)] md:min-h-[460px] md:flex-row">
      <div className="flex min-h-0 min-w-0 shrink-0 flex-col gap-2.5 border-b border-line-soft bg-sunken p-2.5 md:w-[250px] md:border-b-0 md:border-r">
        {search}
        <nav className="-mx-1 flex min-h-0 min-w-0 flex-1 flex-col gap-0.5 overflow-auto px-1" aria-label={t('memory.title')}>
          {fileTree(paths).filter((row) => !Array.from(collapsed).some((path) => row.path.startsWith(`${path}/`))).map((row) => {
            const note = notes.find((note) => note.path === row.path);
            const Icon = row.folder ? Folder : FileText;
            return (
              <button key={row.path} type="button" className={`ss-nv text-start ${!row.folder && selected === row.path ? 'on' : ''}`}
                style={{ paddingLeft: 10 + row.depth * 20 }} disabled={busy}
                aria-expanded={row.folder ? !collapsed.has(row.path) : undefined}
                aria-current={!row.folder && selected === row.path ? 'true' : undefined}
                aria-label={row.folder ? row.path : `${note?.title} ${row.path}`}
                onClick={() => row.folder ? flip(row.path) : setParams(noteLink(row.path).slice(1))}>
                {row.folder && (collapsed.has(row.path) ? <ChevronRight size={13} /> : <ChevronDown size={13} />)}
                <Icon size={15} className="shrink-0" />
                <span className="truncate font-mono text-[13px]">{row.label}</span>
                {note?.invalid && <span className="ss-tag warn">{t('memory.invalid')}</span>}
              </button>
            );
          })}
        </nav>
        <span className="shrink-0 truncate px-1 pt-1 font-mono text-[11.5px] text-ink-3" title={shortenHome(root)}>{shortenHome(root)}</span>
      </div>
      {!selected ? <div className="min-w-0 flex-1">{empty}</div> : <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <BoxHeader content={content} view={view} onChange={setView} views={[
          { value: 'preview', label: t('instructions.target.view.preview') },
          { value: 'source', label: t('instructions.target.view.source') },
        ]}>
          <span className="mx-1 h-4 w-px bg-line" aria-hidden />
          <button type="button" className="ss-ib" aria-label={t('instructions.edit')} title={t('instructions.edit')} disabled={unavailable} onClick={() => file.data && onEdit(file.data)}><Pencil size={15} /></button>
          <button type="button" className="ss-ib" aria-label={t('memory.move')} title={t('memory.move')} disabled={unavailable} onClick={() => file.data && onMove(file.data)}><FolderInput size={15} /></button>
          <CopyButton value={`${root}/${selected}`} title={t('skillEditor.copyPath')} copiedLabel="" errorMessage={t('memory.copyFailed')} unstyled className="ss-ib" size={15} strokeWidth={2} />
          <Link className="ss-ib" to={`/backup?tab=files&path=${encodeURIComponent(`${root}/${selected}`)}`} aria-label={t('memory.history')} title={t('memory.history')}><History size={15} /></Link>
          <button type="button" className="ss-ib !text-bad" aria-label={t('memory.delete')} title={t('memory.delete')} disabled={unavailable} onClick={() => file.data && onDelete(file.data)}><Trash2 size={15} /></button>
        </BoxHeader>
        <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-auto px-6 py-5">
          <p className="break-all font-mono text-[12px] text-ink-3">{selected}</p>
          {index?.unindexed.includes(selected) && <p className="flex flex-wrap items-center gap-2 text-[12.5px] text-ink-2">
            {t('memory.notInIndex')}
            <Button variant="link" disabled={busy || !index.version} onClick={() => void onLinkIndex(selected)}>{t('memory.addToIndex')}</Button>
          </p>}
          {invalid ? <div className="ss-note warn"><span>{invalid}<br />{t('memory.invalidHint')}</span></div>
            : file.error ? <div className="ss-note bad">{file.error.message}</div>
            : !file.data ? <div className="grid min-h-[200px] place-items-center"><Spinner /></div>
            : view === 'source' ? <CodeView content={file.data.content ?? ''} lang={selected} />
            : <div><MarkdownView size="lg" components={markdown}>{file.data.content ?? ''}</MarkdownView></div>}
        </div>
      </div>}
    </div>
  );
}
