import { useState } from 'react';
import { Check, KeyRound, Link2, Plus, SquareTerminal, X } from 'lucide-react';
import { mcpApi, mcpTargets, type MCPServer, type MCPValue } from '../../api/mcp';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import DialogShell from '../DialogShell';
import SegmentedControl from '../SegmentedControl';
import { Select } from '../Input';
import { useT } from '../../i18n';
import { joinCommand, splitCommand, targetLabel } from './mcpView';

// Mirrors mcp.serverName
const NAME = /^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/;

interface EnvRow { id: string; key: string; fromEnv: boolean; value: string }

const envRows = (env: Record<string, MCPValue> = {}): EnvRow[] =>
  Object.entries(env).map(([key, v]) => ({ id: crypto.randomUUID(), key, fromEnv: typeof v !== 'string', value: typeof v === 'string' ? v : v.fromEnv }));

interface Props {
  initial?: { name: string; server: MCPServer };
  /** Inherited `mcp.targets`, used when the server has none of its own. */
  defaultTargets: string[];
  existingNames: string[];
  availableTargets?: readonly string[];
  onClose: () => void;
  onSaved: () => void;
}

/** Add or edit one source server. Saving only changes the source; Sync writes the config files. */
export default function MCPServerDialog({ initial, defaultTargets, existingNames, availableTargets = mcpTargets, onClose, onSaved }: Props) {
  const t = useT();
  const server = initial?.server;
  const [name, setName] = useState(initial?.name ?? '');
  const [http, setHttp] = useState(Boolean(server?.url));
  const [command, setCommand] = useState(server?.command ? joinCommand([server.command, ...(server.args ?? [])]) : '');
  const [env, setEnv] = useState(() => envRows(server?.env));
  const [url, setUrl] = useState(server?.url ?? '');
  const [tokenEnv, setTokenEnv] = useState(server?.bearerToken?.fromEnv ?? '');
  const [targets, setTargets] = useState<string[]>(server?.targets ?? defaultTargets);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  const trimmed = name.trim();
  const taken = !initial && existingNames.includes(trimmed);
  const nameError = trimmed && !NAME.test(trimmed) ? t('mcp.nameHint') : taken ? t('mcp.nameTaken') : '';
  const words = splitCommand(command);
  const canSave = Boolean(trimmed) && !nameError && targets.length > 0 && (http ? url.trim() !== '' : words.length > 0) && !saving;
  const title = t(initial ? 'mcp.editServer' : 'mcp.addServer');
  const visibleTargets = new Set([...availableTargets, ...targets]);

  const save = async () => {
    if (!canSave) return;
    const [cmd, ...args] = words;
    const rows = env.filter((r) => r.key.trim());
    const next: MCPServer = http
      ? {
          url: url.trim(),
          ...(tokenEnv.trim() && { bearerToken: { fromEnv: tokenEnv.trim() } }),
          // Headers have no fields here; keep the ones the server already has
          ...(server?.url && server.headers && { headers: server.headers }),
        }
      : {
          command: cmd,
          ...(args.length > 0 && { args }),
          ...(rows.length > 0 && { env: Object.fromEntries(rows.map((r) => [r.key.trim(), r.fromEnv ? { fromEnv: r.value.trim() } : r.value])) }),
        };
    const transport = http ? 'streamable-http' : 'stdio';
    if (server?.transport === transport) next.transport = transport;
    // A server without its own targets keeps inheriting while the selection matches
    const inherited = !server?.targets && targets.length === defaultTargets.length && targets.every((x) => defaultTargets.includes(x));
    if (!inherited) next.targets = mcpTargets.filter((x) => targets.includes(x));
    setSaving(true);
    setError('');
    try {
      await mcpApi.save({ name: trimmed, server: next, replace: Boolean(initial) });
      onSaved();
    } catch (e) {
      setError((e as Error).message);
      setSaving(false);
    }
  };

  const patchRow = (id: string, patch: Partial<EnvRow>) => setEnv(env.map((r) => (r.id === id ? { ...r, ...patch } : r)));

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={saving} ariaLabel={title} className="!max-w-[720px]">
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={saving}><X size={16} /></button>
      </div>
      <form id="mcp-server" className="db" onSubmit={(e) => { e.preventDefault(); void save(); }}>
        <div className="grid grid-cols-2 gap-3.5">
          <div className="ss-fld">
            <label htmlFor="mcp-name">{t('mcp.name')}</label>
            <span className={`ss-inp ${nameError ? 'err' : ''}`}>
              <input id="mcp-name" autoFocus={!initial} value={name} onChange={(e) => setName(e.target.value)} placeholder="github" disabled={Boolean(initial) || saving} />
            </span>
            {nameError && <span className="hp !text-bad">{nameError}</span>}
          </div>
          <div className="ss-fld">
            <span className="text-[13px] font-semibold">{t('mcp.transport')}</span>
            <SegmentedControl
              className="self-start"
              value={http ? 'streamable-http' : 'stdio'}
              onChange={(v) => setHttp(v === 'streamable-http')}
              options={[{ value: 'stdio', label: 'stdio' }, { value: 'streamable-http', label: 'streamable-http' }]}
            />
          </div>
        </div>

        {http ? (
          <>
            <div className="ss-fld">
              <label htmlFor="mcp-url">URL</label>
              <span className="ss-inp font-mono">
                <Link2 size={15} className="shrink-0 text-ink-3" />
                <input id="mcp-url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/mcp" disabled={saving} />
              </span>
            </div>
            <div className="ss-fld">
              <label htmlFor="mcp-token">{t('mcp.tokenEnv')}</label>
              <span className="ss-inp font-mono">
                <KeyRound size={15} className="shrink-0 text-ink-3" />
                <input id="mcp-token" value={tokenEnv} onChange={(e) => setTokenEnv(e.target.value)} placeholder="DOCS_API_KEY" disabled={saving} />
              </span>
              <span className="hp">{t('mcp.tokenEnvHint')}</span>
            </div>
          </>
        ) : (
          <>
            <div className="ss-fld">
              <label htmlFor="mcp-command">{t('mcp.command')}</label>
              <span className="ss-inp font-mono">
                <SquareTerminal size={15} className="shrink-0 text-ink-3" />
                <input id="mcp-command" value={command} onChange={(e) => setCommand(e.target.value)} placeholder="npx -y @modelcontextprotocol/server-github" disabled={saving} />
              </span>
              <span className="hp">{t('mcp.commandHint')}</span>
            </div>
            <div className="ss-fld">
              <span className="text-[13px] font-semibold">{t('mcp.environment')}</span>
              <div className="ss-list !shadow-none">
                {env.map((r) => (
                  <div key={r.id} className="ss-r !min-h-12 !px-3 !py-1.5">
                    <span className="ss-inp min-w-0 flex-1 font-mono">
                      <input value={r.key} onChange={(e) => patchRow(r.id, { key: e.target.value })} placeholder="API_KEY" aria-label={t('mcp.envName')} disabled={saving} />
                    </span>
                    <Select
                      className="w-[170px] shrink-0"
                      value={r.fromEnv ? 'env' : 'value'}
                      onChange={(v) => patchRow(r.id, { fromEnv: v === 'env' })}
                      options={[{ value: 'env', label: t('mcp.fromEnvironment') }, { value: 'value', label: t('mcp.literalValue') }]}
                      disabled={saving}
                    />
                    <span className="ss-inp min-w-0 flex-1 font-mono">
                      <input value={r.value} onChange={(e) => patchRow(r.id, { value: e.target.value })} placeholder={r.fromEnv ? 'VARIABLE' : ''} aria-label={t(r.fromEnv ? 'mcp.fromEnvironment' : 'mcp.literalValue')} disabled={saving} />
                    </span>
                    <button type="button" className="ss-ib shrink-0" aria-label={t('mcp.removeVariable')} onClick={() => setEnv(env.filter((x) => x.id !== r.id))} disabled={saving}><X size={16} /></button>
                  </div>
                ))}
                <div className="ss-r !min-h-10">
                  <button type="button" className="flex items-center gap-[7px] text-[13px] text-ink-2 hover:text-ink" onClick={() => setEnv([...env, { id: crypto.randomUUID(), key: '', fromEnv: true, value: '' }])} disabled={saving}>
                    <Plus size={14} />
                    {t('mcp.addVariable')}
                  </button>
                </div>
              </div>
              <span className="hp">{t('mcp.environmentHint')}</span>
            </div>
          </>
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
                  disabled={saving || (target === 'claude-desktop' && http && !on)}
                >
                  <span className="ic"><AgentIcon target={target} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                  {targetLabel(target)}{target === 'claude-desktop' && <span className="ss-tag">stdio</span>}
                </button>
              );
            })}
          </div>
        </div>
        {error && <div className="ss-note bad"><span className="flex-1">{error}</span></div>}
      </form>
      <div className="df">
        <span className="flex-1 text-[13px] text-ink-2">{t(targets.length === 1 ? 'mcp.writes.one' : 'mcp.writes.other', { count: targets.length })}</span>
        <Button variant="ghost" onClick={onClose} disabled={saving}>{t('common.cancel')}</Button>
        <Button type="submit" form="mcp-server" variant="primary" loading={saving} disabled={!canSave}>{t('common.save')}</Button>
      </div>
    </DialogShell>
  );
}
