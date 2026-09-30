import { useState } from 'react';
import type { KeyboardEvent, ReactNode } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { mcpApi, type MCPMutation } from '../../api/mcp';
import { Folder, Lock, TriangleAlert, X } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import CodeView from '../CodeView';
import CopyButton from '../CopyButton';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import Spinner from '../Spinner';
import { useT } from '../../i18n';
import { inside, targetLabel, toolGapKey } from './mcpView';
import { queryKeys } from '../../lib/queryKeys';
import { fileName, shortenHome } from '../../lib/paths';
import { useAppContext } from '../../context/AppContext';
import { useMcpQuery } from '../../hooks/useSharedQueries';

/** The list item for the Skillshare source; Agents use their target name. */
const SOURCE = '';

const formatOf = (path: string) => ({ json: 'JSON', jsonc: 'JSONC', toml: 'TOML', yaml: 'YAML', yml: 'YAML' })[path.split('.').pop()?.toLowerCase() ?? ''];

/**
 * What Sync would write for one server, per Agent. Read only on purpose: the source keeps
 * secrets as references, and an editable native view would invite pasting them back in as plain text.
 */
export default function MCPConfigView({ mutation }: { mutation: MCPMutation }) {
  const t = useT();
  const { isProjectMode, projectRoot } = useAppContext();
  const sourcePath = useMcpQuery().data?.source.path;
  const targets = mutation.server?.targets ?? [];
  const [picked, setPicked] = useState<string>();
  const items = [SOURCE, ...targets];
  const shown = picked !== undefined && items.includes(picked) ? picked : targets[0] ?? SOURCE;
  // Pi previews read native settings and ownership; keep the previous view while refreshing.
  const view = useQuery({ queryKey: [...queryKeys.mcp, 'render', JSON.stringify(mutation)], queryFn: () => mcpApi.render(mutation), placeholderData: keepPreviousData });
  const renderedFor = (target: string) => view.data?.rendered.find((r) => r.target === target);
  const files = view.data?.rendered.filter((r) => r.content !== undefined && !r.error).length ?? 0;

  const root = mutation.project ?? (isProjectMode ? projectRoot : undefined);
  // Paths in the list are relative to the scope: the project root, or home for the global config.
  // Claude Code's off list sits in ~/.claude.json even for a project, so it keeps its ~.
  const relative = (path: string) => root ? (inside(root, path) ? path.slice(root.length + 1) : shortenHome(path)) : shortenHome(path).replace(/^~[\\/]/, '');
  const sourceHint = !sourcePath ? '' : mutation.project ? `${fileName(sourcePath)} · mcp.projects` : root && inside(root, sourcePath) ? relative(sourcePath) : fileName(sourcePath);
  const sourceJSON = JSON.stringify({ [mutation.name ?? 'server']: mutation.server }, null, 2);

  const move = (e: KeyboardEvent<HTMLDivElement>) => {
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0;
    if (!step) return;
    e.preventDefault();
    const next = Math.min(items.length - 1, Math.max(0, items.indexOf(shown) + step));
    setPicked(items[next]);
    e.currentTarget.querySelectorAll('button')[next]?.focus();
  };
  const item = (key: string, icon: ReactNode, label: string, hint: string, title?: string, error?: string) => (
    <button key={key} type="button" aria-pressed={shown === key} onClick={() => setPicked(key)}
      className={`ss-tn !h-auto min-h-[50px] w-full !gap-2.5 !px-2.5 py-1.5 text-left${shown === key ? ' sel' : ''}`}>
      <span className={`flex w-[18px] shrink-0 justify-center ${shown === key ? '' : 'text-ink-2'}`}>{icon}</span>
      <span className="flex min-w-0 flex-1 flex-col gap-px">
        <span className={`truncate text-[13px] ${shown === key ? 'font-bold' : 'font-semibold'}`}>{label}</span>
        {/* On the selection colour the hint keeps its ink; --ink-3 fades out on Playful's amber. */}
        {hint && <span className={`truncate font-mono text-[11px] ${shown === key ? 'opacity-70' : 'text-ink-3'}`} title={title}>{hint}</span>}
      </span>
      {error && <span className="shrink-0 text-warn" title={error}><TriangleAlert size={14} aria-hidden="true" /><span className="sr-only">{error}</span></span>}
    </button>
  );

  const rendered = shown === SOURCE ? undefined : renderedFor(shown);
  const path = shown === SOURCE ? sourcePath : rendered?.path;
  const content = shown === SOURCE ? sourceJSON : rendered?.content;
  const format = shown !== SOURCE && path ? formatOf(path) : undefined;
  return (
    <>
      <div className="flex flex-wrap items-center gap-2.5">
        {view.data && <span className="text-sm font-semibold">{t(files === 1 ? 'mcp.viewConfigFiles.one' : 'mcp.viewConfigFiles.other', { count: files })}</span>}
        <span className="inline-flex h-[26px] min-w-0 items-center gap-1.5 rounded-full border border-line bg-sunken px-2.5 text-xs text-ink-2" title={root}>
          <Folder size={13} aria-hidden="true" />
          <span className="truncate">{root ? `${t('app.project')} ${fileName(root)} · ${shortenHome(root)}` : t('app.global')}</span>
        </span>
      </div>
      {view.error && <div className="ss-note bad"><span className="flex-1">{(view.error as Error).message}</span></div>}
      <div className="flex items-start gap-4">
        <div className="mt-2 flex max-h-[420px] w-[212px] shrink-0 flex-col gap-0.5 overflow-y-auto" onKeyDown={move}>
          {item(SOURCE, <Folder size={16} aria-hidden="true" />, t('mcp.sourceConfig'), sourceHint, sourcePath)}
          <div className="mx-2.5 my-1 border-t border-line-soft" role="separator" />
          {targets.map((target) => {
            const r = renderedFor(target);
            return item(target, <AgentIcon target={target} size={18} />, targetLabel(target), r ? relative(r.path) : '', r?.path, r?.error);
          })}
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-2.5">
          <div className="flex min-h-8 items-center gap-2.5">
            {shown === SOURCE ? <Folder size={16} className="shrink-0 text-ink-2" aria-hidden="true" /> : <AgentIcon target={shown} size={18} />}
            <span className="min-w-0 flex-1 truncate font-mono text-xs" title={path}>{path ? shortenHome(path) : shown === SOURCE ? t('mcp.sourceConfig') : targetLabel(shown)}</span>
            {format && <span className="ss-tag">{format}</span>}
            {content !== undefined && <CopyButton value={content} />}
          </div>
          {shown !== SOURCE && view.isPending && <Spinner size="sm" />}
          {rendered?.error && <div className="ss-note warn"><span className="flex-1">{rendered.error}</span></div>}
          {rendered?.toolGaps?.length ? <div className="ss-note warn"><span className="flex-1">{t('mcp.tools.notAppliedHere', { parts: rendered.toolGaps.map((gap) => t(toolGapKey(gap))).join(t('mcp.tools.partSeparator')) })}</span></div> : null}
          {content !== undefined && <CodeView content={content} lang={shown === SOURCE ? 'json' : path ?? ''} className="max-h-[420px]" />}
        </div>
      </div>
      <p className="flex items-center gap-1.5 text-xs text-ink-3"><Lock size={13} aria-hidden="true" className="shrink-0" />{t('mcp.viewConfigHint')}</p>
    </>
  );
}

/** The same view for a server as it is saved, reached from its row. */
export function MCPConfigDialog({ mutation, onClose }: { mutation: MCPMutation; onClose: () => void }) {
  const t = useT();
  return (
    <DialogShell open onClose={onClose} padding="none" ariaLabel={t('mcp.viewConfig')} className="!max-w-[880px]">
      <div className="dh">
        <div className="flex min-w-0 flex-col gap-1"><h2 className="ss-h2">{t('mcp.viewConfig')}</h2><p className="truncate font-mono text-xs text-ink-3">{mutation.name}</p></div>
        <IconButton icon={<X size={16} />} label={t('common.close')} onClick={onClose} />
      </div>
      <div className="db"><MCPConfigView mutation={mutation} /></div>
      <div className="df"><Button variant="ghost" onClick={onClose}>{t('common.close')}</Button></div>
    </DialogShell>
  );
}
