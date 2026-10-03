import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ChevronDown, ChevronRight, FileText, Folder, Trash2 } from 'lucide-react';
import type { Components } from 'react-markdown';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '../../api/client';
import type { MemoryNote, MemoryIndex } from '../../api/memory';
import { useT } from '../../i18n';
import { fileTree } from '../../lib/fileTree';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import Button from '../Button';
import CodeView from '../CodeView';
import CopyButton from '../CopyButton';
import MarkdownView, { ViewToggle } from '../MarkdownView';
import Spinner from '../Spinner';

/** Uses the same file tree and preview components as a skill's Files tab. */
export default function MemoryBrowser({ notes, root, busy, onEdit, onDelete, index, onLinkIndex }: {
  notes: MemoryNote[];
  root: string;
  busy: boolean;
  onEdit: (note: MemoryNote) => void;
  onDelete: (note: MemoryNote) => void;
  index?: MemoryIndex;
  onLinkIndex: (path: string) => Promise<void>;
}) {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const picked = params.get('note') ?? '';
  const [raw, setRaw] = useState(false);
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
  return (
    <div className="grid grid-cols-1 md:grid-cols-[240px_minmax(0,1fr)] items-start gap-6">
      <nav className="flex min-w-0 flex-col gap-0.5" aria-label={t('memory.title')}>
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
      <div className="flex min-w-0 flex-col gap-2.5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <span className="break-all font-mono text-[13px] font-semibold">{selected}</span>
          <div className="flex flex-wrap items-center gap-2">
            <ViewToggle raw={raw} onChange={setRaw} />
            <CopyButton value={`${root}/${selected}`} title={t('skillEditor.copyPath')} label={t('skillEditor.copyPath')} copiedLabel={t('memory.copied')} errorMessage={t('memory.copyFailed')} unstyled className="ss-btn ghost sm" />
            <Button variant="ghost" size="sm" disabled={busy || !!invalid || !file.data || file.isFetching} onClick={() => file.data && onEdit(file.data)}>{t('instructions.edit')}</Button>
            <Button variant="ghost" size="sm" disabled={busy || !!invalid || !file.data || file.isFetching} onClick={() => file.data && onDelete(file.data)}><Trash2 size={14} />{t('memory.delete')}</Button>
            <Link className="ss-more text-[13px]" to={`/backup?tab=files&path=${encodeURIComponent(`${root}/${selected}`)}`}>{t('memory.history')}</Link>
            {index?.unindexed.includes(selected) && <Button variant="ghost" size="sm" disabled={busy || !index.version} onClick={() => void onLinkIndex(selected)}>{t('memory.addToIndex')}</Button>}
          </div>
        </div>
        {invalid ? <div className="ss-note warn"><span>{invalid}<br />{t('memory.invalidHint')}</span></div>
          : file.error ? <div className="ss-note bad">{file.error.message}</div>
          : !file.data ? <div className="ss-code grid min-h-[360px] place-items-center"><Spinner /></div>
          : raw ? <CodeView content={file.data.content ?? ''} lang={selected} className="max-h-[60vh] min-h-[360px]" />
          : <div className="ss-box max-h-[60vh] min-h-[360px] overflow-auto !shadow-none !px-7 !py-6"><MarkdownView size="lg" components={markdown}>{file.data.content ?? ''}</MarkdownView></div>}
      </div>
    </div>
  );
}
