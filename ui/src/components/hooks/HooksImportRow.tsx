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
const ADOPT_WARNING =
  'saving the import takes over the existing registrations in place; sync leaves them as they are instead of adding duplicates';

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
export default function HooksImportRow({
  row,
  targetLabel,
  change,
  checked,
  importable,
  open,
  busy,
  name,
  nameFixed,
  nameError,
  onCheck,
  onToggle,
  onName,
  onEdit,
  onRemove,
  onUndo,
}: Props) {
  const t = useT();
  const removed = Boolean(change && 'remove' in change);
  const before = rowEvents(row);
  const events = change && 'events' in change ? change.events : before;
  const binding = row.candidate.entry.bindings[row.target];
  const lines = bindingLines(binding?.events ? { ...binding, events } : binding);
  const title = rowTitle(row);
  const matcher = lines[0]?.matcher ?? '';
  // A code hook is one file, or a folder of them, imported whole.
  const code =
    lines.length > 0
      ? []
      : Object.entries(
          binding?.package ??
            (binding?.code === undefined ? {} : { [binding.codeFile ?? '']: binding.code }),
        );
  const summary =
    lines.length > 0
      ? commandSummary(lines[0].command)
      : code.length > 0
        ? t('hooks.importWholeFile')
        : (binding?.codeFile ?? '');
  const edited = change && 'events' in change ? describeChange(before, change.events) : undefined;
  const notes = [
    ...row.candidate.problems.map((message) => ({ message, problem: true })),
    ...row.candidate.warnings
      .filter((w) => w !== ADOPT_WARNING)
      .map((message) => ({ message, problem: false })),
  ];
  const fade = 'animate-[ss-fade-in_.15s_ease-out]';
  const expanded = open && !change;
  return (
    <div className="[border-top:var(--sep)]">
      <div className="flex min-h-14 items-center gap-3 py-1.5 pl-4 pr-3">
        <Checkbox
          hideLabel
          label={t('hooks.importPick', { name: row.candidate.name })}
          checked={checked}
          disabled={!importable || Boolean(change) || busy}
          onChange={onCheck}
        />
        {/* A larger place to click; the button that opens the row is the chevron at the end. */}
        <div
          className={`flex min-w-0 flex-1 flex-col gap-0.5 ${change ? '' : 'cursor-pointer'}`}
          onClick={change ? undefined : onToggle}
        >
          <span className="flex min-w-0 items-center gap-2">
            <span
              className={`min-w-0 truncate font-mono text-[13px] font-semibold transition-colors duration-150 ${removed ? 'text-ink-3 line-through' : ''}`}
            >
              {title}
            </span>
            {matcher && lines.length === 1 && <span className="ss-tag shrink-0">{matcher}</span>}
            {lines.length > 1 && (
              <span className="shrink-0 text-xs text-ink-3">
                {t(plural('hooks.commandCount', lines.length), { count: String(lines.length) })}
              </span>
            )}
            {change && <span className={`ss-tag inf shrink-0 ${fade}`}>pending</span>}
          </span>
          {removed ? (
            <span className={`truncate text-xs text-bad ${fade}`}>
              {row.target === 'copilot' && row.candidate.source
                ? t('hooks.importWillDeleteFile', { file: row.candidate.source })
                : t('hooks.importWillRemove', { target: targetLabel })}
            </span>
          ) : edited ? (
            <span className={`truncate text-xs text-ink-2 ${fade}`}>
              {t(edited.key, edited.params)}
            </span>
          ) : nameError ? (
            <span className="truncate text-xs text-bad">{nameError}</span>
          ) : notes.some((n) => n.problem) ? (
            <span className="truncate text-xs text-bad">
              {hookMessage(t, notes.find((n) => n.problem)!.message)}
            </span>
          ) : (
            <span className={`truncate text-xs text-ink-2 ${code.length > 0 ? '' : 'font-mono'}`}>
              {summary}
            </span>
          )}
        </div>
        {change ? (
          <span className={`flex shrink-0 items-center gap-1 ${fade}`}>
            {!removed && onEdit && (
              <Button variant="ghost" size="sm" onClick={onEdit} disabled={busy}>
                {t('hooks.importEditAction')}
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={onUndo} disabled={busy}>
              {t('hooks.importUndo')}
            </Button>
          </span>
        ) : (
          <span className="flex shrink-0 items-center gap-0.5">
            {onEdit && onRemove && (
              <>
                <Tooltip content={t('hooks.importEditAction')}>
                  <IconButton
                    title={undefined}
                    icon={<Pencil size={15} />}
                    label={t('hooks.importEdit', { name: row.candidate.name })}
                    onClick={onEdit}
                    disabled={busy}
                  />
                </Tooltip>
                <Tooltip content={t('hooks.importRemoveAction')}>
                  <IconButton
                    title={undefined}
                    variant="danger-outline"
                    icon={<Trash2 size={15} />}
                    label={t('hooks.importRemove', { name: row.candidate.name })}
                    onClick={onRemove}
                    disabled={busy}
                  />
                </Tooltip>
              </>
            )}
            <IconButton
              title={undefined}
              aria-expanded={open}
              className={open ? '!bg-[var(--sel)] !text-ink' : ''}
              icon={
                <ChevronDown
                  size={15}
                  className={`transition-transform duration-200 ease-[cubic-bezier(.3,.7,.3,1)] motion-reduce:transition-none ${open ? 'rotate-180' : ''}`}
                />
              }
              label={row.candidate.name}
              onClick={onToggle}
            />
          </span>
        )}
      </div>
      <Reveal open={expanded}>
        <div className="flex flex-col gap-3.5 pb-[18px] pl-11 pr-4 pt-0.5">
          {lines.map((l, i) => (
            <div key={i} className="flex flex-col gap-1.5">
              {/* One command needs no heading: the row above already names its event and filter. */}
              {(lines.length > 1 || l.event !== title) && (
                <span className="flex items-baseline gap-2 text-xs text-ink-3">
                  {lines.length > 1 && (
                    <span className="font-semibold text-ink-2">
                      {t('hooks.commandN', { n: String(i + 1) })}
                    </span>
                  )}
                  {l.event !== title && <span className="font-mono text-ink">{l.event}</span>}
                  {l.matcher && (
                    <span>
                      {t('hooks.matcher')} <span className="font-mono text-ink">{l.matcher}</span>
                    </span>
                  )}
                </span>
              )}
              <pre className="m-0 whitespace-pre-wrap rounded-[var(--r-ctl)] bg-sunken px-3 py-2.5 font-mono text-[12.5px] leading-[1.6] [overflow-wrap:anywhere]">
                {l.command}
              </pre>
            </div>
          ))}
          {code.map(([file, text]) => (
            <div key={file} className="flex flex-col gap-1.5">
              {file && <span className="font-mono text-xs font-semibold">{file}</span>}
              <pre className="m-0 max-h-[260px] overflow-y-auto whitespace-pre-wrap rounded-[var(--r-ctl)] bg-sunken px-3 py-2.5 font-mono text-[12.5px] leading-[1.6] [overflow-wrap:anywhere]">
                {text}
              </pre>
            </div>
          ))}
          {notes.map(({ message, problem }) => (
            <span
              key={`${problem}:${message}`}
              className={`text-xs leading-normal ${problem ? 'text-bad' : 'text-ink-2'}`}
            >
              {hookMessage(t, message)}
            </span>
          ))}
          <label className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span className="shrink-0 text-xs font-semibold text-ink-2">
              {t('hooks.importNameLabel')}
            </span>
            <span className={`ss-inp w-[240px] font-mono ${nameError ? 'err' : ''}`}>
              <input
                value={name}
                onChange={(e) => onName(e.target.value)}
                aria-label={t('hooks.importNameFor', { name: row.candidate.name })}
                disabled={!checked || busy || nameFixed}
              />
            </span>
            {nameError && <span className="text-xs text-bad">{nameError}</span>}
          </label>
          {row.adopt && !onEdit && (
            <span className="text-xs text-ink-3">{t('hooks.importNotEditable')}</span>
          )}
        </div>
      </Reveal>
    </div>
  );
}
