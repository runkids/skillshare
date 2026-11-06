import { Fragment, useState, useRef, useEffect, useCallback, useId, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { Check, ChevronDown, Search, X } from 'lucide-react';

export interface SelectOption {
  value: string;
  label: string;
  description?: string;
  icon?: ReactNode;
  /** Heading shown above the first of consecutive options that share it. */
  group?: string;
  /** Muted text after the label, e.g. "· default". */
  note?: string;
  /** Shown but cannot be picked; the description says why. */
  disabled?: boolean;
  /** A short tag at the right end of the option, e.g. a capability. */
  badge?: string;
  /** Muted caption at the right end, e.g. a count. Not shown with `columns`, like `mono`. */
  trailing?: string;
  /** The label is a path or identifier: set it in the monospace font. */
  mono?: boolean;
  /** Draws a line above the option, to set it apart from the ones before it. */
  separated?: boolean;
  /** Trigger text when this option is chosen, if shorter than the label. */
  triggerLabel?: string;
  /** A monospace line under the description, e.g. a file path. */
  detail?: string;
  /** Full text of `detail` for a tooltip, when `detail` is shortened. */
  detailTitle?: string;
}

interface SelectProps {
  label?: string;
  /** Names the control when it has no visible label. */
  ariaLabel?: string;
  value?: string;
  onChange?: (value: string) => void;
  /** Multi-select: the chosen values. A click toggles one and the menu stays open. */
  values?: string[];
  onChangeValues?: (values: string[]) => void;
  /** Multi-select: shown in the trigger while nothing is chosen. */
  placeholder?: string;
  options: SelectOption[];
  className?: string;
  size?: 'sm' | 'md';
  disabled?: boolean;
  /** Muted text shown before the value inside the trigger, e.g. "Sort". */
  prefix?: string;
  /**
   * A filter chip instead of a field: at `clearValue` it shows only `prefix`;
   * otherwise it fills in, shows the value and gets a clear button labelled `clearLabel`.
   */
  chip?: { clearValue: string; clearLabel: string };
  /** Which trigger edge the menu lines up with; end opens it leftward, for a trigger at the right of a row. */
  align?: 'start' | 'end';
  /** Options as one line each: a monospace name column, the description beside it and the badge at the end. */
  columns?: boolean;
  /** Marks the trigger as a field with a problem, like an input with `err`. */
  invalid?: boolean;
  /** A quiet button instead of a field, for a setting that sits in a footer. */
  quiet?: boolean;
  /** Open the menu above the trigger even when there is room below. */
  up?: boolean;
  /** A text box at the top of the menu that filters options by label or value. */
  search?: { placeholder: string; empty: string; clear: string };
}

const selectTriggerSizes = {
  sm: 'h-[30px] text-xs',
  md: '',
};

// Position the dropdown in viewport (fixed) coordinates relative to the trigger.
interface DropdownPos {
  left?: number;
  /** Set instead of left when the menu lines up with the trigger's right edge. */
  right?: number;
  minWidth: number;
  top?: number;
  bottom?: number;
}

export function Select({ label, ariaLabel, value = '', onChange, values, onChangeValues, placeholder, options, className = '', size = 'md', disabled = false, prefix, chip, align = 'start', columns = false, invalid = false, quiet = false, up = false, search }: SelectProps) {
  const labelId = useId();
  const listId = useId();
  const [open, setOpen] = useState(false);
  const [focusIdx, setFocusIdx] = useState(-1);
  const [pos, setPos] = useState<DropdownPos | null>(null);
  const [query, setQuery] = useState('');
  const triggerRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const needle = query.trim().toLowerCase();
  const shown = needle ? options.filter((o) => o.label.toLowerCase().includes(needle) || o.value.toLowerCase().includes(needle)) : options;

  const isOn = (v: string) => (values ? values.includes(v) : v === value);
  const chosen = options.filter((o) => isOn(o.value));
  const selectedLabel = values ? chosen.map((o) => o.label).join(', ') : chosen[0]?.label ?? value;
  // Options with descriptions need room to read; widen the popup to ~15rem
  // (matching the dropdownWidth estimate below) so descriptions wrap nicely.
  const hasDescriptions = options.some((o) => o.description);

  // Compute fixed-position coordinates from the trigger rect. Rendered via a
  // portal to document.body so the dropdown escapes any ancestor's
  // transform/overflow/stacking-context that would otherwise clip it or trap
  // its z-index (matches SplitButton/TargetMenu/Tooltip).
  const computePos = useCallback(() => {
    if (!triggerRef.current) return;
    const rect = triggerRef.current.getBoundingClientRect();
    const dropdownHeight = Math.min(options.length * 48, 256); // rough est, max 16rem
    const minWidth = columns ? Math.max(rect.width, 520) : quiet ? Math.max(rect.width, 460) : hasDescriptions ? Math.max(rect.width, 240) : rect.width;

    // Vertical: prefer below, flip up if not enough space below but enough above.
    const spaceBelow = window.innerHeight - rect.bottom;
    const dropUp = up || spaceBelow < dropdownHeight + 8 && rect.top > dropdownHeight;

    // Horizontal: left-align to the trigger (or right-align with align="end",
    // so a wide menu grows leftward); clamp so the popup never overflows
    // the right viewport edge.
    let left = rect.left;
    if (left + minWidth > window.innerWidth - 8) {
      left = Math.max(8, window.innerWidth - 8 - minWidth);
    }

    setPos({
      ...(align === 'end' ? { right: document.documentElement.clientWidth - rect.right } : { left }),
      minWidth,
      top: dropUp ? undefined : rect.bottom + 4,
      bottom: dropUp ? window.innerHeight - rect.top + 4 : undefined,
    });
  }, [options.length, hasDescriptions, align, columns, quiet, up]);

  // Open the menu, computing position from the live trigger rect first so the
  // portal renders already positioned (no mispositioned flash, no setState in
  // an effect body).
  const openMenu = useCallback(() => {
    computePos();
    setOpen(true);
  }, [computePos]);

  // Reposition on scroll/resize. Capture phase catches scrolls in any ancestor;
  // internal list scrolling recomputes from the (unchanged) trigger rect, so it
  // is a harmless no-op and the dropdown stays open (unlike a close-on-scroll).
  useEffect(() => {
    if (!open) return;
    const onScrollResize = () => computePos();
    window.addEventListener('scroll', onScrollResize, true);
    window.addEventListener('resize', onScrollResize);
    return () => {
      window.removeEventListener('scroll', onScrollResize, true);
      window.removeEventListener('resize', onScrollResize);
    };
  }, [open, computePos]);

  // Close on outside click (trigger or portal menu are both "inside").
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      const target = e.target as Node;
      if (
        triggerRef.current && !triggerRef.current.contains(target) &&
        listRef.current && !listRef.current.contains(target) &&
        !menuRef.current?.contains(target)
      ) {
        setOpen(false);
        setQuery('');
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  // Scroll focused item into view
  useEffect(() => {
    if (!open || focusIdx < 0 || !listRef.current) return;
    const items = listRef.current.children;
    if (items[focusIdx]) {
      (items[focusIdx] as HTMLElement).scrollIntoView({ block: 'nearest' });
    }
  }, [open, focusIdx]);

  const select = useCallback((val: string) => {
    if (options.find((o) => o.value === val)?.disabled) return;
    if (values) {
      // Option order, so the saved list does not depend on the click order.
      onChangeValues?.(options.map((o) => o.value).filter((v) => (v === val ? !values.includes(v) : values.includes(v))));
      return;
    }
    onChange?.(val);
    setOpen(false);
    setQuery('');
  }, [onChange, onChangeValues, values, options]);

  // Closing clears the search and, when focus was in the search box, returns it to the trigger.
  const close = useCallback(() => {
    setOpen(false);
    if (!search) return;
    setQuery('');
    if (menuRef.current?.contains(document.activeElement)) triggerRef.current?.querySelector('button')?.focus();
  }, [search]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        if (!open) {
          openMenu();
          setFocusIdx(0);
        } else {
          setFocusIdx((i) => Math.min(i + 1, shown.length - 1));
        }
        break;
      case 'ArrowUp':
        e.preventDefault();
        if (open) {
          setFocusIdx((i) => Math.max(i - 1, 0));
        }
        break;
      case 'Enter':
      case ' ':
        // Typing a space into the search box is text, not a pick.
        if (e.key === ' ' && search && open) break;
        e.preventDefault();
        if (open && focusIdx >= 0 && shown[focusIdx]) {
          select(shown[focusIdx].value);
        } else {
          openMenu();
          setFocusIdx(Math.max(0, options.findIndex((o) => isOn(o.value))));
        }
        break;
      case 'Escape':
        // An open menu takes the key; the dialog around it stays.
        if (open) e.stopPropagation();
        close();
        break;
      case 'Tab':
        // The menu sits outside the dialog, so Tab from the search box would leave it; close and move on from the trigger.
        if (open && search) close();
        break;
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- isOn reads value and values
  }, [open, focusIdx, options, shown, search, value, values, select, openMenu, close]);

  const toggle = () => {
    if (disabled) return;
    if (open) { close(); }
    else { openMenu(); setFocusIdx(options.findIndex((o) => isOn(o.value))); }
  };
  const maxHeight = columns ? '20rem' : '16rem';
  const menuClass = `ss-menu fixed z-[9999] !w-auto animate-dropdown-in ${size === 'sm' ? 'text-xs' : 'text-[13px]'}`;
  // At least as wide as the trigger; wider for description options so they
  // wrap nicely. Bounded so long descriptions never stretch the dropdown
  // across the page.
  const menuStyle = pos ? {
    left: pos.left,
    right: pos.right,
    top: pos.top,
    bottom: pos.bottom,
    minWidth: pos.minWidth,
    maxWidth: columns ? 'min(40rem, calc(100vw - 1rem))' : quiet ? 'min(30rem, calc(100vw - 1rem))' : 'min(22rem, calc(100vw - 1rem))',
  } : undefined;
  const menuList = pos && (
        <ul
          ref={listRef}
          id={listId}
          role="listbox"
          aria-label={search ? ariaLabel ?? label : undefined}
          aria-multiselectable={values ? true : undefined}
          className={search ? 'flex min-h-0 flex-col overflow-auto' : `${menuClass} overflow-auto`}
          style={search ? { maxHeight } : { ...menuStyle, maxHeight }}
        >
          {shown.map((opt, i) => {
            const isSelected = isOn(opt.value);
            const isFocused = i === focusIdx;
            const heading = opt.group && opt.group !== shown[i - 1]?.group ? opt.group : '';
            return (
              <Fragment key={opt.value}>
              {heading && <li role="presentation" className="px-2 pt-2 pb-1 text-xs font-semibold text-ink-3">{heading}</li>}
              {opt.separated && <li role="presentation" className="mx-1.5 my-1 h-px shrink-0 bg-line-soft" />}
              <li
                role="option"
                aria-selected={isSelected}
                aria-disabled={opt.disabled || undefined}
                className={`min-h-8 shrink-0 rounded-[7px] flex ${quiet ? 'items-start gap-2.5 p-2.5' : 'items-center gap-2 px-2 py-1.5'} ${opt.disabled ? 'cursor-not-allowed text-ink-3' : isFocused ? 'cursor-pointer bg-sel text-sel-ink' : isSelected || quiet ? 'cursor-pointer text-ink' : 'cursor-pointer text-ink-2'}`}
                onMouseEnter={() => setFocusIdx(i)}
                onMouseDown={(e) => { e.preventDefault(); select(opt.value); }}
              >
                <span className={`w-4 shrink-0 flex items-center justify-center ${quiet ? 'h-5' : ''}`}>
                  {isSelected && <Check size={quiet ? 16 : size === 'sm' ? 12 : 14} />}
                </span>
                {opt.icon && <span className="flex shrink-0 items-center">{opt.icon}</span>}
                {columns ? (
                  <>
                    <span className={`w-[160px] shrink-0 truncate font-mono ${isSelected ? 'font-medium' : ''}`}>{opt.label}</span>
                    <span className={`min-w-0 flex-1 truncate ${isFocused && !opt.disabled ? '' : 'text-ink-2'}`}>{opt.description}</span>
                    {opt.badge && <span className="ss-tag shrink-0 !font-sans">{opt.badge}</span>}
                  </>
                ) : (
                <span className="flex-1 min-w-0">
                  <span className={`block truncate ${opt.mono ? 'font-mono' : ''} ${quiet ? `font-semibold leading-5 ${opt.mono ? 'text-[13px]' : 'text-sm'}` : isSelected ? 'font-medium' : ''}`}>
                    {opt.label}
                    {opt.note && <span className="font-normal opacity-70"> {opt.note}</span>}
                  </span>
                  {opt.description && (
                    <span className={`block mt-0.5 ${quiet ? 'text-[13px] text-ink-2' : `text-xs ${isFocused && !opt.disabled ? 'opacity-70' : 'text-ink-3'}`}`}>
                      {opt.description}
                    </span>
                  )}
                  {opt.detail && <span className="mt-0.5 block truncate font-mono text-xs text-ink-3" title={opt.detailTitle}>{opt.detail}</span>}
                </span>
                )}
                {opt.trailing && <span className={`shrink-0 text-xs ${isFocused && !opt.disabled ? 'opacity-70' : 'text-ink-3'}`}>{opt.trailing}</span>}
              </li>
              </Fragment>
            );
          })}
        </ul>
  );
  const chipSet = chip && value !== chip.clearValue;

  return (
    <div ref={triggerRef} className={`relative ${className}`}>
      {label && (
        <label id={labelId} className="block text-[13px] font-semibold mb-1.5">
          {label}
        </label>
      )}
      {chip ? (
        <span className={`ss-chip ${chipSet ? 'set' : ''} ${open ? 'open' : ''}`}>
          <button
            type="button"
            disabled={disabled}
            onClick={toggle}
            onKeyDown={handleKeyDown}
            className="flex items-center gap-1.5 min-w-0 outline-none"
            role="combobox"
            aria-label={ariaLabel}
            aria-expanded={open}
            aria-haspopup="listbox"
          >
            <span className={chipSet ? 'p shrink-0' : 'shrink-0'}>{prefix}</span>
            {chipSet ? <span className="truncate">{selectedLabel}</span> : <ChevronDown size={13} strokeWidth={2} className="shrink-0" />}
          </button>
          {chipSet && (
            <button type="button" className="x" aria-label={chip.clearLabel} onClick={() => onChange?.(chip.clearValue)}>
              <X size={12} strokeWidth={2.4} />
            </button>
          )}
        </span>
      ) : quiet ? (
        <button
          type="button"
          disabled={disabled}
          onClick={toggle}
          onKeyDown={handleKeyDown}
          className={`inline-flex h-[30px] cursor-pointer items-center gap-1 rounded-[8px] px-2 text-[13px] font-semibold text-ink outline-none hover:bg-sel disabled:cursor-not-allowed disabled:opacity-50 ${open ? 'bg-sel' : 'bg-transparent'} ${chosen[0]?.mono ? 'font-mono' : ''}`}
          role="combobox"
          aria-label={ariaLabel}
          aria-expanded={open}
          aria-haspopup="listbox"
        >
          <span className="truncate">{chosen[0]?.triggerLabel ?? selectedLabel}</span>
          <ChevronDown size={14} className={`shrink-0 text-ink-2 transition-transform duration-200 ${open ? 'rotate-180' : ''}`} />
        </button>
      ) : (
      <button
        type="button"
        disabled={disabled}
        onClick={toggle}
        onKeyDown={handleKeyDown}
        className={`ss-inp w-full justify-between text-left outline-none ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'} ${selectTriggerSizes[size]} ${open ? 'border-accent' : ''} ${invalid ? 'err' : ''}`}
        role="combobox"
        aria-invalid={invalid || undefined}
        aria-labelledby={label ? labelId : undefined}
        aria-label={label ? undefined : ariaLabel}
        aria-expanded={open}
        aria-haspopup="listbox"
      >
        <span className="flex items-center gap-1.5 min-w-0">
          {prefix && <span className="text-ink-3 shrink-0">{prefix}</span>}
          {chosen.some((o) => o.icon) && (
            <span className={values ? 'ss-stack shrink-0' : 'flex shrink-0 items-center'}>
              {chosen.slice(0, 6).map((o) => <span key={o.value} className={values ? 'ss-at' : 'flex items-center'}>{o.icon}</span>)}
            </span>
          )}
          <span className={`truncate ${columns || chosen[0]?.mono ? 'font-mono' : ''} ${chosen.length === 0 && placeholder ? 'text-ink-3' : ''}`}>{chosen.length === 0 && placeholder ? placeholder : selectedLabel}</span>
        </span>
        <ChevronDown
          size={size === 'sm' ? 13 : 15}
          strokeWidth={2}
          className={`shrink-0 text-ink-3 transition-transform duration-200 ${open ? 'rotate-180' : ''}`}
        />
      </button>
      )}
      {open && pos && createPortal(
        search ? (
          <div ref={menuRef} className={menuClass} style={menuStyle}>
            <label className="mb-1 flex h-8 shrink-0 items-center gap-2 rounded-[7px] bg-sunken px-2 text-ink-3">
              <Search size={14} aria-hidden className="shrink-0" />
              <input
                autoFocus
                type="search"
                value={query}
                onChange={(e) => { setQuery(e.target.value); setFocusIdx(0); }}
                onKeyDown={handleKeyDown}
                placeholder={search.placeholder}
                aria-label={search.placeholder}
                aria-controls={listId}
                className="min-w-0 flex-1 bg-transparent text-ink outline-none placeholder:text-ink-3 [&::-webkit-search-cancel-button]:appearance-none"
              />
              {query && (
                <button
                  type="button"
                  // Keep focus in the search box.
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => { setQuery(''); setFocusIdx(0); }}
                  aria-label={search.clear}
                  className="grid size-5 shrink-0 cursor-pointer place-items-center rounded-[5px] text-ink-3 hover:text-ink"
                >
                  <X size={13} aria-hidden />
                </button>
              )}
            </label>
            {menuList}
            {shown.length === 0 && <p className="px-2 py-1.5 text-ink-3">{search.empty}</p>}
          </div>
        ) : menuList,
        document.body,
      )}
    </div>
  );
}
