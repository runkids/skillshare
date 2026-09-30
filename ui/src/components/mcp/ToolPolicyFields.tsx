import { useId, useState } from 'react';
import type { ChangeEvent } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { ChevronDown, ChevronRight, Download, TriangleAlert, X } from 'lucide-react';
import { mcpApi, type MCPMutation, type MCPToolPolicy } from '../../api/mcp';
import { mcpCheckApi } from '../../api/mcpCheck';
import { queryKeys } from '../../lib/queryKeys';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { Select } from '../Input';
import { serversFor } from './useMCPCheck';
import { describeMessage, hasToolPolicy, targetLabel, toolExposures, toolGapKey, toolNamePattern, toolSummary } from './mcpView';

interface ListProps { label: string; hint: string; values: string[]; suggestions: string[]; onChange: (values: string[]) => void; disabled: boolean }

/** Tool names as chips, typed or picked from the server's own list. */
function ToolList({ label, hint, values, suggestions, onChange, disabled }: ListProps) {
  const t = useT();
  const id = useId();
  const [text, setText] = useState('');
  const [error, setError] = useState('');
  const add = (raw: string) => {
    const names = raw.split(',').map((s) => s.trim()).filter(Boolean);
    const bad = names.find((name) => !toolNamePattern.test(name));
    if (bad) { setError(t('mcp.tools.badName', { name: bad })); return; }
    setError('');
    setText('');
    const next = [...values, ...names.filter((name, i) => !values.includes(name) && names.indexOf(name) === i)];
    if (next.length !== values.length) onChange(next);
  };
  // Picking from the suggestions replaces the text in one go; that is a choice, not typing.
  const typed = (e: ChangeEvent<HTMLInputElement>) => {
    const picked = !(e.nativeEvent instanceof InputEvent) || e.nativeEvent.inputType === 'insertReplacementText';
    if (picked && suggestions.includes(e.target.value)) add(e.target.value);
    else { setText(e.target.value); setError(''); }
  };
  return (
    <div className="ss-fld">
      <label htmlFor={id}>{label}</label>
      {values.length > 0 && (
        <span className="flex flex-wrap gap-1.5">
          {values.map((name) => (
            <span key={name} className="inline-flex h-7 max-w-full items-center gap-0.5 rounded-full bg-sunken pl-2.5 pr-0.5 font-mono text-xs text-ink">
              <span className="truncate">{name}</span>
              <button type="button" className="grid h-6 w-6 shrink-0 place-items-center rounded-full text-ink-2 hover:bg-sel hover:text-sel-ink" aria-label={t('mcp.tools.remove', { name })} onClick={() => onChange(values.filter((x) => x !== name))} disabled={disabled}><X size={12} /></button>
            </span>
          ))}
        </span>
      )}
      <span className={`ss-inp font-mono ${error ? 'err' : ''}`}>
        <input id={id} list={`${id}-tools`} value={text} onChange={typed} placeholder={t('mcp.tools.placeholder')} disabled={disabled}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ',') { e.preventDefault(); add(text); } }}
          onBlur={() => { if (text.trim()) add(text); }} />
      </span>
      <datalist id={`${id}-tools`}>{suggestions.filter((name) => !values.includes(name)).map((name) => <option key={name} value={name} />)}</datalist>
      {error ? <span className="hp !text-bad">{error}</span> : <span className="hp">{hint}</span>}
    </div>
  );
}

interface Props {
  tools: MCPToolPolicy;
  onChange: (tools: MCPToolPolicy) => void;
  disabled: boolean;
  /** The saved server's name, which loading its tools starts; absent for a server not saved yet. */
  savedName?: string;
  /** The mcp.projects root the server is saved under. */
  project?: string;
  /** The server as the form describes it, once it is complete enough to preview. */
  mutation?: MCPMutation;
}

/** Which of the server's tools reach the model, for every Agent at once; each Agent that cannot follow a part is named. */
export default function ToolPolicyFields({ tools, onChange, disabled, savedName, project, mutation }: Props) {
  const t = useT();
  const set = hasToolPolicy(tools);
  const [open, setOpen] = useState(set);
  const [loaded, setLoaded] = useState<{ loading?: boolean; names?: string[]; error?: string }>({});
  // The same preview the config view reads, so opening it after this costs nothing.
  const view = useQuery({ queryKey: [...queryKeys.mcp, 'render', JSON.stringify(mutation)], queryFn: () => mcpApi.render(mutation!), enabled: set && Boolean(mutation), placeholderData: keepPreviousData });
  const gaps = set ? (view.data?.rendered ?? []).filter((r) => r.toolGaps?.length && mutation?.server?.targets?.includes(r.target)) : [];

  const load = async () => {
    if (!savedName) return;
    setLoaded({ loading: true });
    try {
      const server = serversFor(await mcpCheckApi.live(savedName), project).find((s) => s.name === savedName);
      const names = server?.live?.toolNames;
      if (names?.length) setLoaded({ names: [...new Set(names)].sort() });
      else {
        const failure = server?.findings.find((f) => f.level === 'error');
        setLoaded({ error: failure ? describeMessage(t, failure.message) : t(server?.live ? 'mcp.tools.loadEmpty' : 'mcp.tools.loadFailed') });
      }
    } catch (e) {
      setLoaded({ error: (e as Error).message });
    }
  };

  return (
    <div className="ss-fld">
      <button type="button" className="ss-disc self-start" aria-expanded={open} onClick={() => setOpen(!open)}>
        {open ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
        {t('mcp.tools.title')}
        {set && <span className="font-normal text-ink-3">· {toolSummary(t, tools)}</span>}
      </button>
      {open && (
        <div className="ml-[22px] flex flex-col gap-3.5">
          <span className="hp">{t('mcp.tools.hint')}</span>
          <div className="ss-fld">
            <Select label={t('mcp.tools.expose')} value={tools.expose ?? ''} disabled={disabled}
              onChange={(value) => onChange({ ...tools, expose: (value || undefined) as MCPToolPolicy['expose'] })}
              options={[{ value: '', label: t('mcp.tools.exposeNone'), note: `· ${t('mcp.tools.exposeNoneNote')}` }, ...toolExposures.map((value) => ({ value, label: value, note: `· ${t(`mcp.tools.exposeNote.${value}`)}` }))]} />
          </div>
          <div className="grid grid-cols-2 gap-3.5">
            <ToolList label={t('mcp.tools.allow')} hint={t('mcp.tools.allowHint')} values={tools.allow ?? []} suggestions={loaded.names ?? []} onChange={(allow) => onChange({ ...tools, allow })} disabled={disabled} />
            <ToolList label={t('mcp.tools.deny')} hint={t('mcp.tools.denyHint')} values={tools.deny ?? []} suggestions={loaded.names ?? []} onChange={(deny) => onChange({ ...tools, deny })} disabled={disabled} />
          </div>
          <div className="flex flex-wrap items-center gap-2.5">
            <Button variant="secondary" size="sm" onClick={() => void load()} loading={loaded.loading} disabled={disabled || !savedName}>{!loaded.loading && <Download size={14} />}{t('mcp.tools.load')}</Button>
            <span className={`text-xs ${loaded.error ? 'text-bad' : 'text-ink-3'}`}>
              {!savedName ? t('mcp.tools.loadAfterSave') : loaded.error ?? (loaded.names ? t(loaded.names.length === 1 ? 'mcp.tools.loaded.one' : 'mcp.tools.loaded.other', { count: loaded.names.length }) : t('mcp.tools.loadHint'))}
            </span>
          </div>
          {gaps.length > 0 && (
            <div className="ss-note warn">
              <TriangleAlert size={16} className="mt-0.5 shrink-0" />
              <div className="flex flex-1 flex-col gap-1">
                <span>{t('mcp.tools.notApplied')}</span>
                {gaps.map((r) => (
                  <span key={r.target} className="flex items-center gap-1.5">
                    <AgentIcon target={r.target} size={14} />
                    {t('mcp.tools.notAppliedFor', { agent: targetLabel(r.target), parts: r.toolGaps!.map((gap) => t(toolGapKey(gap))).join(t('mcp.tools.partSeparator')) })}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
