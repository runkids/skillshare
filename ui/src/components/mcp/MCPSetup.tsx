import { useId, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, ArrowLeft, ArrowRight, Braces, Check, Download, Link, RefreshCw, ShieldCheck, X, XCircle } from 'lucide-react';
import { mcpApi, mcpTargets, type MCPCandidate, type MCPMutation, type MCPPlan, type MCPServer } from '../../api/mcp';
import Badge from '../Badge';
import Button from '../Button';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import SegmentedControl from '../SegmentedControl';
import { Checkbox, Input, Textarea } from '../Input';
import AgentIcon from '../AgentIcon';
import MCPPreview, { type MCPResolve } from './MCPPreview';
import { describeEndpoint } from './mcpView';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';

export type MCPSetupMode = 'url' | 'json' | 'import';

interface Props {
  initial?: { name: string; server: MCPServer };
  initialMode?: MCPSetupMode;
  /** Opens the import step for a conflicting Agent entry. */
  importFrom?: { target: string; name: string };
  /** Inherited `mcp.targets`; saving this exact selection keeps the server inheriting it. */
  defaultTargets: string[];
  /** The server's own `targets`, which stay explicit on save. */
  explicitTargets?: string[];
  existingNames?: string[];
  paths?: Record<string, string>;
  onClose: () => void;
  onSaved: (backups: string[]) => void;
}

function Step({ n, label, state }: { n: number; label: string; state: 'done' | 'current' | 'todo' }) {
  return <li aria-current={state === 'current' ? 'step' : undefined} className={`flex items-center gap-1.5 ${state === 'current' ? 'font-semibold text-pencil' : 'text-pencil-light'}`}>
    <span className={`w-5 h-5 rounded-full text-[11px] flex items-center justify-center ${state === 'current' ? 'bg-pencil text-paper' : state === 'done' ? 'bg-success-light text-success' : 'border border-muted-dark'}`}>
      {state === 'done' ? <Check size={11} strokeWidth={3} aria-hidden="true" /> : n}
    </span>
    {label}
  </li>;
}

export default function MCPSetup({ initial, initialMode, importFrom, defaultTargets, explicitTargets, existingNames = [], paths = {}, onClose, onSaved }: Props) {
  const t = useT();
  const ids = useId();
  const simpleRemote = initial?.server.url && !Object.keys(initial.server.headers ?? {}).length;
  const [mode, setMode] = useState<MCPSetupMode | 'edit'>(initial ? (simpleRemote ? 'url' : 'edit') : importFrom ? 'import' : initialMode ?? 'url');
  const [name, setName] = useState(initial?.name ?? importFrom?.name ?? '');
  const [input, setInput] = useState(simpleRemote ? initial.server.url! : initial ? JSON.stringify(initial.server, null, 2) : '');
  const [tokenEnv, setTokenEnv] = useState(initial?.server.bearerToken?.fromEnv ?? '');
  const [replaceExisting, setReplaceExisting] = useState(Boolean(importFrom));
  const [from, setFrom] = useState(importFrom?.target ?? 'claude');
  const [tomlFrom, setTomlFrom] = useState('codex');
  const [targets, setTargets] = useState<string[]>(explicitTargets ?? defaultTargets);
  const [pasted, setPasted] = useState<MCPCandidate[]>([]);
  const [picked, setPicked] = useState(importFrom?.name ?? '');
  const [preview, setPreview] = useState<{ plan: MCPPlan; mutation: MCPMutation } | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const imported = useQuery({ queryKey: ['mcp-import', from], queryFn: () => mcpApi.import({ from }), enabled: mode === 'import', gcTime: 0 });

  const candidates = mode === 'import' ? imported.data?.candidates ?? [] : pasted;
  const candidate = candidates.find(c => c.name === picked) ?? null;
  const selectedTargets = new Set(targets);
  const listed = mode === 'import' || (mode === 'json' && pasted.length > 0);
  // JSON formats are detected by the server; Codex and Grok share one TOML shape.
  const toml = mode === 'json' && /^\s*\[mcp_servers[.\]]/m.test(input);
  const reportError = (e: unknown) => setError(e instanceof Error ? e.message : t('common.error.generic'));

  const switchMode = (next: MCPSetupMode) => { setMode(next); setPicked(''); setPasted([]); setInput(''); };
  const pick = (next: MCPCandidate) => { setPicked(next.name); setName(next.name); };
  const readPasted = async () => {
    setBusy(true); setError('');
    try {
      setPasted((await mcpApi.import({ ...(toml ? { from: tomlFrom } : {}), content: input, name })).candidates);
      setPicked('');
      // Credential-bearing pasted input is no longer needed after parsing.
      setInput('');
    } catch (e) { reportError(e); } finally { setBusy(false); }
  };
  const runPreview = async (mutation: MCPMutation) => {
    setBusy(true); setError('');
    try { setPreview({ plan: await mcpApi.preview(mutation), mutation }); }
    catch (e) { reportError(e); } finally { setBusy(false); }
  };
  const buildPreview = () => {
    let server: MCPServer;
    try {
      if (mode === 'url') server = { url: input.trim(), ...(tokenEnv.trim() ? { bearerToken: { fromEnv: tokenEnv.trim() } } : {}) };
      else if (mode === 'edit') server = JSON.parse(input) as MCPServer;
      else {
        if (!candidate || candidate.problems.length) throw new Error(t('mcp.chooseServer'));
        server = candidate.server;
      }
    } catch (e) { reportError(e); return; }
    const inherited = !explicitTargets && targets.length === defaultTargets.length && targets.every(target => defaultTargets.includes(target));
    const mutation: MCPMutation = { name, server: { ...server, targets: inherited ? undefined : targets }, replace: Boolean(initial) || replaceExisting };
    // Adopt takes ownership of an identical Agent entry without rewriting it; a differing entry stays a conflict.
    if (candidate?.from && selectedTargets.has(candidate.from)) mutation.resolutions = [{ target: candidate.from, name: candidate.name, action: 'adopt' }];
    void runPreview(mutation);
  };
  const resolve: MCPResolve = (target, conflictName, action) => {
    if (action === 'import') {
      setPreview(null); setMode('import'); setFrom(target); setPicked(conflictName); setName(conflictName); setReplaceExisting(true);
      return;
    }
    const mutation = preview!.mutation;
    // One resolution per entry: Replace overrides an earlier adopt.
    const others = (mutation.resolutions ?? []).filter(r => r.target !== target || r.name !== conflictName);
    void runPreview({ ...mutation, resolutions: [...others, { target, name: conflictName, action: 'replace' }] });
  };
  const save = async (sync: boolean) => {
    if (!preview) return;
    setBusy(true); setError('');
    try { onSaved((await mcpApi.configure(preview.mutation, preview.plan.revision, sync)).backupIds ?? []); }
    catch (e) { reportError(e); } finally { setBusy(false); }
  };

  const title = initial ? t('mcp.edit') : t('mcp.add');
  const agentLabel = (target: string) => <span className="inline-flex items-center gap-1.5"><AgentIcon target={target} size={14} />{target}</span>;
  const methodLabel = (Icon: typeof Link, key: string) => <span className="inline-flex items-center gap-1.5"><Icon size={14} aria-hidden="true" />{t(key)}</span>;

  return <DialogShell open onClose={onClose} preventClose={busy} maxWidth="2xl" ariaLabel={title}>
    <div className="flex flex-col gap-5 max-h-[80vh]">
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <h2 className="text-xl font-semibold">{title}</h2>
          <ol className="flex items-center gap-2 text-sm">
            <Step n={1} label={t('mcp.stepSetup')} state={preview ? 'done' : 'current'} />
            <li aria-hidden="true" className="w-5 border-t border-muted-dark" />
            <Step n={2} label={t('mcp.stepPreview')} state={preview ? 'current' : 'todo'} />
          </ol>
        </div>
        <IconButton icon={<X size={16} strokeWidth={2.5} />} label={t('common.close')} disabled={busy} onClick={onClose} />
      </div>
      <div className="space-y-5 overflow-auto min-h-0 -mx-1 px-1 pb-1">
      {error ? <p role="alert" className="text-danger">{error}</p> : null}

      {preview ? <>
        <MCPPreview plan={preview.plan} busy={busy} onResolve={resolve} />
        <div className="flex flex-wrap items-center justify-between gap-3 pt-4 border-t border-dashed border-pencil-light/30">
          <p className="flex items-center gap-1.5 text-xs text-pencil-light"><ShieldCheck size={14} className="text-success" aria-hidden="true" />{t('mcp.backupNote')}</p>
          <div className="flex flex-wrap gap-2">
            <Button variant="ghost" disabled={busy} onClick={() => setPreview(null)}><ArrowLeft size={16} aria-hidden="true" />{t('common.back')}</Button>
            <Button variant="secondary" loading={busy} onClick={() => save(false)}>{t('mcp.saveOnly')}</Button>
            <Button disabled={preview.plan.blocked} loading={busy} onClick={() => save(true)}>{t('mcp.saveSync')}</Button>
          </div>
        </div>
      </> : <>
        {!initial ? <div role="group" aria-labelledby={`${ids}-method`} className="space-y-2">
          <p id={`${ids}-method`} className="text-pencil-light">{t('mcp.method')}</p>
          <SegmentedControl<MCPSetupMode> connected value={mode as MCPSetupMode} onChange={switchMode} options={[
            { value: 'url', label: methodLabel(Link, 'mcp.url') },
            { value: 'json', label: methodLabel(Braces, 'mcp.paste') },
            { value: 'import', label: methodLabel(Download, 'mcp.import') },
          ]} />
        </div> : null}

        <div className={mode === 'url' ? 'grid gap-4 sm:grid-cols-2' : ''}>
          <div>
            <Input label={t('mcp.name')} value={name} disabled={Boolean(initial)} aria-describedby={`${ids}-name`} onChange={e => setName(e.target.value)} />
            <p id={`${ids}-name`} className="mt-1 text-xs text-pencil-light">{t('mcp.nameHint')}</p>
          </div>
          {mode === 'url' ? <div>
            <Input label={t('mcp.tokenEnv')} className="font-mono" value={tokenEnv} aria-describedby={`${ids}-token`} onChange={e => setTokenEnv(e.target.value)} placeholder="DOCS_TOKEN" />
            <p id={`${ids}-token`} className="mt-1 text-xs text-pencil-light">{t('mcp.tokenEnvHint')}</p>
          </div> : null}
        </div>
        {mode === 'url' ? <div>
          <Input label={t('mcp.url')} className="font-mono" value={input} aria-describedby={`${ids}-url`} onChange={e => setInput(e.target.value)} placeholder="https://example.com/mcp" />
          <p id={`${ids}-url`} className="mt-1 flex items-center gap-1.5 text-xs text-pencil-light"><ShieldCheck size={13} className="text-success shrink-0" aria-hidden="true" />{t('mcp.urlHint')}</p>
        </div> : null}

        {mode === 'import' ? <div role="group" aria-labelledby={`${ids}-from`} className="space-y-2">
          <p id={`${ids}-from`} className="text-pencil-light">{t('mcp.from')}</p>
          <SegmentedControl value={from} onChange={setFrom} options={mcpTargets.map(value => ({ value, label: agentLabel(value) }))} />
        </div> : null}
        {mode === 'json' || mode === 'edit' ? <div>
          <Textarea label={mode === 'edit' ? t('mcp.definition') : t('mcp.paste')} rows={7} className="font-mono" value={input} aria-describedby={mode === 'json' ? `${ids}-paste` : undefined} onChange={e => setInput(e.target.value)} />
          {mode === 'json' ? <p id={`${ids}-paste`} className="mt-1 text-xs text-pencil-light">{t('mcp.pasteHint')}</p> : null}
        </div> : null}
        {toml ? <div role="group" aria-labelledby={`${ids}-toml`} className="space-y-2">
          <p id={`${ids}-toml`} className="text-pencil-light">{t('mcp.tomlFrom')}</p>
          <SegmentedControl value={tomlFrom} onChange={setTomlFrom} options={['codex', 'grok'].map(value => ({ value, label: agentLabel(value) }))} />
        </div> : null}
        {mode === 'json' ? <Button variant="secondary" loading={busy} disabled={!input.trim()} onClick={readPasted}>{t('mcp.read')}</Button> : null}

        {listed ? <div role="radiogroup" aria-labelledby={`${ids}-candidates`} className="space-y-2">
          <div className="flex items-center justify-between gap-3">
            <div id={`${ids}-candidates`} className="min-w-0 text-pencil-light">
              <p className="text-sm">{mode === 'import' && imported.data ? t('mcp.foundServers', { count: candidates.length }) : t('mcp.chooseServer')}</p>
              {mode === 'import' && paths[from] ? <p className="font-mono text-xs truncate" title={paths[from]}>{shortenHome(paths[from])}</p> : null}
            </div>
            {mode === 'import' ? <Button size="sm" variant="ghost" className="shrink-0" loading={imported.isFetching} onClick={() => void imported.refetch()}><RefreshCw size={14} aria-hidden="true" />{t('mcp.reread')}</Button> : null}
          </div>
          {imported.error && mode === 'import' ? <p role="alert" className="text-danger">{imported.error.message}</p> : null}
          {candidates.map(c => {
            const blocked = c.problems.length > 0;
            const selected = picked === c.name;
            return <label key={c.name} className={`flex items-start gap-3 p-3 border rounded-[var(--radius-md)] transition-colors ${blocked ? 'border-dashed border-muted cursor-not-allowed' : selected ? 'border-pencil ring-1 ring-pencil bg-paper cursor-pointer' : 'border-muted hover:border-muted-dark cursor-pointer'}`}>
              <input type="radio" name={`${ids}-candidate`} className="mt-1 accent-[var(--color-pencil)]" checked={selected} disabled={blocked} onChange={() => pick(c)} />
              <span className="flex-1 min-w-0 space-y-1">
                <span className="flex flex-wrap items-center gap-2">
                  <span className={`font-semibold ${blocked ? 'text-pencil-light' : ''}`}>{c.name}</span>
                  {blocked ? null : <Badge>{t(c.server.url ? 'mcp.remote' : 'mcp.local')}</Badge>}
                  {existingNames.includes(c.name) ? <Badge variant="info">{t('mcp.alreadyInSource')}</Badge> : null}
                </span>
                {describeEndpoint(c.server) ? <span className="block font-mono text-xs text-pencil-light truncate">{describeEndpoint(c.server)}</span> : null}
                {[...new Set(c.problems)].map(problem => <span key={problem} className="flex items-start gap-1.5 text-xs text-danger"><XCircle size={13} className="shrink-0 mt-px" aria-hidden="true" />{problem}</span>)}
                {[...new Set(c.warnings)].map(warning => <span key={warning} className="flex items-start gap-1.5 text-xs text-warning"><AlertTriangle size={13} className="shrink-0 mt-px" aria-hidden="true" />{warning}</span>)}
              </span>
            </label>;
          })}
        </div> : null}

        {!initial && existingNames.includes(name) ? <Checkbox label={t('mcp.replaceSource')} checked={replaceExisting} onChange={setReplaceExisting} /> : null}

        <div role="group" aria-labelledby={`${ids}-targets`} className="space-y-2">
          <div className="flex items-baseline justify-between gap-2">
            <p id={`${ids}-targets`} className="text-pencil-light">{t('mcp.targets')}</p>
            <span className="text-xs text-pencil-light">{t('mcp.selectedCount', { count: targets.length })}</span>
          </div>
          <div className="grid gap-2 sm:grid-cols-2">
            {mcpTargets.map(target => {
              const checked = selectedTargets.has(target);
              return <div key={target} className={`min-w-0 px-3 py-2.5 border rounded-[var(--radius-md)] transition-colors ${checked ? 'border-pencil ring-1 ring-pencil bg-paper' : 'border-muted'}`}>
                <div className="flex items-center justify-between gap-2">
                  <Checkbox size="sm" label={target} checked={checked} onChange={on => setTargets(prev => on ? [...prev, target] : prev.filter(item => item !== target))} />
                  <AgentIcon target={target} />
                </div>
                {paths[target] ? <p className="mt-1 ps-6 font-mono text-[11px] text-pencil-light truncate" title={paths[target]}>{shortenHome(paths[target])}</p> : null}
              </div>;
            })}
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3 pt-4 border-t border-dashed border-pencil-light/30">
          <p className="text-xs text-pencil-light">{t('mcp.previewHint')}</p>
          <div className="flex gap-2">
            <Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
            <Button loading={busy} disabled={!name || !targets.length || ((mode === 'json' || mode === 'import') && (!candidate || candidate.problems.length > 0))} onClick={buildPreview}>
              {t('mcp.preview')}<ArrowRight size={16} aria-hidden="true" />
            </Button>
          </div>
        </div>
      </>}
      </div>
    </div>
  </DialogShell>;
}
