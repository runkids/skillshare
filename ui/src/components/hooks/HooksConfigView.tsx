import { useState } from 'react';
import type { KeyboardEvent, ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Folder, Lock, TriangleAlert, X } from 'lucide-react';
import { useAppContext } from '../../context/AppContext';
import { hooksApi, type HookMutation } from '../../api/hooks';
import { useT } from '../../i18n';
import { fileName, shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import CodeView from '../CodeView';
import CopyButton from '../CopyButton';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import Spinner from '../Spinner';
import { hookLabel } from './hooksView';

/** The list item for the Skillshare source; every native file is numbered after it. */
const SOURCE = -1;

/**
 * What sync would write for one saved hook, per Agent file. Read only: nothing is written or run by opening it,
 * and it shows this hook's part of each file, never the shared settings around it.
 */
export function HooksConfigView({ mutation, sourcePath }: { mutation: HookMutation; sourcePath?: string }) {
  const t = useT();
  const { isProjectMode, projectRoot } = useAppContext();
  const root = mutation.project ?? (isProjectMode ? projectRoot : undefined);
  const entry = mutation.entry;
  const view = useQuery({ queryKey: [...queryKeys.hooks, 'render', JSON.stringify(mutation)], queryFn: () => hooksApi.render(mutation) });
  const files = view.data?.rendered ?? [];
  // A disabled hook still renders so its definition can be checked, but sync writes none of it.
  const written = entry?.enabled === false ? 0 : files.filter((f) => f.content !== undefined && !f.error).length;
  const [picked, setPicked] = useState<number>();
  const shown = picked !== undefined && picked < files.length ? picked : files.length > 0 ? 0 : SOURCE;
  const sourceJSON = JSON.stringify({ [mutation.name ?? 'hook']: entry }, null, 2);
  const noAgents = Object.keys(entry?.bindings ?? {}).length === 0;

  const move = (e: KeyboardEvent<HTMLDivElement>) => {
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0;
    if (!step) return;
    e.preventDefault();
    const next = Math.min(files.length - 1, Math.max(SOURCE, shown + step));
    setPicked(next);
    e.currentTarget.querySelectorAll('button')[next + 1]?.focus();
  };
  const item = (key: number, icon: ReactNode, label: string, hint: string, title?: string, error?: string) => (
    <button key={key} type="button" aria-pressed={shown === key} onClick={() => setPicked(key)}
      className={`ss-tn !h-auto min-h-[50px] w-full !gap-2.5 !px-2.5 py-1.5 text-left${shown === key ? ' sel' : ''}`}>
      <span className={`flex w-[18px] shrink-0 justify-center ${shown === key ? '' : 'text-ink-2'}`}>{icon}</span>
      <span className="flex min-w-0 flex-1 flex-col gap-px">
        <span className={`truncate text-[13px] ${shown === key ? 'font-bold' : 'font-semibold'}`}>{label}</span>
        {hint && <span className={`truncate font-mono text-[11px] ${shown === key ? 'opacity-70' : 'text-ink-3'}`} title={title}>{hint}</span>}
      </span>
      {error && <span className="shrink-0 text-warn" title={error}><TriangleAlert size={14} aria-hidden="true" /><span className="sr-only">{error}</span></span>}
    </button>
  );

  const file = shown === SOURCE ? undefined : files[shown];
  const path = shown === SOURCE ? sourcePath : file?.path;
  const content = shown === SOURCE ? sourceJSON : file?.content;
  return (
    <>
      <div className="flex flex-wrap items-center gap-2.5">
        {view.data && <span className="text-sm font-semibold">{t(written === 1 ? 'mcp.viewConfigFiles.one' : 'mcp.viewConfigFiles.other', { count: written })}</span>}
        <span className="inline-flex h-[26px] min-w-0 items-center gap-1.5 rounded-full border border-line bg-sunken px-2.5 text-xs text-ink-2" title={root}>
          <Folder size={13} aria-hidden="true" />
          <span className="truncate">{root ? `${t('app.project')} ${fileName(root)} · ${shortenHome(root)}` : t('app.global')}</span>
        </span>
      </div>
      {view.error && <div className="ss-note bad"><span className="flex-1">{(view.error as Error).message}</span></div>}
      {entry?.enabled === false && <div className="ss-note warn"><span className="flex-1">{t('hooks.viewConfigDisabled')}</span></div>}
      {noAgents && <div className="ss-note inf"><span className="flex-1">{t('hooks.viewConfigNoAgents')}</span></div>}
      <div className="flex items-start gap-4">
        <div className="mt-2 flex max-h-[420px] w-[212px] shrink-0 flex-col gap-0.5 overflow-y-auto" onKeyDown={move}>
          {item(SOURCE, <Folder size={16} aria-hidden="true" />, t('mcp.sourceConfig'), sourcePath ? fileName(sourcePath) : '', sourcePath)}
          {files.length > 0 && <div className="mx-2.5 my-1 border-t border-line-soft" role="separator" />}
          {files.map((f, i) => item(i, <AgentIcon target={f.target} size={18} />, hookLabel(f.target), f.path ? fileName(f.path) : '', f.path, f.error))}
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-2.5">
          <div className="flex min-h-8 items-center gap-2.5">
            {file ? <AgentIcon target={file.target} size={18} /> : <Folder size={16} className="shrink-0 text-ink-2" aria-hidden="true" />}
            <span className="min-w-0 flex-1 truncate font-mono text-xs" title={path}>{path ? shortenHome(path) : file ? hookLabel(file.target) : t('mcp.sourceConfig')}</span>
            {content !== undefined && <CopyButton value={content} />}
          </div>
          {view.isPending && <Spinner size="sm" />}
          {file?.error && <div className="ss-note warn"><span className="flex-1">{file.error}</span></div>}
          {content !== undefined && <CodeView content={content} lang={shown === SOURCE ? 'json' : path ?? ''} className="max-h-[420px]" />}
        </div>
      </div>
      <p className="flex items-center gap-1.5 text-xs text-ink-3"><Lock size={13} aria-hidden="true" className="shrink-0" />{t('hooks.viewConfigHint')}</p>
    </>
  );
}

export function HooksConfigDialog({ mutation, sourcePath, onClose }: { mutation: HookMutation; sourcePath?: string; onClose: () => void }) {
  const t = useT();
  return (
    <DialogShell open onClose={onClose} padding="none" ariaLabel={t('mcp.viewConfig')} className="!max-w-[880px]">
      <div className="dh">
        <div className="flex min-w-0 flex-col gap-1"><h2 className="ss-h2">{t('mcp.viewConfig')}</h2><p className="truncate font-mono text-xs text-ink-3">{mutation.name}</p></div>
        <IconButton icon={<X size={16} />} label={t('common.close')} onClick={onClose} />
      </div>
      <div className="db"><HooksConfigView mutation={mutation} sourcePath={sourcePath} /></div>
      <div className="df"><Button variant="ghost" onClick={onClose}>{t('common.close')}</Button></div>
    </DialogShell>
  );
}
