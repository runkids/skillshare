import { Plus, SquareTerminal, X } from 'lucide-react';
import { useT } from '../../i18n';
import CodeEditor from '../CodeEditor';
import SegmentedControl from '../SegmentedControl';
import {
  eventSuggestions, hookLabel, isCodeAgent, newFileRow, newRow, parseNative, rowBlank, rowProblems, switchMode, eventsToRows,
  type BindingCheck, type BindingDraft,
} from './hooksView';

interface Props {
  agent: string;
  name: string;
  draft: BindingDraft;
  check: BindingCheck;
  onChange: (draft: BindingDraft) => void;
  disabled: boolean;
}

/** Event, matcher, command and timeout as plain fields. Everything else in the native entry is carried along untouched. */
function SimpleEvents({ agent, draft, onChange, disabled }: Pick<Props, 'agent' | 'draft' | 'onChange' | 'disabled'>) {
  const t = useT();
  const list = `hook-events-${agent}`;
  const patch = (id: string, change: Partial<BindingDraft['rows'][number]>) => onChange({ ...draft, rows: draft.rows.map((r) => (r.id === id ? { ...r, ...change } : r)) });
  return (
    <div className="ss-fld">
      <datalist id={list}>{eventSuggestions(agent).map((e) => <option key={e} value={e} />)}</datalist>
      <div className="ss-list !shadow-none">
        {draft.rows.map((r, i) => {
          const problems = rowProblems(r);
          const shown = (key: 'event' | 'command' | 'timeout') => !rowBlank(r) && problems[key];
          return (
            <div key={r.id} className="ss-r !flex-col !items-stretch gap-2 !py-2.5">
              <div className="flex items-center gap-2">
                <span className={`ss-inp min-w-0 flex-1 font-mono ${shown('event') ? 'err' : ''}`}>
                  <input list={list} value={r.event} onChange={(e) => patch(r.id, { event: e.target.value })} placeholder={eventSuggestions(agent)[0]} aria-label={`${t('hooks.event')} ${i + 1}`} disabled={disabled} />
                </span>
                {r.shape === 'group' && (
                  <span className="ss-inp min-w-0 flex-1 font-mono">
                    <input value={r.matcher} onChange={(e) => patch(r.id, { matcher: e.target.value })} placeholder={t('hooks.matcherPlaceholder')} aria-label={`${t('hooks.matcher')} ${i + 1}`} disabled={disabled} />
                  </span>
                )}
                <span className={`ss-inp w-[120px] shrink-0 font-mono ${shown('timeout') ? 'err' : ''}`}>
                  <input inputMode="numeric" value={r.timeout} onChange={(e) => patch(r.id, { timeout: e.target.value })} placeholder={r.timeoutKey} aria-label={`${t('hooks.timeout')} ${i + 1}`} disabled={disabled} />
                </span>
                <button type="button" className="ss-ib shrink-0" aria-label={`${t('hooks.removeRow')} ${i + 1}`} onClick={() => onChange({ ...draft, rows: draft.rows.filter((x) => x.id !== r.id) })} disabled={disabled}><X size={16} /></button>
              </div>
              <span className={`ss-inp font-mono ${shown('command') ? 'err' : ''}`}>
                <SquareTerminal size={15} className="shrink-0 text-ink-3" />
                <input value={r.command} onChange={(e) => patch(r.id, { command: e.target.value })} placeholder="$CLAUDE_PROJECT_DIR/.claude/hooks/check.sh" aria-label={`${t('hooks.command')} ${i + 1}`} disabled={disabled} />
              </span>
            </div>
          );
        })}
        <div className="ss-r !min-h-10">
          <button type="button" className="flex items-center gap-[7px] text-[13px] text-ink-2 hover:text-ink" onClick={() => onChange({ ...draft, rows: [...draft.rows, newRow(agent)] })} disabled={disabled}>
            <Plus size={14} />{t('hooks.addRow')}
          </button>
        </div>
      </div>
      <span className="hp">{t('hooks.fieldsHint')}</span>
    </div>
  );
}

function ScriptFiles({ name, draft, onChange, disabled }: Pick<Props, 'name' | 'draft' | 'onChange' | 'disabled'>) {
  const t = useT();
  const patch = (id: string, change: Partial<BindingDraft['files'][number]>) => onChange({ ...draft, files: draft.files.map((f) => (f.id === id ? { ...f, ...change } : f)) });
  return (
    <div className="ss-fld">
      <span className="text-[13px] font-semibold">{t('hooks.files')}</span>
      {draft.files.length > 0 && (
        <div className="ss-list !shadow-none">
          {draft.files.map((f, i) => (
            <div key={f.id} className="ss-r !flex-col !items-stretch gap-2 !py-2.5">
              <div className="flex items-center gap-2">
                <span className="ss-inp min-w-0 flex-1 font-mono">
                  <input value={f.name} onChange={(e) => patch(f.id, { name: e.target.value })} placeholder="check.sh" aria-label={`${t('hooks.fileName')} ${i + 1}`} disabled={disabled} />
                </span>
                <button type="button" className="ss-ib shrink-0" aria-label={`${t('hooks.removeFile')} ${i + 1}`} onClick={() => onChange({ ...draft, files: draft.files.filter((x) => x.id !== f.id) })} disabled={disabled}><X size={16} /></button>
              </div>
              <CodeEditor value={f.content} onChange={(content) => patch(f.id, { content })} ariaLabel={`${t('hooks.fileContent')} ${i + 1}`} disabled={disabled} minHeight="96px" />
            </div>
          ))}
        </div>
      )}
      <div>
        <button type="button" className="flex items-center gap-[7px] text-[13px] text-ink-2 hover:text-ink" onClick={() => onChange({ ...draft, files: [...draft.files, newFileRow()] })} disabled={disabled}>
          <Plus size={14} />{t('hooks.addFile')}
        </button>
      </div>
      <span className="hp">{t('hooks.filesHint', { name: name || '<name>' })}</span>
    </div>
  );
}

/** One Agent's part of a hook: command Agents edit events, code Agents edit their native extension. */
export default function HookBindingEditor({ agent, name, draft, check, onChange, disabled }: Props) {
  const t = useT();
  if (isCodeAgent(agent)) {
    return (
      <div className="flex flex-col gap-3.5">
        <div className="ss-fld">
          <span className="text-[13px] font-semibold">{t('hooks.code')}</span>
          <CodeEditor value={draft.code} onChange={(code) => onChange({ ...draft, code })} ariaLabel={`${hookLabel(agent)} ${t('hooks.code')}`} placeholder={t('hooks.codePlaceholder')} disabled={disabled} minHeight="240px" maxHeight="480px" />
          {check.codeMissing ? <span className="hp !text-bad">{t('hooks.codeRequired')}</span> : <span className="hp">{t('hooks.codeHint', { agent: hookLabel(agent), name: name || '<name>' })}</span>}
        </div>
        <div className="ss-note inf"><span className="flex-1">{t('hooks.codeVersionNote', { agent: hookLabel(agent) })}</span></div>
      </div>
    );
  }
  const native = draft.mode === 'native';
  // The simple fields can only show what they can put back; anything else stays in the native editor.
  const canSimple = !native || (() => { const p = parseNative(draft.native); return !p.error && eventsToRows(p.value) !== null; })();
  return (
    <div className="flex flex-col gap-3.5">
      <SegmentedControl<'simple' | 'native'>
        className="self-start"
        value={draft.mode}
        onChange={(mode) => onChange(switchMode(draft, mode))}
        options={[{ value: 'simple', label: t('hooks.mode.simple') }, { value: 'native', label: t('hooks.mode.native') }]}
      />
      {native ? (
        <div className="ss-fld">
          <CodeEditor value={draft.native} onChange={(text) => onChange({ ...draft, native: text })} lang="json" ariaLabel={`${hookLabel(agent)} ${t('hooks.mode.native')}`} placeholder={'{\n  "PreToolUse": [\n    { "matcher": "Bash", "hooks": [{ "type": "command", "command": "./check.sh", "timeout": 30 }] }\n  ]\n}'} disabled={disabled} minHeight="160px" maxHeight="420px" />
          {check.nativeError ? <span className="hp !text-bad">{t('hooks.nativeInvalid')}</span> : <span className="hp">{t('hooks.nativeHint', { agent: hookLabel(agent) })}</span>}
          {!canSimple && !check.nativeError && <span className="hp">{t('hooks.nativeOnly')}</span>}
        </div>
      ) : <SimpleEvents agent={agent} draft={draft} onChange={onChange} disabled={disabled} />}
      <ScriptFiles name={name} draft={draft} onChange={onChange} disabled={disabled} />
    </div>
  );
}
