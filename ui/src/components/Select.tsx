import { Fragment, useState, useRef, useEffect, useCallback, useId, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { Check, ChevronDown, X } from 'lucide-react';

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

export function Select({ label, ariaLabel, value = '', onChange, values, onChangeValues, placeholder, options, className = '', size = 'md', disabled = false, prefix, chip, align = 'start', columns = false, invalid = false }: SelectProps) {
  const labelId = useId();
  const [open, setOpen] = useState(false);
  const [focusIdx, setFocusIdx] = useState(-1);
  const [pos, setPos] = useState<DropdownPos | null>(null);
  const triggerRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

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
    const minWidth = columns ? Math.max(rect.width, 520) : hasDescriptions ? Math.max(rect.width, 240) : rect.width;

    // Vertical: prefer below, flip up if not enough space below but enough above.
    const spaceBelow = window.innerHeight - rect.bottom;
    const dropUp = spaceBelow < dropdownHeight + 8 && rect.top > dropdownHeight;

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
  }, [options.length, hasDescriptions, align, columns]);

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
        listRef.current && !listRef.current.contains(target)
      ) {
        setOpen(false);
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
  }, [onChange, onChangeValues, values, options]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        if (!open) {
          openMenu();
          setFocusIdx(0);
        } else {
          setFocusIdx((i) => Math.min(i + 1, options.length - 1));
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
        e.preventDefault();
        if (open && focusIdx >= 0) {
          select(options[focusIdx].value);
        } else {
          openMenu();
          setFocusIdx(Math.max(0, options.findIndex((o) => isOn(o.value))));
        }
        break;
      case 'Escape':
        // An open menu takes the key; the dialog around it stays.
        if (open) e.stopPropagation();
        setOpen(false);
        break;
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- isOn reads value and values
  }, [open, focusIdx, options, value, values, select, openMenu]);

  const toggle = () => {
    if (disabled) return;
    if (open) { setOpen(false); }
    else { openMenu(); setFocusIdx(options.findIndex((o) => isOn(o.value))); }
  };
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
        <ul
          ref={listRef}
          role="listbox"
          aria-multiselectable={values ? true : undefined}
          className={`ss-menu fixed z-[9999] !w-auto overflow-auto animate-dropdown-in ${size === 'sm' ? 'text-xs' : 'text-[13px]'}`}
          style={{
            left: pos.left,
            right: pos.right,
            top: pos.top,
            bottom: pos.bottom,
            maxHeight: columns ? '20rem' : '16rem',
            // At least as wide as the trigger; wider for description options so
            // they wrap nicely. Bounded so long descriptions never stretch the
            // dropdown across the page.
            minWidth: pos.minWidth,
            maxWidth: columns ? 'min(40rem, calc(100vw - 1rem))' : 'min(22rem, calc(100vw - 1rem))',
          }}
        >
          {options.map((opt, i) => {
            const isSelected = isOn(opt.value);
            const isFocused = i === focusIdx;
            const heading = opt.group && opt.group !== options[i - 1]?.group ? opt.group : '';
            return (
              <Fragment key={opt.value}>
              {heading && <li role="presentation" className="px-2 pt-2 pb-1 text-xs font-semibold text-ink-3">{heading}</li>}
              {opt.separated && <li role="presentation" className="mx-1.5 my-1 h-px shrink-0 bg-line-soft" />}
              <li
                role="option"
                aria-selected={isSelected}
                aria-disabled={opt.disabled || undefined}
                className={`min-h-8 shrink-0 px-2 py-1.5 rounded-[7px] flex items-center gap-2 ${opt.disabled ? 'cursor-not-allowed text-ink-3' : isFocused ? 'cursor-pointer bg-sel text-sel-ink' : isSelected ? 'cursor-pointer text-ink' : 'cursor-pointer text-ink-2'}`}
                onMouseEnter={() => setFocusIdx(i)}
                onMouseDown={(e) => { e.preventDefault(); select(opt.value); }}
              >
                <span className="w-4 shrink-0 flex items-center justify-center">
                  {isSelected && <Check size={size === 'sm' ? 12 : 14} />}
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
                  <span className={`block truncate ${opt.mono ? 'font-mono' : ''} ${isSelected ? 'font-medium' : ''}`}>
                    {opt.label}
                    {opt.note && <span className="font-normal opacity-70"> {opt.note}</span>}
                  </span>
                  {opt.description && (
                    <span className={`block text-xs mt-0.5 ${isFocused && !opt.disabled ? 'opacity-70' : 'text-ink-3'}`}>
                      {opt.description}
                    </span>
                  )}
                </span>
                )}
                {opt.trailing && <span className={`shrink-0 text-xs ${isFocused && !opt.disabled ? 'opacity-70' : 'text-ink-3'}`}>{opt.trailing}</span>}
              </li>
              </Fragment>
            );
          })}
        </ul>,
        document.body,
      )}
    </div>
  );
}
