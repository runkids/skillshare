import { useState, type ReactNode } from 'react';
import { useQuery, type QueryKey } from '@tanstack/react-query';
import { File, FileCode2, FileText, Folder } from 'lucide-react';
import { ApiError } from '../api/http';
import CodeView from './CodeView';
import DialogHeader from './DialogHeader';
import DialogShell from './DialogShell';
import MarkdownView, { ViewToggle } from './MarkdownView';
import Spinner from './Spinner';
import { useT } from '../i18n';
import { CODE_EXT, fileTree } from '../lib/fileTree';
import { isMarkdown } from '../lib/highlight';
import { shortenPath } from '../lib/paths';

const NUL = String.fromCharCode(0);

type FileQuery = (path: string) => { key: QueryKey; read: () => Promise<string> };

/**
 * Files shown read only in the page, for a folder or one file the dashboard's machine may have
 * no desktop to open: a tree beside the picked file, or the file alone when there is one.
 * `files` is a fixed list, or a query that lists them. `fresh` reads again on every open, for
 * files a sync or a hand edit can change.
 */
export default function FilesDialog({
  title,
  subtitle,
  ariaLabel,
  emptyText,
  files,
  file,
  fresh = false,
  onClose,
}: {
  title: string;
  subtitle?: string;
  ariaLabel: string;
  emptyText?: string;
  files: string[] | { key: QueryKey; list: () => Promise<string[]> };
  file: FileQuery;
  fresh?: boolean;
  onClose: () => void;
}) {
  const browse = (names: string[]) => (
    <FileBrowser names={names} file={file} fresh={fresh} emptyText={emptyText} />
  );
  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      ariaLabel={ariaLabel}
      maxWidth={Array.isArray(files) && files.length === 1 ? '4xl' : '6xl'}
    >
      <DialogHeader
        title={title}
        titleClassName="font-mono"
        subtitle={subtitle ? shortenPath(subtitle) : null}
        subtitleProps={{ className: 'truncate font-mono text-xs text-ink-3', title: subtitle }}
        onClose={onClose}
      />
      <div className="db">
        {Array.isArray(files) ? (
          browse(files)
        ) : (
          <ListedFiles query={files} fresh={fresh}>
            {browse}
          </ListedFiles>
        )}
      </div>
    </DialogShell>
  );
}

function ListedFiles({
  query,
  fresh,
  children,
}: {
  query: { key: QueryKey; list: () => Promise<string[]> };
  fresh: boolean;
  children: (names: string[]) => ReactNode;
}) {
  const list = useQuery({
    queryKey: query.key,
    queryFn: query.list,
    staleTime: fresh ? 0 : undefined,
    // A fresh view also forgets its copy when closed, so reopening never flashes old content.
    gcTime: fresh ? 0 : undefined,
  });
  if (list.error) {
    return (
      <div className="ss-note bad">
        <span className="flex-1">{(list.error as Error).message}</span>
      </div>
    );
  }
  if (list.isPending) {
    return (
      <div className="grid h-40 place-items-center">
        <Spinner />
      </div>
    );
  }
  return children(list.data);
}

function FileBrowser({
  names,
  file,
  fresh,
  emptyText,
}: {
  names: string[];
  file: FileQuery;
  fresh: boolean;
  emptyText?: string;
}) {
  const [picked, setPicked] = useState('');
  const selected = names.includes(picked)
    ? picked
    : (names.find((f) => /^readme\.md$/i.test(f)) ?? names[0]);
  if (!selected) return <p className="text-[13px] text-ink-2">{emptyText}</p>;
  const single = names.length === 1;
  const pane = <FilePane name={selected} query={file(selected)} fresh={fresh} titled={!single} />;
  if (single) {
    // The dialog's title already names the file, and a short file needs no empty space below it.
    return <div className="flex max-h-[calc(100vh-13rem)] min-h-0 flex-col">{pane}</div>;
  }
  return (
    // A fixed height, so each pane scrolls by itself and the list stays beside a long file.
    <div className="grid h-[calc(100vh-13rem)] min-h-[360px] grid-cols-[240px_minmax(0,1fr)] gap-6">
      <div className="flex min-h-0 flex-col gap-0.5 overflow-y-auto">
        {fileTree(names).map((row) => {
          const Icon = row.folder
            ? Folder
            : row.label.endsWith('.md')
              ? FileText
              : CODE_EXT.test(row.label)
                ? FileCode2
                : File;
          const inner = (
            <>
              <Icon size={15} className="shrink-0" />
              <span className="truncate font-mono text-[13px]">{row.label}</span>
            </>
          );
          const style = { paddingLeft: 10 + row.depth * 20 };
          return row.folder ? (
            <div key={row.path} className="ss-nv" style={style}>
              {inner}
            </div>
          ) : (
            <button
              key={row.path}
              type="button"
              aria-current={row.path === selected}
              className={`ss-nv text-left ${row.path === selected ? 'on' : ''}`}
              style={style}
              onClick={() => setPicked(row.path)}
            >
              {inner}
            </button>
          );
        })}
      </div>
      {pane}
    </div>
  );
}

function FilePane({
  name,
  query,
  fresh,
  titled,
}: {
  name: string;
  query: { key: QueryKey; read: () => Promise<string> };
  fresh: boolean;
  titled: boolean;
}) {
  const t = useT();
  const [raw, setRaw] = useState(false);
  const file = useQuery({
    queryKey: query.key,
    queryFn: query.read,
    staleTime: fresh ? 0 : undefined,
    gcTime: fresh ? 0 : undefined,
  });
  const content = file.data;
  const markdown = isMarkdown(name);
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2.5">
      {(titled || markdown) && (
        <div className="flex min-h-8 items-center justify-between gap-3">
          {titled && <span className="truncate font-mono text-[13px] font-semibold">{name}</span>}
          {markdown && content !== undefined && <ViewToggle raw={raw} onChange={setRaw} />}
        </div>
      )}
      {file.error ? (
        <div className="ss-note bad">
          <span className="flex-1">
            {file.error instanceof ApiError && file.error.status === 413
              ? t('resourceDetail.files.tooLarge')
              : file.error instanceof ApiError && file.error.status === 404
                ? t('resourceDetail.files.notFound')
                : file.error.message}
          </span>
        </div>
      ) : content === undefined ? (
        <div className="ss-code grid min-h-0 flex-1 place-items-center">
          <Spinner />
        </div>
      ) : content.includes(NUL) ? (
        <div className="ss-note">
          <File size={16} />
          <div className="flex-1">{t('resourceDetail.files.binary')}</div>
        </div>
      ) : !markdown || raw ? (
        <CodeView content={content} lang={name} className="min-h-0 flex-1" />
      ) : (
        <div className="ss-box min-h-0 flex-1 overflow-auto !shadow-none !px-7 !py-6">
          <MarkdownView size="lg">{content}</MarkdownView>
        </div>
      )}
    </div>
  );
}
