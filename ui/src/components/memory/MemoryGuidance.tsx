import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, ChevronDown, Copy } from 'lucide-react';
import { Link } from 'react-router-dom';
import { api, ApiError } from '../../api/client';
import type { MemoryGuidancePlan, MemoryGuidanceTarget, MemoryInstructions, MemoryUpdateMode } from '../../api/memory';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import { shortenHome } from '../../lib/paths';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import CopyButton from '../CopyButton';
import DialogShell from '../DialogShell';
import { lineDiff } from '../instructions/instructionsView';
import type { DiffLine } from '../instructions/instructionsView';
import { useToast } from '../Toast';
import Tooltip from '../Tooltip';

const MODES: MemoryUpdateMode[] = ['passive', 'active'];

/** Copies the guidance of the chosen update mode. */
function CopyGuidanceMenu({ instructions }: { instructions: MemoryInstructions }) {
  const t = useT();
  const { toast } = useToast();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => { if (!ref.current?.contains(e.target as Node)) setOpen(false); };
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false); };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onKey); };
  }, [open]);
  const copy = async (mode: MemoryUpdateMode) => {
    setOpen(false);
    try { await navigator.clipboard.writeText(instructions[mode]); toast(t('memory.copied'), 'success'); }
    catch { toast(t('memory.copyFailed'), 'error'); }
  };
  return (
    <div ref={ref} className="relative">
      <button type="button" className="ss-btn ghost sm" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(!open)}>
        <Copy size={12} strokeWidth={2.5} />{t('memory.copyInstructions')}<ChevronDown size={14} />
      </button>
      {open && <div role="menu" className="ss-menu absolute right-0 top-full z-50 mt-1 !w-[320px]">
        {MODES.map((mode) => <button key={mode} type="button" role="menuitem" className="!h-auto flex-col !items-start !gap-0.5 !py-2" onClick={() => void copy(mode)}>
          <span className="font-mono text-[13px] font-semibold text-ink">{mode}</span>
          <span className="text-[12.5px] leading-snug text-ink-2">{t(`memory.mode.${mode}`)}</span>
        </button>)}
      </div>}
    </div>
  );
}

/** One file's change as a unified diff, like a Git diff. */
function ChangeDiff({ path, before, after }: { path: string; before: string; after: string }) {
  // Within each run of changed lines, removals come before additions, as in Git.
  const lines: DiffLine[] = [];
  let run: DiffLine[] = [];
  for (const l of [...lineDiff(before, after), null]) {
    if (l && l.kind !== 'same') { run.push(l); continue; }
    lines.push(...run.filter((r) => r.kind === 'del'), ...run.filter((r) => r.kind === 'add'));
    run = [];
    if (l) lines.push(l);
  }
  const added = lines.filter((l) => l.kind === 'add').length;
  const removed = lines.filter((l) => l.kind === 'del').length;
  return (
    <div className="ss-list !shadow-none">
      <div className="ss-lh !normal-case">
        <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] font-semibold text-ink" title={path}>{shortenHome(path)}</span>
        <span className="font-mono text-[12px]">
          {added > 0 && <span className="text-ok">+{added}</span>} {removed > 0 && <span className="text-bad">−{removed}</span>}
        </span>
      </div>
      <pre className="ss-code !max-h-[50vh] !rounded-none !border-0 !overflow-auto !whitespace-pre-wrap" style={{ overflowWrap: 'anywhere' }}>
        {lines.map((l, i) => (
          <span key={i} className={l.kind === 'same' ? 'block' : l.kind}>{l.kind === 'add' ? '+ ' : l.kind === 'del' ? '− ' : '  '}{l.text || ' '}</span>
        ))}
      </pre>
    </div>
  );
}

export default function MemoryGuidance({ initialized, instructions }: { initialized: boolean; instructions: MemoryInstructions }) {
  const t = useT();
  const client = useQueryClient();
  // States follow assignments and files that other tabs and the CLI change, so never trust a cached copy.
  const guidance = useQuery({ queryKey: [...queryKeys.memory.all, 'guidance'], queryFn: api.getMemoryGuidance, enabled: initialized, staleTime: 0 });
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [modes, setModes] = useState<Record<string, MemoryUpdateMode>>({});
  const [plan, setPlan] = useState<MemoryGuidancePlan | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const close = () => { setOpen(false); setPlan(null); setError(''); };
  const targets = guidance.data?.targets ?? [];
  const modeOf = (target: MemoryGuidanceTarget) => modes[target.name] ?? target.mode ?? 'passive';
  const connected = (target: MemoryGuidanceTarget) => target.state === 'configured' || target.state === 'outdated';
  // Connected targets whose mode changed are part of the request without a check.
  const requested = targets.filter((target) => selected.includes(target.name) || (connected(target) && modeOf(target) !== (target.mode ?? 'passive'))).map((target) => target.name);
  const requestModes = Object.fromEntries(targets.filter((target) => requested.includes(target.name)).map((target) => [target.name, modeOf(target)]));
  // Targets reading one file share one block, so they switch together.
  const setMode = (target: MemoryGuidanceTarget, mode: MemoryUpdateMode) => setModes((prev) => ({
    ...prev, ...Object.fromEntries(targets.filter((other) => other.name === target.name || (!!target.file && other.file === target.file)).map((other) => [other.name, mode])),
  }));
  const sharers = (target: MemoryGuidanceTarget) => target.file ? targets.filter((other) => other.name !== target.name && other.file === target.file).map((other) => other.name) : [];
  const review = async () => {
    setBusy(true); setError('');
    try { setPlan(await api.planMemoryGuidance(requested, requestModes)); }
    catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  };
  const apply = async () => {
    if (!plan || busy) return;
    setBusy(true); setError('');
    try {
      const result = await api.applyMemoryGuidance(requested, requestModes, plan.token);
      void client.invalidateQueries({ queryKey: queryKeys.memory.all });
      void client.invalidateQueries({ queryKey: queryKeys.instructions.all });
      void client.invalidateQueries({ queryKey: queryKeys.fileBackups.all });
      if (result.success) close();
      else { setPlan(null); setError(result.errors.map((e) => `${e.path}: ${e.error}`).join('\n')); }
    } catch (err) { setPlan(null); setError(err instanceof ApiError && err.code === 'memory_guidance_stale' ? t('memory.guidanceStale') : (err as Error).message); }
    finally { setBusy(false); }
  };
  const prompt = t('memory.checkPrompt');
  // The specific reason beats the generic state; it must be readable without hovering.
  const statusText = (target: (typeof targets)[number]) => target.detail ? t(`memory.connectionDetail.${target.detail}`, undefined, target.detail) : t(`memory.connection.${target.state}`);
  return (
    <section className="flex min-w-0 flex-col gap-3" aria-label={t('memory.useWithAgents')}>
      <div className="flex flex-wrap items-center gap-2 pt-1.5 pl-1">
        <h3 className="text-[15px] font-bold">{t('memory.useWithAgents')}</h3>
        {!!targets.length && <span className="text-[12.5px] text-ink-3">{t('memory.connectedCount', { count: targets.filter((target) => target.state === 'configured').length, total: targets.length })}</span>}
        <span className="flex-1" />
        <CopyGuidanceMenu instructions={instructions} />
        <Tooltip content={<span className="block whitespace-pre-wrap break-words font-mono">{prompt}</span>}>
          <CopyButton value={prompt} title={t('memory.check')} label={t('memory.check')} copiedLabel={t('memory.copied')} errorMessage={t('memory.copyFailed')} unstyled className="ss-btn ghost sm" />
        </Tooltip>
        <Button variant="secondary" size="sm" disabled={!initialized || !targets.length} onClick={() => { void guidance.refetch(); setSelected([]); setModes({}); setPlan(null); setOpen(true); }}>{t('memory.connect')}</Button>
      </div>
      {guidance.error && <div className="ss-note bad">{guidance.error.message}</div>}
      {!!targets.length && <div className="ss-list">
        {targets.map((target) => <div key={target.name} className="ss-r !min-h-[56px]">
          <span className="ss-at"><AgentIcon target={target.name} size={17} /></span>
          <span className="w-[110px] shrink-0 truncate font-mono text-[13px] font-semibold">{target.name}</span>
          <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] text-ink-2" title={target.file}>{target.file ? shortenHome(target.file) : ''}</span>
          {target.mode && connected(target) && <span className="ss-tag">{target.mode}</span>}
          <span className={`ss-st ${target.state === 'configured' ? 'ok' : target.state === 'unconfigured' ? 'off' : 'warn'}`}>{statusText(target)}</span>
        </div>)}
      </div>}
      {initialized && guidance.data?.targets.length === 0 && <p className="text-[13px] text-ink-3">{t('memory.noTargets')}</p>}
      <p className="pl-1 text-[12.5px] leading-relaxed text-ink-3">
        {t('memory.connectionHint')} <Link to="?tab=instructions" className="ss-more">{t('memory.openInstructions')}</Link>
      </p>
      <DialogShell open={open} onClose={close} preventClose={busy} padding="none" maxWidth={plan ? "4xl" : "lg"} ariaLabel={t('memory.connect')}>
        <div className="dh"><h2 className="ss-h2">{t(plan ? 'memory.reviewConnections' : 'memory.connect')}</h2></div>
        <div className="db flex flex-col gap-4">
          {error && <div role="alert" className="ss-note bad whitespace-pre-wrap">{error}</div>}
          {!plan ? <>
            <p className="text-[13px] text-ink-2">{t('memory.connectHint')}</p>
            <div className="ss-list !shadow-none">
              {targets.map((target) => {
                const on = selected.includes(target.name) || target.state === 'configured';
                const disabled = busy || target.state === 'configured' || target.state === 'broken';
                const modeOff = busy || target.state === 'broken' || !(on || connected(target));
                const shares = sharers(target);
                return <div key={target.name} className="ss-r !min-h-11 !gap-2.5 !py-1.5">
                  <label title={target.file ? shortenHome(target.file) : undefined} className={`flex min-w-0 flex-1 items-center gap-2.5 ${disabled ? 'cursor-not-allowed' : 'cursor-pointer'}`}>
                    <input type="checkbox" className="ss-chk-input sr-only" checked={on} disabled={disabled}
                      onChange={(e) => setSelected((prev) => e.target.checked ? [...prev, target.name] : prev.filter((name) => name !== target.name))} />
                    <span className={`ss-chk ${on ? 'on' : ''} ${disabled ? 'opacity-40' : ''}`}>{on && <Check size={12} strokeWidth={3} />}</span>
                    <span className="ss-at !h-6 !w-6"><AgentIcon target={target.name} size={14} /></span>
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate font-mono text-[13px] font-semibold text-ink">{target.name}</span>
                      {shares.length > 0 && <span className="truncate text-[12px] text-ink-3" title={shares.join(", ")}>{t('memory.sharesFile', { targets: shares.join(', ') })}</span>}
                    </span>
                    {requested.includes(target.name) && <span className="ss-tag !border-transparent !bg-[var(--accent-bg)] !text-accent">pending</span>}
                    <span className={`text-right text-[12.5px] ${target.state === 'outdated' || target.state === 'broken' ? 'text-warn' : 'text-ink-3'}`}>{statusText(target)}</span>
                  </label>
                  <div role="radiogroup" aria-label={t('memory.updateMode', { target: target.name })} className={`ss-seg ${modeOff ? 'opacity-40' : ''}`}>
                    {MODES.map((mode) => <button key={mode} type="button" role="radio" aria-checked={modeOf(target) === mode} disabled={modeOff}
                      className={`font-mono ${modeOf(target) === mode ? 'on' : ''}`} onClick={() => setMode(target, mode)}>{mode}</button>)}
                  </div>
                </div>;
              })}
            </div>
          </> : <>
            {(plan.warnings ?? []).map((warning) => <div key={`${warning.code}:${warning.path}:${warning.target ?? ""}`} className="ss-note warn">{warning.path}: {warning.code === 'also_read_by' ? t('memory.alsoReadBy', { targets: (warning.targets ?? []).join(', ') }) : t('memory.overLimit', { target: warning.target ?? '', limit: warning.limit ?? 0, chars: warning.chars ?? 0 })}</div>)}
            {(plan.skipped ?? []).map((item) => <div key={item.target} className="ss-note warn">{item.target}: {t(`memory.skipped.${item.reason}`, undefined, item.reason)}</div>)}
            {plan.changes.length === 0 && <p>{t('memory.noChanges')}</p>}
            {plan.changes.map((change) => <ChangeDiff key={change.path} path={change.path} before={change.before} after={change.after} />)}
          </>}
        </div>
        <div className="df">
          <Button variant="ghost" disabled={busy} onClick={close}>{t('common.cancel')}</Button>
          {plan && <Button variant="ghost" disabled={busy} onClick={() => setPlan(null)}>{t('common.back')}</Button>}
          <Button variant="primary" loading={busy} disabled={plan ? plan.changes.length === 0 : requested.length === 0} onClick={() => plan ? void apply() : void review()}>{t(plan ? 'memory.applyConnections' : 'memory.reviewConnections')}</Button>
        </div>
      </DialogShell>
    </section>
  );
}
