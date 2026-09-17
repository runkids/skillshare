import { useState, useRef, useEffect, useCallback } from 'react';
import { createPortal } from 'react-dom';
import { Check, ChevronDown } from 'lucide-react';

export interface SelectOption {
  value: string;
  label: string;
  description?: string;
}

interface SelectProps {
  label?: string;
  value: string;
  onChange: (value: string) => void;
  options: SelectOption[];
  className?: string;
  size?: 'sm' | 'md';
  disabled?: boolean;
  /** Muted text shown before the value inside the trigger, e.g. "Sort". */
  prefix?: string;
}

const selectTriggerSizes = {
  sm: 'h-[30px] text-xs',
  md: '',
};

// Position the dropdown in viewport (fixed) coordinates relative to the trigger.
interface DropdownPos {
  left: number;
  minWidth: number;
  top?: number;
  bottom?: number;
}

export function Select({ label, value, onChange, options, className = '', size = 'md', disabled = false, prefix }: SelectProps) {
  const [open, setOpen] = useState(false);
  const [focusIdx, setFocusIdx] = useState(-1);
  const [pos, setPos] = useState<DropdownPos | null>(null);
  const triggerRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  const selected = options.find((o) => o.value === value);
  const selectedLabel = selected?.label ?? value;
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
    const minWidth = hasDescriptions ? Math.max(rect.width, 240) : rect.width;

    // Vertical: prefer below, flip up if not enough space below but enough above.
    const spaceBelow = window.innerHeight - rect.bottom;
    const dropUp = spaceBelow < dropdownHeight + 8 && rect.top > dropdownHeight;

    // Horizontal: left-align to the trigger; clamp so the popup never overflows
    // the right viewport edge.
    let left = rect.left;
    if (left + minWidth > window.innerWidth - 8) {
      left = Math.max(8, window.innerWidth - 8 - minWidth);
    }

    setPos({
      left,
      minWidth,
      top: dropUp ? undefined : rect.bottom + 4,
      bottom: dropUp ? window.innerHeight - rect.top + 4 : undefined,
    });
  }, [options.length, hasDescriptions]);

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
    onChange(val);
    setOpen(false);
  }, [onChange]);

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
          setFocusIdx(Math.max(0, options.findIndex((o) => o.value === value)));
        }
        break;
      case 'Escape':
        setOpen(false);
        break;
    }
  }, [open, focusIdx, options, value, select, openMenu]);

  return (
    <div ref={triggerRef} className={`relative ${className}`}>
      {label && (
        <label className="block text-[13px] font-semibold mb-1.5">
          {label}
        </label>
      )}
      <button
        type="button"
        disabled={disabled}
        onClick={() => {
          if (disabled) return;
          if (open) { setOpen(false); }
          else { openMenu(); setFocusIdx(options.findIndex((o) => o.value === value)); }
        }}
        onKeyDown={handleKeyDown}
        className={`ss-inp w-full justify-between text-left outline-none ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'} ${selectTriggerSizes[size]} ${open ? 'border-accent' : ''}`}
        role="combobox"
        aria-expanded={open}
        aria-haspopup="listbox"
      >
        <span className="flex items-center gap-1.5 min-w-0">
          {prefix && <span className="text-ink-3 shrink-0">{prefix}</span>}
          <span className="truncate">{selectedLabel}</span>
        </span>
        <ChevronDown
          size={size === 'sm' ? 13 : 15}
          strokeWidth={2}
          className={`shrink-0 text-ink-3 transition-transform duration-200 ${open ? 'rotate-180' : ''}`}
        />
      </button>
      {open && pos && createPortal(
        <ul
          ref={listRef}
          role="listbox"
          className={`ss-menu fixed z-[9999] !w-auto overflow-auto animate-dropdown-in ${size === 'sm' ? 'text-xs' : 'text-[13px]'}`}
          style={{
            left: pos.left,
            top: pos.top,
            bottom: pos.bottom,
            maxHeight: '16rem',
            // At least as wide as the trigger; wider for description options so
            // they wrap nicely. Bounded so long descriptions never stretch the
            // dropdown across the page.
            minWidth: pos.minWidth,
            maxWidth: 'min(22rem, calc(100vw - 1rem))',
          }}
        >
          {options.map((opt, i) => {
            const isSelected = opt.value === value;
            const isFocused = i === focusIdx;
            return (
              <li
                key={opt.value}
                role="option"
                aria-selected={isSelected}
                className={`min-h-8 px-2 py-1.5 rounded-[7px] cursor-pointer flex items-center gap-2 ${isFocused ? 'bg-sel text-sel-ink' : isSelected ? 'text-ink' : 'text-ink-2'}`}
                onMouseEnter={() => setFocusIdx(i)}
                onMouseDown={(e) => { e.preventDefault(); select(opt.value); }}
              >
                <span className="w-4 shrink-0 flex items-center justify-center">
                  {isSelected && <Check size={size === 'sm' ? 12 : 14} />}
                </span>
                <span className="flex-1 min-w-0">
                  <span className={`block truncate ${isSelected ? 'font-medium' : ''}`}>
                    {opt.label}
                  </span>
                  {opt.description && (
                    <span className="block text-xs text-ink-3 mt-0.5">
                      {opt.description}
                    </span>
                  )}
                </span>
              </li>
            );
          })}
        </ul>,
        document.body,
      )}
    </div>
  );
}
