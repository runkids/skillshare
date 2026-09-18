import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { File, FileCode2, FileText, Folder, X } from 'lucide-react';
import { pluginsApi } from '../../api/plugins';
import CodeView from '../CodeView';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import MarkdownView, { ViewToggle } from '../MarkdownView';
import Spinner from '../Spinner';
import { useT } from '../../i18n';
import { CODE_EXT, fileTree } from '../../lib/fileTree';
import { isMarkdown } from '../../lib/highlight';
import { shortenPath } from '../../lib/paths';

const NUL = String.fromCharCode(0);

/** The files of a plugin, read only, from the local copy Skillshare reviewed: the same two panes as a skill's Files tab. */
export default function PluginFilesDialog({ name, source, onClose }: { name: string; source?: string; onClose: () => void }) {
  const t = useT();
  const [picked, setPicked] = useState('');
  const [raw, setRaw] = useState(false);
  const list = useQuery({ queryKey: ['plugin-files', name], queryFn: () => pluginsApi.files(name) });
  const files = list.data?.files ?? [];
  const selected = files.includes(picked) ? picked : files.find((f) => /^readme\.md$/i.test(f)) ?? files[0];
  const file = useQuery({ queryKey: ['plugin-file', name, selected], queryFn: () => pluginsApi.file(name, selected), enabled: !!selected });
  const content = file.data?.content;
  const markdown = isMarkdown(selected ?? '');
  const error = list.error ?? file.error;
  return (
    <DialogShell open onClose={onClose} padding="none" ariaLabel={t('plugins.viewFiles')} maxWidth="6xl">
      <div className="dh">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="ss-h2 font-mono">{name}</h2>
          {source && <p className="truncate font-mono text-xs text-ink-3" title={source}>{shortenPath(source)}</p>}
        </div>
        <IconButton icon={<X size={16} />} label={t('common.close')} onClick={onClose} />
      </div>
      <div className="db">
        {error ? <div className="ss-note bad"><span className="flex-1">{(error as Error).message}</span></div>
          : list.isPending ? <div className="grid h-40 place-items-center"><Spinner /></div>
            : files.length === 0 ? <p className="text-[13px] text-ink-2">{t('plugins.noFiles')}</p> : (
              // A fixed height, so each pane scrolls by itself and the list stays beside a long file.
              <div className="grid h-[calc(100vh-13rem)] min-h-[360px] grid-cols-[240px_minmax(0,1fr)] gap-6">
                <div className="flex min-h-0 flex-col gap-0.5 overflow-y-auto">
                  {fileTree(files).map((row) => {
                    const Icon = row.folder ? Folder : row.label.endsWith('.md') ? FileText : CODE_EXT.test(row.label) ? FileCode2 : File;
                    const inner = <><Icon size={15} className="shrink-0" /><span className="truncate font-mono text-[13px]">{row.label}</span></>;
                    const style = { paddingLeft: 10 + row.depth * 20 };
                    return row.folder
                      ? <div key={row.path} className="ss-nv" style={style}>{inner}</div>
                      : <button key={row.path} type="button" aria-current={row.path === selected} className={`ss-nv text-left ${row.path === selected ? 'on' : ''}`} style={style} onClick={() => setPicked(row.path)}>{inner}</button>;
                  })}
                </div>
                <div className="flex min-h-0 min-w-0 flex-col gap-2.5">
                  <div className="flex min-h-8 items-center justify-between gap-3">
                    <span className="truncate font-mono text-[13px] font-semibold">{selected}</span>
                    {markdown && content !== undefined && <ViewToggle raw={raw} onChange={setRaw} />}
                  </div>
                  {content === undefined ? <div className="ss-code grid min-h-0 flex-1 place-items-center"><Spinner /></div>
                    : content.includes(NUL) ? <div className="ss-note"><File size={16} /><div className="flex-1">{t('resourceDetail.files.binary')}</div></div>
                      : !markdown || raw ? <CodeView content={content} lang={selected} className="min-h-0 flex-1" />
                        : <div className="ss-box min-h-0 flex-1 overflow-auto !shadow-none !px-7 !py-6"><MarkdownView size="lg">{content}</MarkdownView></div>}
                </div>
              </div>
            )}
      </div>
    </DialogShell>
  );
}
