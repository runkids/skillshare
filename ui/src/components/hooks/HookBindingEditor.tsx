import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';
import type { CompletionSource } from '@codemirror/autocomplete';
import type { Diagnostic } from '@codemirror/lint';
import {
  AlertCircle,
  ChevronDown,
  Maximize2,
  Minimize2,
  Plus,
  Trash2,
  WandSparkles,
  X,
} from 'lucide-react';
import { useT } from '../../i18n';
import CodeEditor from '../CodeEditor';
import IconButton from '../IconButton';
import Reveal from '../Reveal';
import SegmentedControl from '../SegmentedControl';
import { Select } from '../Select';
import {
  eventSuggestions,
  hookLabel,
  isCodeAgent,
  newFileRow,
  newRow,
  parseNative,
  rowBlank,
  rowProblems,
  rowShape,
  switchMode,
  eventsToRows,
} from './hooksView';
import type { BindingCheck, BindingDraft } from './hooksView';
import {
  commandPlaceholder,
  commandVariables,
  completionContext,
  eventDescription,
  eventHasMatcher,
  fieldNames,
  formatJson,
  lineOf,
  lintNative,
  timeoutUnit,
} from './hookCatalog';
import type { HookCatalog, HookDiagnostic } from './hookCatalog';
import { dialogGhost } from '../../lib/exitGhost';
import { commandSummary, formatCommand } from './hooksImport';

interface Props {
  agent: string;
  name: string;
  draft: BindingDraft;
  check: BindingCheck;
  onChange: (draft: BindingDraft) => void;
  disabled: boolean;
  catalog?: HookCatalog;
  /** The command cards edge to edge, and no script files: for a hook edited in its target file. */
  cards?: boolean;
  /** With `cards`: where the fields / native JSON switch is rendered, outside the editor's own body. */
  tabsHost?: HTMLElement | null;
}

type T = ReturnType<typeof useT>;

const diagnosticText = (t: T, d: HookDiagnostic) => t(d.key, d.params);

/** Native JSON problems of one target, in the dashboard's language. */
function useNativeLint(agent: string, catalog: HookCatalog | undefined) {
  const t = useT();
  return useMemo(
    () =>
      (text: string): Diagnostic[] =>
        lintNative(text, agent, catalog).map((d) => ({
          from: d.from,
          to: d.to,
          severity: d.severity,
          message: diagnosticText(t, d),
        })),
    [agent, catalog, t],
  );
}

/** Completes event names at the top level and native fields and values inside them. */
function useCompletions(agent: string, catalog: HookCatalog | undefined): CompletionSource {
  const t = useT();
  return useMemo<CompletionSource>(
    () => (ctx) => {
      const text = ctx.state.doc.toString();
      const at = completionContext(text, ctx.pos);
      if (!at) return null;
      const from = at.quoted ? at.from + 1 : at.from;
      if (!ctx.explicit && !at.quoted && from === ctx.pos) return null;
      const wrap = (label: string) => (at.quoted ? label : `"${label}"`);
      const labels =
        at.kind === 'value'
          ? at.key === 'type'
            ? [{ label: 'command' }]
            : []
          : at.depth === 1
            ? (
                catalog?.[agent]?.events ??
                eventSuggestions(agent).map((name) => ({ name, description: '' }))
              ).map((e) => ({ label: e.name, detail: eventDescription(t, e.name, e.description) }))
            : fieldNames(agent).map((label) => ({ label }));
      if (labels.length === 0) return null;
      return {
        from,
        options: labels.map((o) => ({ ...o, apply: wrap(o.label), type: 'property' })),
        validFor: /^[A-Za-z_]*$/,
      };
    },
    [agent, catalog, t],
  );
}

type Row = BindingDraft['rows'][number];

/** What the event, matcher and timeout fields of a row offer for one target. */
function useRowFields(agent: string, catalog: HookCatalog | undefined) {
  const t = useT();
  const documented =
    catalog?.[agent]?.events ??
    eventSuggestions(agent).map((name) => ({ name, description: '', matcher: true }));
  return {
    unit: t(
      timeoutUnit(catalog, agent) === 'milliseconds'
        ? 'hooks.unit.milliseconds'
        : 'hooks.unit.seconds',
    ),
    options: (r: Row) => [
      ...(r.event && !documented.some((e) => e.name === r.event)
        ? [{ value: r.event, label: r.event, description: t('hooks.eventUndocumented') }]
        : []),
      ...documented.map((e) => ({
        value: e.name,
        label: e.name,
        description: eventDescription(t, e.name, e.description),
        badge: e.matcher ? t('hooks.matcherBadge') : undefined,
        group: t('hooks.eventsOf', { agent: hookLabel(agent) }),
      })),
    ],
    hasMatcher: (r: Row) =>
      r.shape === 'group' &&
      (r.event ? eventHasMatcher(catalog, agent, r.event) || Boolean(r.matcher) : true),
    pickEvent: (r: Row, event: string): Partial<Row> => ({
      event,
      shape: agent === 'antigravity' ? rowShape(agent, event) : r.shape,
      ...(!eventHasMatcher(catalog, agent, event) && { matcher: '' }),
    }),
  };
}

/**
 * Event, matcher, command and timeout, one collapsible card per command, with the command in a multi-line editor
 * that can be laid out one statement per line. Everything else in the native entry is carried along untouched.
 * One card is open at a time.
 */
function CommandCards({
  agent,
  draft,
  onChange,
  disabled,
  catalog,
}: Pick<Props, 'agent' | 'draft' | 'onChange' | 'disabled' | 'catalog'>) {
  const t = useT();
  const [open, setOpen] = useState(draft.rows[0]?.id ?? '');
  const patch = (id: string, change: Partial<Row>) =>
    onChange({ ...draft, rows: draft.rows.map((r) => (r.id === id ? { ...r, ...change } : r)) });
  const fields = useRowFields(agent, catalog);
  const variables = commandVariables(agent);
  const add = () => {
    const row = newRow(agent, draft.rows[draft.rows.length - 1]?.event);
    onChange({ ...draft, rows: [...draft.rows, row] });
    setOpen(row.id);
  };
  return (
    <div className="flex flex-col">
      {draft.rows.map((r, i) => {
        const problems = rowProblems(r);
        const shown = (key: 'event' | 'command' | 'timeout') => !rowBlank(r) && problems[key];
        const on = open === r.id;
        const title = t('hooks.commandN', { n: String(i + 1) });
        return (
          <div key={r.id} className="[border-top:var(--sep)]">
            <div className="flex min-h-12 items-center gap-2 pl-4 pr-3">
              <button
                type="button"
                className="flex min-w-0 flex-1 items-center gap-2.5 self-stretch text-left"
                aria-expanded={on}
                onClick={() => setOpen(on ? '' : r.id)}
              >
                <ChevronDown
                  size={15}
                  className={`shrink-0 text-ink-3 transition-transform duration-200 ease-[cubic-bezier(.3,.7,.3,1)] motion-reduce:transition-none ${on ? '' : '-rotate-90'}`}
                />
                <span className="shrink-0 text-[13px] font-semibold">{title}</span>
                {!on && !rowBlank(r) && Object.values(problems).some(Boolean) && (
                  <span className="ss-tag bad shrink-0" aria-label={t('hooks.tabInvalid')}>
                    !
                  </span>
                )}
                {!on && r.matcher && (
                  <span className="shrink-0 font-mono text-xs text-ink-2">{r.matcher}</span>
                )}
                {!on && (
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3">
                    {commandSummary(r.command)}
                  </span>
                )}
              </button>
              <IconButton
                variant="danger-outline"
                icon={<Trash2 size={15} />}
                label={`${t('hooks.removeRow')} ${i + 1}`}
                onClick={() =>
                  onChange({ ...draft, rows: draft.rows.filter((x) => x.id !== r.id) })
                }
                disabled={disabled}
              />
            </div>
            <Reveal open={on}>
              <div className="flex flex-col gap-4 pb-[18px] pl-[41px] pr-4">
                <div className="flex items-end gap-2.5">
                  <div className="ss-fld min-w-0 flex-1">
                    <span className="text-xs font-semibold text-ink-2">{t('hooks.event')}</span>
                    <Select
                      columns
                      invalid={shown('event')}
                      ariaLabel={`${t('hooks.event')} ${i + 1}`}
                      value={r.event}
                      placeholder={t('hooks.pickEvent')}
                      disabled={disabled}
                      options={fields.options(r)}
                      onChange={(event) => patch(r.id, fields.pickEvent(r, event))}
                    />
                  </div>
                  {fields.hasMatcher(r) && (
                    <div className="ss-fld min-w-0 flex-1">
                      <span className="text-xs font-semibold text-ink-2">{t('hooks.matcher')}</span>
                      <span className="ss-inp font-mono">
                        <input
                          value={r.matcher}
                          onChange={(e) => patch(r.id, { matcher: e.target.value })}
                          placeholder={t('hooks.matcherPlaceholder')}
                          aria-label={`${t('hooks.matcher')} ${i + 1}`}
                          disabled={disabled}
                        />
                      </span>
                    </div>
                  )}
                  <div className="ss-fld w-[130px] shrink-0">
                    <span className="text-xs font-semibold text-ink-2">{t('hooks.timeout')}</span>
                    <span className={`ss-inp font-mono ${shown('timeout') ? 'err' : ''}`}>
                      <input
                        inputMode="numeric"
                        value={r.timeout}
                        onChange={(e) => patch(r.id, { timeout: e.target.value })}
                        aria-label={`${t('hooks.timeout')} ${i + 1}`}
                        disabled={disabled}
                      />
                      <span className="shrink-0 font-sans text-xs text-ink-3">{fields.unit}</span>
                    </span>
                  </div>
                </div>
                <Expandable
                  title={`${title} · ${t('hooks.command')}`}
                  tools={(expanded) => (
                    <>
                      {!expanded && (
                        <span className="shrink-0 text-xs font-semibold text-ink-2">
                          {t('hooks.command')}
                        </span>
                      )}
                      <span className="flex min-w-0 flex-1 flex-wrap items-center justify-end gap-1.5 text-xs text-ink-3">
                        {variables.length > 0 && t('hooks.insertVariable')}
                        {variables.map((v) => (
                          <button
                            key={v}
                            type="button"
                            className="ss-tag hover:text-ink"
                            onClick={() => patch(r.id, { command: r.command + v })}
                            disabled={disabled}
                          >
                            {v}
                          </button>
                        ))}
                      </span>
                      <button
                        type="button"
                        className="flex shrink-0 items-center gap-1.5 text-xs font-semibold text-ink-2 hover:text-ink disabled:opacity-50"
                        disabled={disabled}
                        onClick={() => patch(r.id, { command: formatCommand(r.command) })}
                      >
                        <WandSparkles size={13} />
                        {t('hooks.format')}
                      </button>
                    </>
                  )}
                >
                  {(expanded) => (
                    <>
                      <CodeEditor
                        value={r.command}
                        onChange={(command) => patch(r.id, { command })}
                        ariaLabel={`${t('hooks.command')} ${i + 1}`}
                        placeholder={commandPlaceholder(agent)}
                        disabled={disabled}
                        wrap
                        minHeight="88px"
                        maxHeight="260px"
                        fill={expanded}
                        className={`${expanded ? 'min-h-0 flex-1' : ''} ${shown('command') ? '!border-bad' : '!border-transparent'}`}
                      />
                      <span className="hp">{t('hooks.commandHint')}</span>
                    </>
                  )}
                </Expandable>
              </div>
            </Reveal>
          </div>
        );
      })}
      <div className="flex min-h-12 items-center px-4 [border-top:var(--sep)]">
        <button
          type="button"
          className="flex w-fit items-center gap-2 text-[13px] text-ink-2 hover:text-ink"
          onClick={add}
          disabled={disabled}
        >
          <Plus size={14} />
          {t('hooks.addRow')}
        </button>
      </div>
    </div>
  );
}

function ScriptFiles({
  name,
  draft,
  onChange,
  disabled,
}: Pick<Props, 'name' | 'draft' | 'onChange' | 'disabled'>) {
  const t = useT();
  const patch = (id: string, change: Partial<BindingDraft['files'][number]>) =>
    onChange({ ...draft, files: draft.files.map((f) => (f.id === id ? { ...f, ...change } : f)) });
  return (
    <div className="ss-fld">
      <span className="text-[13px] font-semibold">{t('hooks.files')}</span>
      {draft.files.length > 0 && (
        <div className="ss-list !shadow-none">
          {draft.files.map((f, i) => (
            <div key={f.id} className="ss-r !flex-col !items-stretch gap-2 !py-2.5">
              <div className="flex items-center gap-2">
                <span className="ss-inp min-w-0 flex-1 font-mono">
                  <input
                    value={f.name}
                    onChange={(e) => patch(f.id, { name: e.target.value })}
                    placeholder="check.sh"
                    aria-label={`${t('hooks.fileName')} ${i + 1}`}
                    disabled={disabled}
                  />
                </span>
                <button
                  type="button"
                  className="ss-ib shrink-0"
                  aria-label={`${t('hooks.removeFile')} ${i + 1}`}
                  onClick={() =>
                    onChange({ ...draft, files: draft.files.filter((x) => x.id !== f.id) })
                  }
                  disabled={disabled}
                >
                  <X size={16} />
                </button>
              </div>
              <CodeEditor
                value={f.content}
                onChange={(content) => patch(f.id, { content })}
                ariaLabel={`${t('hooks.fileContent')} ${i + 1}`}
                disabled={disabled}
                minHeight="96px"
              />
            </div>
          ))}
        </div>
      )}
      <div>
        <button
          type="button"
          className="flex items-center gap-[7px] text-[13px] text-ink-2 hover:text-ink"
          onClick={() => onChange({ ...draft, files: [...draft.files, newFileRow()] })}
          disabled={disabled}
        >
          <Plus size={14} />
          {t('hooks.addFile')}
        </button>
      </div>
      <span className="hp">{t('hooks.filesHint', { name: name || '<name>' })}</span>
    </div>
  );
}

/**
 * An editor that grows to nearly the whole window, over the dialog, and back. It stays the same element, so the
 * editor keeps its text, cursor, lint and completion, and ⌘S still saves the dialog. Esc collapses it once the
 * editor has used the key (closing its completion list) or had no use for it.
 */
function Expandable({
  title,
  tools,
  children,
}: {
  title: string;
  tools: (expanded: boolean) => ReactNode;
  children: (expanded: boolean) => ReactNode;
}) {
  const t = useT();
  const [expanded, setExpanded] = useState(false);
  return (
    <>
      {expanded && (
        <div
          ref={dialogGhost}
          className="animate-[ss-fade-in_.2s_ease-out] fixed inset-0 z-[70] bg-[rgba(20,19,18,.36)]"
          aria-hidden="true"
          onClick={() => setExpanded(false)}
        />
      )}
      <div
        role={expanded ? 'group' : undefined}
        aria-label={expanded ? title : undefined}
        className={
          expanded
            ? 'fixed inset-6 z-[71] flex flex-col gap-2.5 rounded-[14px] border border-line bg-surface p-4 shadow-[var(--sh-dialog)]'
            : 'ss-fld'
        }
        onKeyDown={(e) => {
          if (!expanded || e.key !== 'Escape' || e.defaultPrevented) return;
          e.preventDefault();
          setExpanded(false);
        }}
      >
        <div className="flex items-center gap-3">
          {expanded && <span className="shrink-0 text-[13px] font-semibold">{title}</span>}
          {tools(expanded)}
          <button
            type="button"
            className="flex shrink-0 items-center gap-1.5 text-xs font-semibold text-ink-2 hover:text-ink"
            aria-expanded={expanded}
            onClick={() => setExpanded(!expanded)}
          >
            {expanded ? <Minimize2 size={13} /> : <Maximize2 size={13} />}
            {t(expanded ? 'hooks.collapse' : 'hooks.expand')}
          </button>
        </div>
        {children(expanded)}
      </div>
    </>
  );
}

/** One target's part of a hook: command targets edit events, code targets edit their native extension. */
export default function HookBindingEditor({
  agent,
  name,
  draft,
  check,
  onChange,
  disabled,
  catalog,
  cards = false,
  tabsHost,
}: Props) {
  const t = useT();
  const lint = useNativeLint(agent, catalog);
  const completions = useCompletions(agent, catalog);
  // Why the fields tab refused to open; cleared by any edit of the JSON.
  const [stuck, setStuck] = useState('');
  if (agent === 'git') {
    return (
      <Expandable
        title={`Git ${t('hooks.gitYAML')}`}
        tools={() => <span className="flex-1 text-[13px] font-semibold">{t('hooks.gitYAML')}</span>}
      >
        {(expanded) => (
          <>
            <CodeEditor
              value={draft.native}
              onChange={(native) => onChange({ ...draft, native })}
              lang="yaml"
              ariaLabel={`Git ${t('hooks.gitYAML')}`}
              placeholder={
                'commands:\n  tool.check:\n    events: [pre-commit]\n    command: echo check\n'
              }
              disabled={disabled}
              minHeight="240px"
              maxHeight="480px"
              fill={expanded}
              className={expanded ? 'min-h-0 flex-1' : ''}
            />
            <span className={`hp ${check.nativeError ? '!text-bad' : ''}`}>
              {t(check.nativeError ? 'hooks.gitYAMLError' : 'hooks.gitYAMLHint')}
            </span>
          </>
        )}
      </Expandable>
    );
  }
  if (isCodeAgent(agent)) {
    return (
      <div className="flex flex-col gap-3.5">
        <Expandable
          title={`${hookLabel(agent)} ${t('hooks.code')}`}
          tools={(expanded) => (
            <span className="flex-1 text-[13px] font-semibold">{!expanded && t('hooks.code')}</span>
          )}
        >
          {(expanded) => (
            <>
              <CodeEditor
                value={draft.code}
                onChange={(code) => onChange({ ...draft, code })}
                lang={draft.package ? 'json' : 'typescript'}
                ariaLabel={`${hookLabel(agent)} ${t('hooks.code')}`}
                placeholder={t('hooks.codePlaceholder')}
                disabled={disabled}
                minHeight="240px"
                maxHeight="480px"
                fill={expanded}
                className={expanded ? 'min-h-0 flex-1' : ''}
              />
              {check.codeMissing ? (
                <span className="hp !text-bad">{t('hooks.codeRequired')}</span>
              ) : (
                <span className="hp">
                  {draft.package
                    ? 'package.json'
                    : t('hooks.codeHint', { agent: hookLabel(agent), name: name || '<name>' })}
                </span>
              )}
            </>
          )}
        </Expandable>
        <p className="text-xs leading-normal text-ink-2">
          {t('hooks.codeVersionNote', { agent: hookLabel(agent) })}
        </p>
        {/* Oh My Pi loads project extensions without asking for trust, so a synced file can run at its next start. */}
        {agent === 'omp' && (
          <div className="ss-note warn" role="note">
            <AlertCircle size={16} />
            <span className="flex-1">{t('hooks.ompNoTrust')}</span>
          </div>
        )}
      </div>
    );
  }
  const native = draft.mode === 'native';
  const problems = native ? lintNative(draft.native, agent, catalog) : [];
  const errors = problems.filter((d) => d.severity === 'error');
  const parsed = native ? parseNative(draft.native) : {};
  // The simple fields can only show what they can put back; anything else stays in the native editor.
  const canSimple = !native || (!parsed.error && eventsToRows(parsed.value) !== null);
  const pick = (mode: BindingDraft['mode']) => {
    if (mode === 'simple' && native && !canSimple) {
      const first = errors[0];
      setStuck(
        first
          ? t('hooks.fieldsBlockedError', {
              line: String(lineOf(draft.native, first.from)),
              message: diagnosticText(t, first),
            })
          : t('hooks.nativeOnly'),
      );
      return;
    }
    setStuck('');
    onChange(switchMode(draft, mode));
  };
  return (
    <div
      className={
        cards
          ? native
            ? 'flex flex-col gap-3.5 px-4 pb-4'
            : 'flex flex-col'
          : 'flex flex-col gap-3.5'
      }
    >
      {cards ? (
        tabsHost &&
        createPortal(
          <SegmentedControl
            value={draft.mode}
            onChange={pick}
            options={[
              { value: 'simple', label: t('hooks.mode.simple') },
              {
                value: 'native',
                label: t('hooks.mode.native'),
                ...(errors.length > 0 && { count: errors.length }),
              },
            ]}
          />,
          tabsHost,
        )
      ) : (
        <div className="ss-tabs" role="tablist">
          <button
            type="button"
            role="tab"
            aria-selected={!native}
            className={native ? '' : 'on'}
            onClick={() => pick('simple')}
          >
            {t('hooks.mode.simple')}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={native}
            className={native ? 'on' : ''}
            onClick={() => pick('native')}
          >
            {t('hooks.mode.native')}
            {errors.length > 0 && (
              <span
                className="ss-cnt !bg-bad !text-surface"
                aria-label={t('hooks.errorCount', { count: String(errors.length) })}
              >
                {errors.length}
              </span>
            )}
          </button>
        </div>
      )}
      {native && stuck && (
        <div className="ss-note bad" role="alert">
          <span className="flex-1">{stuck}</span>
        </div>
      )}
      {native ? (
        <Expandable
          title={`${hookLabel(agent)} ${t('hooks.mode.native')}`}
          tools={(expanded) => (
            <>
              <span className="hp flex-1">
                {t('hooks.nativeHint', { agent: hookLabel(agent) })}
              </span>
              {expanded && errors.length > 0 && (
                <span className="ss-cnt shrink-0 !bg-bad !text-surface" aria-hidden="true">
                  {errors.length}
                </span>
              )}
              <button
                type="button"
                className="flex shrink-0 items-center gap-1.5 text-xs font-semibold text-ink-2 hover:text-ink disabled:opacity-50"
                disabled={disabled || Boolean(parsed.error)}
                onClick={() => onChange({ ...draft, native: formatJson(draft.native) })}
              >
                <WandSparkles size={13} />
                {t('hooks.format')}
              </button>
            </>
          )}
        >
          {(expanded) => (
            <>
              <CodeEditor
                value={draft.native}
                onChange={(text) => {
                  setStuck('');
                  onChange({ ...draft, native: text });
                }}
                lang="json"
                lint={lint}
                completions={completions}
                ariaLabel={`${hookLabel(agent)} ${t('hooks.mode.native')}`}
                placeholder={
                  '{\n  "PreToolUse": [\n    { "matcher": "Bash", "hooks": [{ "type": "command", "command": "./check.sh", "timeout": 30 }] }\n  ]\n}'
                }
                disabled={disabled}
                minHeight="180px"
                maxHeight="420px"
                fill={expanded}
                className={expanded ? 'min-h-0 flex-1' : ''}
              />
              {!stuck &&
                problems.slice(0, 3).map((d) => (
                  <span
                    key={`${d.from}:${d.key}`}
                    className={`hp ${d.severity === 'error' ? '!text-bad' : ''}`}
                  >
                    {t('hooks.lint.atLine', { line: String(lineOf(draft.native, d.from)) })}{' '}
                    {diagnosticText(t, d)}
                  </span>
                ))}
              {!stuck && !parsed.error && !canSimple && (
                <span className="hp">{t('hooks.nativeOnly')}</span>
              )}
            </>
          )}
        </Expandable>
      ) : cards ? (
        <CommandCards
          agent={agent}
          draft={draft}
          onChange={onChange}
          disabled={disabled}
          catalog={catalog}
        />
      ) : (
        // The frame's own border stands in for the first card's separator.
        <div className="overflow-hidden rounded-[10px] border border-line">
          <div className="-mt-px">
            <CommandCards
              agent={agent}
              draft={draft}
              onChange={onChange}
              disabled={disabled}
              catalog={catalog}
            />
          </div>
        </div>
      )}
      {!cards && <ScriptFiles name={name} draft={draft} onChange={onChange} disabled={disabled} />}
    </div>
  );
}
