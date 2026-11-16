import { ChevronDown, Pencil, Trash2 } from 'lucide-react';
import { plural, useT } from '../../i18n';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import IconButton from '../IconButton';
import Reveal from '../Reveal';
import Tooltip from '../Tooltip';
import { bindingLines } from './hookCatalog';
import { commandSummary, describeChange, rowEvents, rowTitle } from './hooksImport';
import type { ImportRow, NativeChange } from './hooksImport';
import { hookMessage } from './hooksView';

// Said once in the dialog's subtitle rather than on every row.
const ADOPT_WARNING = 'saving the import takes over the existing registrations in place; sync leaves them as they are instead of adding duplicates';

interface Props {
  row: ImportRow;
  /** The target's name, for saying whose file a removal changes. */
  targetLabel: string;
  change?: NativeChange;
  checked: boolean;
  importable: boolean;
  open: boolean;
  busy: boolean;
  name: string;
  nameFixed: boolean;
  nameError: string;
  onCheck: (on: boolean) => void;
  onToggle: () => void;
  onName: (name: string) => void;
  /** Left out for a hook that cannot be changed in its target file. */
  onEdit?: () => void;
  onRemove?: () => void;
  onUndo: () => void;
}

/**
 * One hook of a target: what runs and when on two lines, the full commands and the import name
 * when opened. A hook with a pending edit or removal says so and can only be put back.
 */
export default function HooksImportRow({ row, targetLabel, change, checked, importable, open, busy, name, nameFixed, nameError, onCheck, onToggle, onName, onEdit, onRemove, onUndo }: Props) {
  const t = useT();
  const removed = Boolean(change && 'remove' in change);
  const before = rowEvents(row);
  const events = change && 'events' in change ? change.events : before;
  const binding = row.candidate.entry.bindings[row.target];
  const lines = bindingLines(binding?.events ? { ...binding, events } : binding);
  const title = rowTitle(row);
  const matcher = lines[0]?.matcher ?? '';
  // A code hook is one file, or a folder of them, imported whole.
  const code = lines.length > 0 ? [] : Object.entries(binding?.package ?? (binding?.code === undefined ? {} : { [binding.codeFile ?? '']: binding.code }));
  const summary = lines.length > 0 ? commandSummary(lines[0].command) : code.length > 0 ? t('hooks.importWholeFile') : binding?.codeFile ?? '';
  const edited = change && 'events' in change ? describeChange(before, change.events) : undefined;
  const notes = [
    ...row.candidate.problems.map((message) => ({ message, problem: true })),
    ...row.candidate.warnings.filter((w) => w !== ADOPT_WARNING).map((message) => ({ message, problem: false })),
  ];
  const fade = 'animate-[ss-fade-in_.15s_ease-out]';
  return (
    <div className="border-b border-line">
      <div className="flex min-h-[52px] items-center gap-3 px-4 py-1.5">
        <Checkbox hideLabel label={t('hooks.importPick', { name: row.candidate.name })} checked={checked} disabled={!importable || Boolean(change) || busy} onChange={onCheck} />
        <button type="button" className="flex min-w-0 flex-1 items-center gap-3 text-left" aria-expanded={open} onClick={onToggle}>
          <span className="flex min-w-0 flex-1 flex-col gap-0.5">
            <span className="flex min-w-0 items-center gap-2">
              <span className={`min-w-0 truncate font-mono text-[13px] font-semibold transition-colors duration-150 ${removed ? 'text-ink-3 line-through' : ''}`}>{title}</span>
              {matcher && <span className="ss-tag shrink-0">{matcher}</span>}
              {lines.length > 1 && <span className="shrink-0 text-xs text-ink-3">{t(plural('hooks.commandCount', lines.length), { count: String(lines.length) })}</span>}
              {change && <span className={`ss-tag inf shrink-0 ${fade}`}>pending</span>}
              {removed && <span className={`min-w-0 truncate text-xs text-bad ${fade}`}>{t('hooks.importWillRemove', { target: targetLabel })}</span>}
              {edited && <span className={`min-w-0 truncate text-xs text-ink-2 ${fade}`}>{t(edited.key, edited.params)}</span>}
            </span>
            {!change && nameError ? <span className="truncate text-xs text-bad">{nameError}</span>
              : !change && notes.some((n) => n.problem) ? <span className="truncate text-xs text-bad">{hookMessage(t, notes.find((n) => n.problem)!.message)}</span>
                : <span className={`truncate text-xs text-ink-2 ${code.length > 0 ? '' : 'font-mono'}`}>{summary}</span>}
          </span>
          <ChevronDown size={15} className={`shrink-0 text-ink-3 transition-transform duration-200 ease-[cubic-bezier(.3,.7,.3,1)] motion-reduce:transition-none ${open ? 'rotate-180' : ''}`} />
        </button>
        {change ? (
          <span className={`flex shrink-0 items-center gap-1 ${fade}`}>
            {!removed && onEdit && <Button variant="ghost" size="sm" onClick={onEdit} disabled={busy}>{t('hooks.importEditAction')}</Button>}
            <Button variant="ghost" size="sm" onClick={onUndo} disabled={busy}>{t('hooks.importUndo')}</Button>
          </span>
        ) : onEdit && onRemove && (
          <span className="flex shrink-0 items-center gap-0.5">
            <Tooltip content={t('hooks.importEditAction')}>
              <IconButton title={undefined} icon={<Pencil size={15} />} label={t('hooks.importEdit', { name: row.candidate.name })} onClick={onEdit} disabled={busy} />
            </Tooltip>
            <Tooltip content={t('hooks.importRemoveAction')}>
              <IconButton title={undefined} variant="danger-outline" className="!text-bad" icon={<Trash2 size={15} />} label={t('hooks.importRemove', { name: row.candidate.name })} onClick={onRemove} disabled={busy} />
            </Tooltip>
          </span>
        )}
      </div>
      <Reveal open={open}>
        <div className="flex flex-col gap-3 border-t border-line bg-sunken py-3 pl-[46px] pr-4">
          {lines.map((l, i) => (
            <div key={i} className="flex flex-col gap-1.5">
              <span className="flex items-center gap-2 text-xs text-ink-2">
                <span className="font-mono font-semibold text-ink">{l.event}</span>
                {l.matcher && <span>{t('hooks.matcher')} <span className="font-mono text-ink">{l.matcher}</span></span>}
              </span>
              <pre className="m-0 whitespace-pre-wrap rounded-md border border-line bg-surface px-3 py-2 font-mono text-xs leading-relaxed [overflow-wrap:anywhere]">{l.command}</pre>
            </div>
          ))}
          {code.map(([file, text]) => (
            <div key={file} className="flex flex-col gap-1.5">
              {file && <span className="font-mono text-xs font-semibold">{file}</span>}
              <pre className="m-0 max-h-[260px] overflow-y-auto whitespace-pre-wrap rounded-md border border-line bg-surface px-3 py-2 font-mono text-xs leading-relaxed [overflow-wrap:anywhere]">{text}</pre>
            </div>
          ))}
          {notes.map(({ message, problem }) => <span key={`${problem}:${message}`} className={`text-xs leading-normal ${problem ? 'text-bad' : 'text-ink-2'}`}>{hookMessage(t, message)}</span>)}
          {!change && (
            <label className="ss-fld max-w-[280px]">
              <span className="text-xs font-semibold text-ink-2">{t('hooks.importNameLabel')}</span>
              <span className={`ss-inp bg-surface font-mono ${nameError ? 'err' : ''}`}>
                <input value={name} onChange={(e) => onName(e.target.value)} aria-label={t('hooks.importNameFor', { name: row.candidate.name })} disabled={!checked || busy || nameFixed} />
              </span>
              {nameError && <span className="text-xs text-bad">{nameError}</span>}
            </label>
          )}
          {!change && row.adopt && !onEdit && <span className="text-xs text-ink-3">{t('hooks.importNotEditable')}</span>}
        </div>
      </Reveal>
    </div>
  );
}
