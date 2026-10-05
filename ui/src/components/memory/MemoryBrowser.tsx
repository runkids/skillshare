import { useState } from 'react';
import type { ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronDown, ChevronRight, ChevronUp, Copy, Ellipsis, FileText, Folder, FolderInput, History, RefreshCw, Trash2 } from 'lucide-react';
import type { Components } from 'react-markdown';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../../api/client';
import type { MemoryNote, MemoryIndex } from '../../api/memory';
import { useT } from '../../i18n';
import { fileTree } from '../../lib/fileTree';
import { shortenHome } from '../../lib/paths';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import Button from '../Button';
import CodeView from '../CodeView';
import MarkdownView from '../MarkdownView';
import Spinner from '../Spinner';
import { SkillContextMenu } from '../TargetMenu';
import { useToast } from '../Toast';
import { BoxHeader } from '../instructions/ViewTabs';

// Same as a shared AGENTS.md file: longer notes open collapsed.
const PREVIEW_LINES = 8;

/**
 * Laid out like a shared AGENTS.md file: the note tree (with the search box on
 * top) on the left, the selected note and then footer on the right.
 */
export default function MemoryBrowser({ notes, root, busy, onEdit, onMove, onDelete, index, onLinkIndex, search, empty, footer }: {
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
  /** Shown under the note. */
  footer?: ReactNode;
}) {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const picked = params.get('note') ?? '';
  const [view, setView] = useState<'preview' | 'source'>('preview');
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  // Keyed by path, so another note opens collapsed again.
  const [expandedPath, setExpandedPath] = useState('');
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);
  const navigate = useNavigate();
  const { toast } = useToast();
  const queryClient = useQueryClient();
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
  const notePath = `${root}/${selected}`;
  const expanded = expandedPath === selected;
  const long = (content ?? '').replace(/\n$/, '').split('\n').length > PREVIEW_LINES;
  const copyPath = async () => {
    try {
      await navigator.clipboard.writeText(notePath);
      toast(t('instructions.shared.pathCopied', { path: shortenHome(notePath) }), 'success');
    } catch {
      toast(t('memory.copyFailed'), 'error');
    }
  };
  return (
    <div className="grid grid-cols-[220px_minmax(0,1fr)] items-start gap-6">
      <div className="sticky top-6 flex max-h-[calc(100dvh-48px)] min-w-0 flex-col gap-2.5">
        <div className="flex items-center gap-2">
          {search}
          <button type="button" className="ss-ib shrink-0" title={t('memory.refresh')} aria-label={t('memory.refresh')} disabled={busy}
            onClick={() => void queryClient.invalidateQueries({ queryKey: queryKeys.memory.all })}>
            <RefreshCw size={16} />
          </button>
        </div>
        <nav className="ss-list min-h-0 !overflow-y-auto flex flex-col gap-0.5 p-1.5" aria-label={t('memory.title')}>
          {fileTree(paths).filter((row) => !Array.from(collapsed).some((path) => row.path.startsWith(`${path}/`))).map((row) => {
            const note = notes.find((note) => note.path === row.path);
            const Icon = row.folder ? Folder : FileText;
            const on = !row.folder && selected === row.path;
            return (
              <button key={row.path} type="button" className={`ss-nv !mb-0 text-start ${on ? 'on' : ''}`}
                style={{ paddingLeft: 10 + row.depth * 20 }} disabled={busy}
                aria-expanded={row.folder ? !collapsed.has(row.path) : undefined}
                aria-current={on ? 'true' : undefined}
                aria-label={row.folder ? row.path : `${note?.title} ${row.path}`}
                onClick={() => row.folder ? flip(row.path) : setParams(noteLink(row.path).slice(1))}>
                {row.folder && (collapsed.has(row.path) ? <ChevronRight size={13} /> : <ChevronDown size={13} />)}
                <Icon size={15} className="shrink-0" />
                <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{row.label}</span>
                {note?.invalid && <span className="ss-tag warn">{t('memory.invalid')}</span>}
              </button>
            );
          })}
        </nav>
        <span className="truncate px-1 font-mono text-[12px] text-ink-3" title={shortenHome(root)}>{shortenHome(root)}</span>
      </div>
      <div className="flex min-w-0 flex-col gap-7">
        {!selected ? empty : <section className="flex min-w-0 flex-col gap-4" aria-label={selected}>
          <div className="flex items-start gap-3">
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <h2 className="truncate font-mono text-[20px] font-bold">{selected}</h2>
              <span className="truncate font-mono text-[12px] text-ink-3" title={notePath}>{shortenHome(notePath)}</span>
            </div>
            <Button variant="secondary" size="sm" disabled={unavailable} onClick={() => file.data && onEdit(file.data)}>{t('instructions.edit')}</Button>
            <button type="button" className="ss-ib" aria-label={t('instructions.shared.more')} aria-haspopup="menu" aria-expanded={menu !== null}
              onClick={(e) => { const r = e.currentTarget.getBoundingClientRect(); setMenu({ x: r.right - 200, y: r.bottom + 4 }); }}>
              <Ellipsis size={16} />
            </button>
          </div>
          {index?.unindexed.includes(selected) && <p className="flex flex-wrap items-center gap-2 text-[12.5px] text-ink-2">
            {t('memory.notInIndex')}
            <Button variant="link" disabled={busy || !index.version} onClick={() => void onLinkIndex(selected)}>{t('memory.addToIndex')}</Button>
          </p>}
          <div className="ss-list">
            <BoxHeader content={content} view={view} onChange={setView} views={[
              { value: 'preview', label: t('instructions.target.view.preview') },
              { value: 'source', label: t('instructions.target.view.source') },
            ]}>
              {long && (
                <Button variant="ghost" size="sm" aria-expanded={expanded} onClick={() => setExpandedPath(expanded ? '' : selected)}>
                  {t(expanded ? 'instructions.preview.collapse' : 'instructions.preview.expand')}
                  {expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                </Button>
              )}
            </BoxHeader>
            <div className={`px-6 py-5 ${long && !expanded ? 'max-h-[230px] overflow-y-auto' : ''}`}>
              {invalid ? <div className="ss-note warn"><span>{invalid}<br />{t('memory.invalidHint')}</span></div>
                : file.error ? <div className="ss-note bad">{file.error.message}</div>
                : !file.data ? <div className="grid min-h-[200px] place-items-center"><Spinner /></div>
                : view === 'source' ? <CodeView content={file.data.content ?? ''} lang={selected} />
                : <MarkdownView size="lg" components={markdown}>{file.data.content ?? ''}</MarkdownView>}
            </div>
          </div>
          <SkillContextMenu open={menu !== null} anchorPoint={menu ?? undefined} onClose={() => setMenu(null)} items={[
            { key: 'copy', label: t('instructions.shared.copyPath'), icon: <Copy size={14} />, onSelect: () => void copyPath() },
            { key: 'history', label: t('memory.history'), icon: <History size={14} />, onSelect: () => navigate(`/backup?tab=files&path=${encodeURIComponent(notePath)}`) },
            ...(unavailable ? [] : [
              { key: 'move', label: t('memory.move'), icon: <FolderInput size={14} />, onSelect: () => file.data && onMove(file.data) },
              { key: 'delete', label: t('memory.delete'), icon: <Trash2 size={14} />, danger: true, onSelect: () => file.data && onDelete(file.data) },
            ]),
          ]} />
        </section>}
        {footer}
      </div>
    </div>
  );
}
