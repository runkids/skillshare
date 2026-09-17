import { useState } from 'react';
import { mcpApi, mcpTargets, type MCPCandidate, type MCPMutation, type MCPPlan, type MCPServer } from '../../api/mcp';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { Checkbox, Input, Select, Textarea } from '../Input';
import MCPPreview from './MCPPreview';
import { useT } from '../../i18n';

interface Props {
  initial?: { name: string; server: MCPServer };
  defaultTargets: string[];
  existingNames?: string[];
  onClose: () => void;
  onSaved: (backups: string[]) => void;
}

export default function MCPSetup({ initial, defaultTargets, existingNames = [], onClose, onSaved }: Props) {
  const t = useT();
  const simpleRemote = initial?.server.url && !Object.keys(initial.server.headers ?? {}).length;
  const [mode, setMode] = useState(initial && !simpleRemote ? 'edit' : 'url');
  const [name, setName] = useState(initial?.name ?? '');
  const [input, setInput] = useState(simpleRemote ? initial.server.url! : initial ? JSON.stringify(initial.server, null, 2) : '');
  const [tokenEnv, setTokenEnv] = useState(initial?.server.bearerToken?.fromEnv ?? '');
  const [replaceExisting, setReplaceExisting] = useState(false);
  const [from, setFrom] = useState('claude');
  const [targets, setTargets] = useState<string[]>(initial?.server.targets ?? defaultTargets);
  const [candidates, setCandidates] = useState<MCPCandidate[]>([]);
  const [candidate, setCandidate] = useState<MCPCandidate | null>(null);
  const [preview, setPreview] = useState<{ plan: MCPPlan; mutation: MCPMutation } | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const reportError = (e: unknown) => setError(e instanceof Error ? e.message : t('common.error.generic'));
  const loadCandidates = async () => {
    setBusy(true); setError('');
    try {
    const result = await mcpApi.import(mode === 'import' ? { from } : { from, content: input, name });
    setCandidates(result.candidates);
    setCandidate(null);
    // Credential-bearing pasted input is no longer needed after parsing.
    setInput('');
    } catch (e) { reportError(e); } finally { setBusy(false); }
  };
  const buildPreview = async () => {
    setBusy(true); setError('');
    try {
    let server: MCPServer;
    if (mode === 'url') server = { url: input.trim(), ...(tokenEnv.trim() ? { bearerToken: { fromEnv: tokenEnv.trim() } } : {}) };
    else if (mode === 'edit') server = JSON.parse(input) as MCPServer;
    else {
      if (!candidate || candidate.problems.length) throw new Error(t('mcp.chooseServer'));
      server = candidate.server;
    }
    const mutation: MCPMutation = { name, server: { ...server, targets }, replace: Boolean(initial) || replaceExisting };
    if (candidate?.from && selectedTargets.has(candidate.from)) mutation.resolutions = [{ target: candidate.from, name: candidate.name, action: 'replace' }];
    const plan = await mcpApi.preview(mutation);
    setPreview({ plan, mutation });
    } catch (e) { reportError(e); } finally { setBusy(false); }
  };
  const save = async (sync: boolean) => {
    if (!preview) return;
    setBusy(true); setError('');
    try {
    const result = await mcpApi.configure(preview.mutation, preview.plan.revision, sync);
    onSaved(result.backupIds ?? []);
    } catch (e) { reportError(e); } finally { setBusy(false); }
  };
  const selectedTargets = new Set(targets);

  return <DialogShell open onClose={onClose} preventClose={busy} maxWidth="2xl" ariaLabel={initial ? t('mcp.edit') : t('mcp.add')}>
    <div className="space-y-4 max-h-[80vh] overflow-auto">
      <h2 className="text-xl font-semibold">{initial ? t('mcp.edit') : t('mcp.add')}</h2>
      <p className="text-sm text-pencil-light">{t('mcp.setupHint')}</p>
      {error ? <p role="alert" className="text-danger">{error}</p> : null}
      {preview ? <>
        <MCPPreview plan={preview.plan} />
        {preview.plan.blocked ? <p className="text-warning">{t('mcp.conflictHint')}</p> : null}
        <div className="flex flex-wrap gap-2">
          <Button variant="ghost" disabled={busy} onClick={() => setPreview(null)}>{t('common.back')}</Button>
          <Button variant="secondary" loading={busy} onClick={() => save(false)}>{t('mcp.saveOnly')}</Button>
          <Button disabled={preview.plan.blocked} loading={busy} onClick={() => save(true)}>{t('mcp.saveSync')}</Button>
        </div>
      </> : <>
        {!initial ? <Select label={t('mcp.method')} value={mode} onChange={value => { setMode(value); setCandidate(null); setCandidates([]); setInput(''); }} options={[
          { value: 'url', label: t('mcp.url') }, { value: 'json', label: t('mcp.paste') }, { value: 'import', label: t('mcp.import') },
        ]} /> : null}
        <Input label={t('mcp.name')} value={name} disabled={Boolean(initial)} onChange={e => setName(e.target.value)} />
        {mode === 'url' ? <Input label={t('mcp.url')} value={input} onChange={e => setInput(e.target.value)} placeholder="https://example.com/mcp" /> : null}
        {mode === 'url' ? <Input label={t('mcp.tokenEnv')} value={tokenEnv} onChange={e => setTokenEnv(e.target.value)} placeholder="DOCS_TOKEN" /> : null}
        {mode === 'json' || mode === 'import' ? <Select label={t('mcp.from')} value={from} onChange={setFrom} options={mcpTargets.map(value => ({ value, label: value }))} /> : null}
        {mode === 'json' || mode === 'edit' ? <Textarea label={mode === 'edit' ? t('mcp.definition') : t('mcp.paste')} rows={7} value={input} onChange={e => setInput(e.target.value)} /> : null}
        {mode === 'json' || mode === 'import' ? <Button variant="secondary" loading={busy} onClick={loadCandidates}>{t('mcp.read')}</Button> : null}
        {candidates.length ? <Select label={t('mcp.chooseServer')} value={candidate?.name ?? ''} onChange={value => { const next = candidates.find(c => c.name === value) ?? null; setCandidate(next); if (next) setName(next.name); }} options={[{ value: '', label: t('mcp.chooseServer') }, ...candidates.map(c => ({ value: c.name, label: c.name }))]} /> : null}
        {[...new Set(candidate?.problems)].map(problem => <p key={problem} role="alert" className="text-danger">{problem}</p>)}
        {[...new Set(candidate?.warnings)].map(warning => <p key={warning} className="text-warning">{warning}</p>)}
        {!initial && existingNames.includes(name) ? <Checkbox label={t('mcp.replaceSource')} checked={replaceExisting} onChange={setReplaceExisting} /> : null}
        <fieldset className="space-y-2">
          <legend className="text-pencil-light mb-2">{t('mcp.targets')}</legend>
          <div className="flex flex-wrap gap-4">{mcpTargets.map(target => <Checkbox key={target} label={target} checked={selectedTargets.has(target)} onChange={checked => setTargets(prev => checked ? [...prev, target] : prev.filter(t => t !== target))} />)}</div>
        </fieldset>
        <div className="flex flex-wrap gap-2">
          <Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
          <Button loading={busy} disabled={!name || !targets.length || ((mode === 'json' || mode === 'import') && (!candidate || Boolean(candidate.problems.length)))} onClick={buildPreview}>{t('mcp.preview')}</Button>
        </div>
      </>}
    </div>
  </DialogShell>;
}
