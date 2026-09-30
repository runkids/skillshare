import { useId, useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { Check, Download, RefreshCw, Search, X } from 'lucide-react';
import { mcpApi, type MCPMutation, type MCPToolPolicy } from '../../api/mcp';
import { mcpCheckApi } from '../../api/mcpCheck';
import { queryKeys } from '../../lib/queryKeys';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import Spinner from '../Spinner';
import Tooltip from '../Tooltip';
import { InfoTip } from './PiSettingsFields';
import { hasToolPolicy, setToolChecked, targetLabel, toolAllowed, toolBlockedBy, toolGapKey, toolNamePattern, toolOutcomes, toolRuleExample, toolRules, toolSummary } from './mcpView';

interface ListProps { label: string; hint: string; placeholder: string; values: string[]; onChange: (values: string[]) => void; disabled: boolean }

/** A row of tool names or patterns as chips, typed into a borderless input. */
function ToolList({ label, hint, placeholder, values, onChange, disabled }: ListProps) {
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
  return (
    <div className="flex flex-col gap-1 px-3.5 py-2">
      <div className="flex min-h-7 flex-wrap items-center gap-2">
        <span className="flex shrink-0 items-center gap-0.5">
          <label htmlFor={id} className="text-[13px] text-ink-2">{label}</label>
          <InfoTip label={t('mcp.tools.aboutRules', { label })} content={hint} />
        </span>
        {values.map((name) => (
          <span key={name} className="inline-flex h-6 max-w-full items-center gap-1 rounded-full bg-sunken pl-[9px] pr-1 font-mono text-xs text-ink">
            <span className="truncate">{name}</span>
            <button type="button" className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full text-ink-3 hover:text-ink" aria-label={t('mcp.tools.remove', { name })} onClick={() => onChange(values.filter((x) => x !== name))} disabled={disabled}><X size={11} /></button>
          </span>
        ))}
        <input id={id} className="min-w-[120px] flex-1 bg-transparent font-mono text-[12.5px] text-ink outline-none placeholder:text-ink-3" value={text} placeholder={values.length ? '' : placeholder} disabled={disabled}
          onChange={(e) => { setText(e.target.value); setError(''); }}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ',') { e.preventDefault(); add(text); } }}
          onBlur={() => { if (text.trim()) add(text); }} />
        {text && <EnterHint />}
      </div>
      {error && <span className="text-[12.5px] text-bad">{error}</span>}
    </div>
  );
}

/** "Press Enter to add", with Enter drawn as a key; the locale decides where the key goes. */
function EnterHint() {
  const [before, after] = useT()('mcp.tools.enterToAdd').split('{key}');
  return (
    <span className="shrink-0 whitespace-nowrap font-sans text-xs text-ink-3">
      {before}<kbd className="rounded-[4px] border border-line bg-sunken px-1 py-px font-sans text-[11px] text-ink-2">Enter</kbd>{after}
    </span>
  );
}

/** One outcome line: the Agents' logos, then what they do. */
function Outcome({ targets, text }: { targets: string[]; text: string }) {
  return (
    <span className="flex items-center gap-2.5">
      <span className="flex shrink-0 items-center gap-1.5">{targets.map((target) => <AgentIcon key={target} target={target} size={16} />)}</span>
      {text}
    </span>
  );
}

interface Props {
  tools: MCPToolPolicy;
  onChange: (tools: MCPToolPolicy) => void;
  disabled: boolean;
  /** How the server is reached as the form describes it, saved or not, which loading its tools starts; absent until it has a command or URL. */
  probe?: MCPMutation;
  /** The server as the form describes it, once it is complete enough to preview. */
  mutation?: MCPMutation;
  /** Why the policy cannot be saved, or empty. */
  error?: string;
  /** Pi is among the chosen targets; only Pi reads the exposure. */
}

/** Which of the server's tools reach the model, for every Agent at once; each Agent that cannot follow a part is named. */
export default function ToolPolicyFields({ tools, onChange, disabled, probe, mutation, error }: Props) {
  const t = useT();
  const set = hasToolPolicy(tools);
  const [query, setQuery] = useState('');
  const [result, setResult] = useState<{ key?: string; loading?: boolean; names?: string[]; error?: string; detail?: string }>({});
  // A result belongs to the connection it was loaded with; after an edit it may not be this server's tools, so it is dropped.
  const key = JSON.stringify(probe ?? null);
  const loaded: typeof result = result.key === key ? result : {};
  const names = loaded.names;
  // The same preview the config view reads, so opening it after this costs nothing.
  const view = useQuery({ queryKey: [...queryKeys.mcp, 'render', JSON.stringify(mutation)], queryFn: () => mcpApi.render(mutation!), enabled: set && Boolean(mutation), placeholderData: keepPreviousData });
  const outcomes = toolOutcomes(set ? (view.data?.rendered ?? []).filter((r) => mutation?.server?.targets?.includes(r.target)) : []);
  // Entries a loaded row stands for are edited by ticking it; the rest stay as rules, and before loading every entry is one.
  const rules = toolRules(tools, names ?? []);
  const backed = (entries: string[] = []) => entries.filter((entry) => names?.includes(entry));
  const example = toolRuleExample(names ?? []);
  const shown = (names ?? []).filter((name) => name.toLowerCase().includes(query.trim().toLowerCase()));
  const summary = names
    ? t('mcp.tools.selected', { count: names.filter((name) => toolAllowed(tools, name)).length, total: names.length })
    : set ? toolSummary(t, tools) : t('mcp.tools.all');

  const load = async () => {
    if (!probe) return;
    // Reloading keeps the list on screen until the new one arrives.
    setResult((r) => ({ key, loading: true, names: r.key === key ? r.names : undefined }));
    try {
      const { live, errorKind, error: detail } = await mcpCheckApi.probe(probe);
      const names = live?.toolNames;
      if (names?.length) setResult({ key, names: [...new Set(names)].sort() });
      else if (live) setResult({ key, error: t('mcp.tools.loadEmpty') });
      else setResult({ key, error: t(`mcp.tools.probeError.${errorKind ?? 'unknown'}`), detail });
    } catch (e) {
      setResult({ key, error: t('mcp.tools.probeError.unknown'), detail: (e as Error).message });
    }
  };
  // Select all and none tick or untick the rows the search shows, one row at a time, in one change.
  const checkShown = (on: boolean) =>
    onChange(shown.filter((name) => toolAllowed(tools, name) !== on && !toolBlockedBy(tools, name)).reduce((next, name) => setToolChecked(next, name, on), tools));
  const action = 'text-[13px] text-ink-2 hover:text-ink';
  const sep = t('mcp.tools.partSeparator');

  return (
    <>
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-1">
          <span className="text-[13px] font-semibold">{t('mcp.tools.title')}</span>
          <InfoTip label={t('mcp.tools.info')} content={t('mcp.tools.hint')} />
          <span className="ml-1 text-[13px] text-ink-3">{summary}</span>
        </div>
        <div className="overflow-hidden rounded-xl border border-line-2 bg-surface">
          {names ? <>
            <div className="flex h-10 items-center gap-3 border-b border-line px-3.5">
              <Search size={15} className="shrink-0 text-ink-3" />
              <input className="min-w-0 flex-1 bg-transparent text-[13px] text-ink outline-none placeholder:text-ink-3" value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t('mcp.tools.search')} aria-label={t('mcp.tools.search')} />
              <button type="button" className={action} onClick={() => checkShown(true)} disabled={disabled}>{t('mcp.selectAll')}</button>
              <button type="button" className={action} onClick={() => checkShown(false)} disabled={disabled}>{t('mcp.tools.selectNone')}</button>
              <span className="h-3.5 w-px bg-line" aria-hidden="true" />
              <Tooltip content={t('mcp.tools.reload')}>
                <button type="button" className={`grid place-items-center ${action}`} aria-label={t('mcp.tools.reload')} onClick={() => void load()} disabled={disabled || !probe || loaded.loading}>
                  {loaded.loading ? <Spinner size="sm" /> : <RefreshCw size={14} />}
                </button>
              </Tooltip>
            </div>
            <div className="grid max-h-[272px] grid-cols-2 overflow-y-auto py-1.5">
              {shown.map((name) => {
                const on = toolAllowed(tools, name);
                const rule = toolBlockedBy(tools, name);
                const row = (
                  <label className={`relative flex h-8 min-w-0 items-center gap-2.5 px-3.5 ${rule ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'}`}>
                    <input type="checkbox" className="sr-only ss-chk-input" checked={on} disabled={disabled || Boolean(rule)} onChange={(e) => onChange(setToolChecked(tools, name, e.target.checked))} />
                    <span className={`ss-chk ${on ? 'on' : ''}`}>{on && <Check size={11} strokeWidth={3.2} />}</span>
                    <span className={`truncate font-mono text-[12.5px] ${on ? 'text-ink' : 'text-ink-3'}`}>{name}</span>
                  </label>
                );
                return rule ? <Tooltip key={name} block content={t('mcp.tools.blockedBy', { rule })}>{row}</Tooltip> : <div key={name}>{row}</div>;
              })}
            </div>
          </> : (
            <div className="flex items-center justify-between gap-3 px-3.5 py-2.5">
              {loaded.error ? (
                <span className="flex items-center gap-0.5 text-[13px] text-bad">
                  {loaded.error}
                  {loaded.detail && <InfoTip label={t('mcp.tools.probeDetails')} content={loaded.detail} />}
                </span>
              ) : <span className="text-[13px] text-ink-3">{t(probe ? 'mcp.tools.loadIntro' : 'mcp.tools.loadNeedsConnection')}</span>}
              <Button variant="secondary" size="sm" className="shrink-0" onClick={() => void load()} loading={loaded.loading} disabled={disabled || !probe}>
                {!loaded.loading && (loaded.error ? <RefreshCw size={14} /> : <Download size={14} />)}{t(loaded.error ? 'install.preview.retry' : 'mcp.tools.load')}
              </Button>
            </div>
          )}
          {/* Patterns and names a row cannot show; Allow only when the policy has one. */}
          <div className="border-t border-line">
            {Boolean(tools.allow?.length) && <div className="border-b border-line"><ToolList label={t('mcp.tools.allowRules')} hint={t('mcp.tools.allowHint')} placeholder={example} values={rules.allow} onChange={(allow) => onChange({ ...tools, allow: [...backed(tools.allow), ...allow] })} disabled={disabled} /></div>}
            <ToolList label={t('mcp.tools.denyRules')} hint={t('mcp.tools.denyHint')} placeholder={example} values={rules.deny} onChange={(deny) => onChange({ ...tools, deny: [...backed(tools.deny), ...deny] })} disabled={disabled} />
          </div>
        </div>
        {error && <span className="text-[12.5px] text-bad">{error}</span>}
      </div>
      {/* What each selected Agent will actually get, grouped by outcome; empty groups are left out. */}
      {(outcomes.full.length > 0 || outcomes.partial.length > 0 || outcomes.none.length > 0) && (
        <div className="flex flex-col gap-1.5 text-[13px] text-ink-3">
          {outcomes.full.length > 0 && <Outcome targets={outcomes.full} text={t('mcp.tools.effectFull', { agents: outcomes.full.map(targetLabel).join(sep) })} />}
          {outcomes.partial.map(({ target, gaps }) => {
            const agent = targetLabel(target);
            const patterns = (entries: string[] = []) => entries.filter((entry) => entry.includes('*')).join(sep);
            const text = gaps.map((gap) =>
              gap === 'deny' ? t('mcp.tools.effectDeny', { agent })
              : gap === 'deny patterns' ? t('mcp.tools.effectDenyPatterns', { agent, rules: patterns(tools.deny) })
              : gap === 'allow patterns' ? t('mcp.tools.effectAllowPatterns', { agent, rules: patterns(tools.allow) })
              : t('mcp.tools.notAppliedHere', { parts: t(toolGapKey(gap)) })).join(' ');
            return <Outcome key={target} targets={[target]} text={text} />;
          })}
          {outcomes.none.length > 0 && <Outcome targets={outcomes.none} text={t('mcp.tools.effectNone', { agents: outcomes.none.map(targetLabel).join(sep) })} />}
        </div>
      )}
    </>
  );
}
