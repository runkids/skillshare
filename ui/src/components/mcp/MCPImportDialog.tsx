import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Braces, Check, Download, FileUp, Info, Plus, X } from 'lucide-react';
import { mcpApi, mcpTargets, type MCPCandidate, type MCPMutation, type MCPServer } from '../../api/mcp';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import CodeEditor from '../CodeEditor';
import DialogShell from '../DialogShell';
import SegmentedControl from '../SegmentedControl';
import { Checkbox, Select } from '../Input';
import { useToast } from '../Toast';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { describeEndpoint, targetLabel } from './mcpView';

/** Where the configuration comes from. Each entry point fixes one; the dialog never switches. */
type Source = 'target' | 'paste';

const SNIPPET_PLACEHOLDER = `{
  "mcpServers": {
    "linear": { "command": "npx", "args": ["-y", "mcp-remote", "https://mcp.linear.app/sse"] }
  }
}`;
const TOML = /^\s*\[mcp_servers[.\]]/m;
const GOOSE_YAML = /^extensions\s*:/m;

/** Pretty-printed JSON, or null when the text isn't plain JSON (comments and trailing commas still import fine). */
function formatJSON(text: string) {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return null;
  }
}

interface Props {
  source: Source;
  servers: Record<string, MCPServer>;
  defaultTargets: string[];
  paths: Record<string, string>;
  detected: string[];
  /** A conflicting entry to take over: it may replace the source server of the same name. */
  conflict?: { target: string; name: string };
  /** Present when this is the paste half of "add a server", so the user can swap back to the form. */
  onMode?: (mode: 'form' | 'paste') => void;
  onClose: () => void;
  onImported: () => void;
}

export default function MCPImportDialog({ source, servers, defaultTargets, paths, detected, conflict, onMode, onClose, onImported }: Props) {
  const t = useT();
  const { toast } = useToast();
  const tab = source;
  const availableTargets = mcpTargets.filter((x) => paths[x]);
  const [from, setFrom] = useState(conflict?.target ?? availableTargets.find((x) => detected.includes(x)) ?? availableTargets[0] ?? '');
  const [content, setContent] = useState('');
  const [pasted, setPasted] = useState('');
  const [tomlFrom, setTomlFrom] = useState('codex');
  const [picked, setPicked] = useState<string[] | null>(null);
  const [targets, setTargets] = useState<string[]>(defaultTargets);
  const [saving, setSaving] = useState(false);
  const visibleTargets = new Set([...availableTargets, ...targets]);

  // Codex and Grok share one TOML shape; JSON formats are detected by the server
  const toml = TOML.test(pasted);
  const tomlInput = TOML.test(content);
  const yamlInput = GOOSE_YAML.test(content);
  const formatted = tomlInput || !content.trim() ? null : formatJSON(content);
  useEffect(() => {
    const id = window.setTimeout(() => setPasted(content.trim()), 400);
    return () => window.clearTimeout(id);
  }, [content]);

  const fromTarget = useQuery({ queryKey: ['mcp-import', from], queryFn: () => mcpApi.import({ from }), enabled: tab === 'target' && from !== '', gcTime: 0, retry: false });
  const fromPaste = useQuery({
    queryKey: ['mcp-import-paste', pasted, toml && tomlFrom],
    queryFn: () => mcpApi.import({ content: pasted, ...(toml && { from: tomlFrom }) }),
    enabled: tab === 'paste' && pasted !== '',
    gcTime: 0,
    retry: false,
  });
  const query = tab === 'target' ? fromTarget : fromPaste;
  const candidates = query.data?.candidates ?? [];

  const exists = (c: MCPCandidate) => c.name in servers && c.name !== conflict?.name;
  const importable = candidates.filter((c) => c.problems.length === 0 && !exists(c));
  // Everything importable starts ticked; a conflict import starts with just that entry
  const selected = picked ?? importable.filter((c) => !conflict || c.name === conflict.name).map((c) => c.name);
  const chosen = importable.filter((c) => selected.includes(c.name));
  const reset = () => setPicked(null);

  const showList = tab === 'target' || candidates.length > 1 || candidates.some((c) => c.problems.length || c.warnings.length || exists(c));

  // An import that matches the inherited targets keeps inheriting them, like the server dialog
  const inherited = targets.length === defaultTargets.length && targets.every((x) => defaultTargets.includes(x));

  const run = async () => {
    setSaving(true);
    const failed: string[] = [];
    for (const c of chosen) {
      // A takeover keeps the targets the existing server already has
      const own = servers[c.name]?.targets;
      const write = own ?? (inherited ? undefined : mcpTargets.filter((x) => targets.includes(x)));
      const mutation: MCPMutation = { name: c.name, server: { ...c.server, ...(write && { targets: write }) }, replace: c.name in servers };
      // Adopting records the tool's identical entry as managed, so the next sync has no conflict there
      if (c.from && (own ?? targets).includes(c.from)) mutation.resolutions = [{ target: c.from, name: c.name, action: 'adopt' }];
      try {
        await mcpApi.save(mutation);
      } catch (e) {
        failed.push(`${c.name}: ${(e as Error).message}`);
      }
    }
    const done = chosen.length - failed.length;
    if (done > 0) toast(t(done === 1 ? 'mcp.toast.imported.one' : 'mcp.toast.imported.other', { count: done }), 'success');
    if (failed.length) toast(failed.join('\n'), 'error');
    onImported();
  };

  const adding = source === 'paste';
  const title = t(adding ? 'mcp.addServer' : 'mcp.importTitle');
  const count = chosen.length;

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={saving} ariaLabel={title} className="!max-w-[640px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t(adding ? 'mcp.pasteSubtitle' : 'mcp.importSubtitle')}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={saving}><X size={16} /></button>
      </div>
      <div className="db">
        {onMode && (
          <SegmentedControl<'form' | 'paste'>
            className="self-start"
            value="paste"
            onChange={onMode}
            options={[{ value: 'form', label: t('mcp.manualTab') }, { value: 'paste', label: t('mcp.pasteTab') }]}
          />
        )}

        {tab === 'target' ? (
          <div className="ss-fld">
            <span className="text-[13px] font-semibold">{t('mcp.target')}</span>
            <Select
              value={from}
              onChange={(v) => { setFrom(v); reset(); }}
              options={availableTargets.map((x) => ({ value: x, label: `${targetLabel(x)}  ${shortenHome(paths[x])}` }))}
              disabled={saving}
            />
            <span className="hp">{t('mcp.fromTargetHint')}</span>
          </div>
        ) : (
          <>
            <div className="ss-fld">
              <span className="flex items-center justify-between">
                <span className="text-[13px] font-semibold">{t('mcp.snippet')}</span>
                <span className="flex items-center gap-4">
                  {formatted !== null && formatted !== content && (
                    <button type="button" className="flex items-center gap-1.5 text-[13px] text-ink-2 hover:text-ink" onClick={() => { setContent(formatted); reset(); }} disabled={saving}>
                      <Braces size={14} />
                      {t('mcp.format')}
                    </button>
                  )}
                  {/* The CLI takes `--file`; in a browser the file picker fills the editor and the flow is the same from there. */}
                  <label className="flex cursor-pointer items-center gap-1.5 text-[13px] text-ink-2 hover:text-ink">
                    <FileUp size={14} />
                    {t('mcp.loadFile')}
                    <input
                      type="file"
                      className="sr-only"
                      accept=".json,.jsonc,.toml,.yaml,.yml"
                      disabled={saving}
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        e.target.value = ''; // so picking the same file again still fires
                        if (file) void file.text().then((text) => { setContent(text); reset(); });
                      }}
                    />
                  </label>
                </span>
              </span>
              <CodeEditor
                value={content}
                onChange={(v) => { setContent(v); reset(); }}
                lang={tomlInput ? '' : yamlInput ? 'yaml' : 'json'}
                placeholder={SNIPPET_PLACEHOLDER}
                ariaLabel={t('mcp.snippet')}
                disabled={saving}
              />
              <span className="hp">{t('mcp.pasteHint')}</span>
            </div>
            {pasted && (
              <div className="flex items-center justify-between gap-4 text-[13px]">
                {query.error ? (
                  <span className="ss-st bad wrap min-w-0">{query.error.message}</span>
                ) : query.data ? (
                  <span className="ss-st ok">{t(candidates.length === 1 ? 'mcp.detected.one' : 'mcp.detected.other', { format: toml ? 'TOML' : GOOSE_YAML.test(pasted) ? 'YAML' : 'JSON', count: candidates.length })}</span>
                ) : (
                  <span className="text-ink-3">{t('mcp.reading')}</span>
                )}
                {toml && (
                  <span className="flex items-center gap-2.5">
                    <span className="text-ink-2">{t('mcp.tomlFrom')}</span>
                    <SegmentedControl value={tomlFrom} onChange={(v) => { setTomlFrom(v); reset(); }} options={['codex', 'grok'].map((v) => ({ value: v, label: v }))} />
                  </span>
                )}
              </div>
            )}
          </>
        )}

        {showList && (
          <div className="ss-list !shadow-none">
            <div className="ss-lh">
              <Checkbox
                hideLabel
                label={t('mcp.selectAll')}
                checked={importable.length > 0 && chosen.length === importable.length}
                indeterminate={chosen.length > 0 && chosen.length < importable.length}
                onChange={(on) => setPicked(on ? importable.map((c) => c.name) : [])}
                disabled={saving || importable.length === 0}
              />
              <span className="flex-1">
                {query.isPending && query.fetchStatus !== 'idle' ? t('mcp.reading') : t(candidates.length === 1 ? 'mcp.found.one' : 'mcp.found.other', { count: candidates.length })}
              </span>
              <span>{t('mcp.transport')}</span>
            </div>
            {tab === 'target' && query.error && <div className="ss-r text-[13px] text-bad">{query.error.message}</div>}
            {candidates.map((c) => {
              const blocked = c.problems.length > 0 || exists(c);
              const on = selected.includes(c.name) && !blocked;
              return (
                <div key={c.name} className={`ss-r !items-start !py-2.5 ${on ? 'sel' : ''} ${blocked ? 'opacity-55' : ''}`}>
                  <Checkbox
                    className="mt-0.5"
                    hideLabel
                    label={c.name}
                    checked={on}
                    onChange={(v) => setPicked(v ? [...selected, c.name] : selected.filter((n) => n !== c.name))}
                    disabled={saving || blocked}
                  />
                  <span className="flex min-w-0 flex-1 flex-col gap-px">
                    <span className="font-mono text-[13px] font-semibold">{c.name}</span>
                    <span className="truncate font-mono text-xs text-ink-3" title={describeEndpoint(c.server)}>{describeEndpoint(c.server)}</span>
                    {[...new Set(c.problems)].map((p) => <span key={p} className="text-xs text-bad">{p}</span>)}
                    {[...new Set(c.warnings)].map((w) => <span key={w} className="text-xs text-warn">{w}</span>)}
                  </span>
                  {exists(c) ? (
                    <span className="ss-st off self-center">{t('mcp.alreadyAdded')}</span>
                  ) : (
                    <span className="ss-tag self-center">{c.server.url ? 'http' : 'stdio'}</span>
                  )}
                </div>
              );
            })}
          </div>
        )}

        <div className="ss-fld">
          <span className="text-[13px] font-semibold">{t('mcp.targets')}</span>
          <div className="flex flex-wrap gap-x-5 gap-y-3">
            {mcpTargets.filter((target) => visibleTargets.has(target)).map((target) => {
              const on = targets.includes(target);
              return (
                <button
                  key={target}
                  type="button"
                  role="checkbox"
                  aria-checked={on}
                  className={`ss-tgl ${on ? 'on' : ''}`}
                  onClick={() => setTargets(on ? targets.filter((x) => x !== target) : [...targets, target])}
                  disabled={saving}
                >
                  <span className="ic"><AgentIcon target={target} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                  {targetLabel(target)}
                </button>
              );
            })}
          </div>
        </div>

        {tab === 'target' && candidates.length > 0 && (
          <div className="ss-note inf">
            <Info size={16} />
            <span className="flex-1">{t('mcp.secretsNote')}</span>
          </div>
        )}
      </div>
      <div className="df">
        {/* Name what is missing: a greyed-out button next to "writes 0 config files" reads as a bug. */}
        <span className="flex-1 text-[13px]">
          {targets.length === 0
            ? <span className="ss-st warn">{t('mcp.pickTarget')}</span>
            : <span className="text-ink-2">{t(targets.length === 1 ? 'mcp.writes.one' : 'mcp.writes.other', { count: targets.length })}</span>}
        </span>
        <Button variant="ghost" onClick={onClose} disabled={saving}>{t('common.cancel')}</Button>
        <Button variant="primary" loading={saving} disabled={count === 0 || targets.length === 0} onClick={run}>
          {adding ? <Plus size={15} /> : <Download size={15} />}
          {/* "Add 0 servers" reads as a bug before anything is pasted; the plain verb doesn't. */}
          {count === 0
            ? t(adding ? 'mcp.addServer' : 'mcp.importAction')
            : t(adding
              ? (count === 1 ? 'mcp.addCount.one' : 'mcp.addCount.other')
              : (count === 1 ? 'mcp.importCount.one' : 'mcp.importCount.other'), { count })}
        </Button>
      </div>
    </DialogShell>
  );
}
