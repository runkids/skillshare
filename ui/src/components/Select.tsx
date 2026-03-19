import { useState, useRef, useEffect, useLayoutEffect, useCallback } from 'react';
import { Check, ChevronDown } from 'lucide-react';
import { radius, shadows } from '../design';

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
}

const selectTriggerSizes = {
  sm: 'px-3 py-1.5 text-xs',
  md: 'px-4 py-2 text-sm',
};

export function Select({ label, value, onChange, options, className = '', size = 'md', disabled = false }: SelectProps) {
  const [open, setOpen] = useState(false);
  const [focusIdx, setFocusIdx] = useState(-1);
  const [dropUp, setDropUp] = useState(false);
  const [dropRight, setDropRight] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  const selected = options.find((o) => o.value === value);
  const selectedLabel = selected?.label ?? value;

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  // Collision detection: determine dropdown direction (up/down, left/right)
  // useLayoutEffect blocks paint until position is computed, preventing visual flash
  useLayoutEffect(() => {
    if (!open || !containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    const dropdownHeight = Math.min(options.length * 48, 256); // rough est, max 16rem
    const dropdownWidth = 240; // rough min-width for description options

    // Vertical: prefer below, flip up if not enough space below but enough above
    const spaceBelow = window.innerHeight - rect.bottom;
    setDropUp(spaceBelow < dropdownHeight + 8 && rect.top > dropdownHeight);

    // Horizontal: default is left-aligned (left: 0). If dropdown overflows right edge, right-align instead.
    const spaceRight = window.innerWidth - rect.left;
    setDropRight(spaceRight < dropdownWidth);
  }, [open, options.length]);

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
          setOpen(true);
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
          setOpen(true);
          setFocusIdx(Math.max(0, options.findIndex((o) => o.value === value)));
        }
        break;
      case 'Escape':
        setOpen(false);
        break;
    }
  }, [open, focusIdx, options, value, select]);

  return (
    <div ref={containerRef} className={`relative ${className}`}>
      {label && (
        <label className="block text-xs font-medium text-pencil-light mb-1">
          {label}
        </label>
      )}
      <button
        type="button"
        disabled={disabled}
        onClick={() => { if (!disabled) { setOpen(!open); setFocusIdx(options.findIndex((o) => o.value === value)); } }}
        onKeyDown={handleKeyDown}
        className={`
          ss-select
          w-full bg-surface border-2 text-pencil text-left
          flex items-center justify-between gap-2
          focus:outline-none focus:border-pencil
          transition-all duration-150
          rounded-[var(--radius-sm)]
          ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
          ${selectTriggerSizes[size]}
          ${open ? 'border-pencil' : 'border-muted hover:border-muted-dark'}
        `}
        role="combobox"
        aria-expanded={open}
        aria-haspopup="listbox"
      >
        <span className="truncate">{selectedLabel}</span>
        <ChevronDown
          size={size === 'sm' ? 13 : 15}
          strokeWidth={2}
          className={`shrink-0 text-muted-dark transition-transform duration-200 ${open ? 'rotate-180' : ''}`}
        />
      </button>
      {open && (
        <ul
          ref={listRef}
          role="listbox"
          className={`
            absolute z-50 min-w-full bg-surface border-2 border-muted overflow-auto py-1 animate-dropdown-in
            ${dropUp ? 'bottom-full mb-1' : 'top-full mt-1'}
            ${dropRight ? 'right-0' : 'left-0'}
            ${size === 'sm' ? 'text-xs' : 'text-sm'}
          `}
          style={{
            borderRadius: radius.md,
            boxShadow: shadows.lg,
            maxHeight: '16rem',
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
                className={`
                  ${size === 'sm' ? 'px-3 py-1.5' : 'px-3.5 py-2'} cursor-pointer flex items-center gap-2 transition-colors duration-100
                  ${isFocused ? 'bg-muted/60' : ''}
                  ${isSelected ? 'text-pencil' : 'text-pencil-light'}
                  hover:bg-muted/60
                `}
                onMouseEnter={() => setFocusIdx(i)}
                onMouseDown={(e) => { e.preventDefault(); select(opt.value); }}
              >
                <span className="w-4 shrink-0 flex items-center justify-center">
                  {isSelected && <Check size={size === 'sm' ? 12 : 14} strokeWidth={2.5} className="text-pencil" />}
                </span>
                <span className="flex-1 min-w-0">
                  <span className={`block truncate ${isSelected ? 'font-medium' : ''}`}>
                    {opt.label}
                  </span>
                  {opt.description && (
                    <span className="block text-xs text-pencil-light/60 mt-0.5 truncate">
                      {opt.description}
                    </span>
                  )}
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
