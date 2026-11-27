import { useLayoutEffect, useRef } from 'react';
import AgentIcon from '../AgentIcon';
import { useT } from '../../i18n';

export interface InstructionFileItem {
  /** Target or tool name, for its logo and the selection. */
  id: string;
  label: string;
  path: string;
  exists: boolean;
}

/**
 * The instruction files one target page manages, one row each: the target's
 * own file first, then (below a hairline, from divider on) the files of tools
 * that share its skills. The list scrolls inside its own height.
 */
export default function InstructionFileList({
  items,
  selected,
  onSelect,
  divider,
  caption,
}: {
  items: InstructionFileItem[];
  selected: string;
  onSelect: (id: string) => void;
  /** Index of the first item under the hairline. */
  divider?: number;
  caption?: string;
}) {
  const t = useT();
  const listRef = useRef<HTMLElement>(null);
  const selectedRef = useRef<HTMLButtonElement>(null);

  // Bring the selected row (?tool=) into view inside the list, without scrolling the page.
  useLayoutEffect(() => {
    const list = listRef.current;
    const row = selectedRef.current;
    if (!list || !row) return;
    if (
      row.offsetTop < list.scrollTop ||
      row.offsetTop + row.offsetHeight > list.scrollTop + list.clientHeight
    ) {
      list.scrollTop = row.offsetTop - list.clientHeight / 2 + row.offsetHeight / 2;
    }
  }, [selected]);

  return (
    <nav
      ref={listRef}
      aria-label={t('instructions.files.title')}
      className="relative flex h-full min-h-0 flex-col gap-0.5 overflow-y-auto"
    >
      <span className="px-2.5 pb-1.5 text-[12px] text-ink-3">{t('instructions.files.title')}</span>
      {items.map((item, i) => {
        const on = item.id === selected;
        return (
          <div key={item.id} className="contents">
            {divider === i && i > 0 && (
              <div className="mx-2.5 my-2 h-px shrink-0 bg-line-soft" role="separator" />
            )}
            <button
              ref={on ? selectedRef : undefined}
              type="button"
              aria-current={on || undefined}
              onClick={() => onSelect(item.id)}
              title={item.path}
              className={`flex shrink-0 items-center gap-2.5 rounded-[8px] border px-2.5 py-2 text-left ${on ? 'border-line bg-surface' : 'border-transparent hover:bg-sunken'}`}
            >
              <span className="flex shrink-0 items-center">
                <AgentIcon target={item.id} size={18} />
              </span>
              <span className="flex min-w-0 flex-1 flex-col gap-px">
                <span
                  className={`truncate text-[13px] ${on ? 'font-semibold text-ink' : 'text-ink-2'}`}
                >
                  {item.label}
                </span>
                <span className="truncate font-mono text-[11px] text-ink-3">
                  {item.path.split(/[\\/]/).pop()}
                </span>
              </span>
              {item.exists ? (
                <span
                  className="ss-st ok shrink-0"
                  role="img"
                  aria-label={t('instructions.files.exists')}
                />
              ) : (
                <span className="shrink-0 text-[11px] text-ink-3">
                  {t('instructions.target.notCreated')}
                </span>
              )}
            </button>
          </div>
        );
      })}
      {caption && <p className="px-2.5 pt-2.5 text-[11.5px] leading-[1.5] text-ink-3">{caption}</p>}
    </nav>
  );
}
