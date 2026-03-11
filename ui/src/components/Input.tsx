import { useState, useRef, useEffect, useCallback, useId } from 'react';
import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react';
import { Check, ChevronDown } from 'lucide-react';
import { radius, shadows } from '../design';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
}

export function Input({ label, className = '', style, id, ...props }: InputProps) {
  const autoId = useId();
  const inputId = id ?? autoId;

  return (
    <div>
      {label && (
        <label
          htmlFor={inputId}
          className="block text-base text-pencil-light mb-1"
        >
          {label}
        </label>
      )}
      <input
        id={inputId}
        className={`
          w-full px-4 py-2.5 bg-surface border border-muted text-pencil
          placeholder:text-muted-dark
          focus:outline-none focus:border-blue focus:ring-2 focus:ring-blue/20
          transition-colors
          ${className}
        `}
        style={{
          borderRadius: radius.sm,
          fontSize: '1rem',
          ...style,
        }}
        {...props}
      />
    </div>
  );
}

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
}

export function Textarea({ label, className = '', style, id, ...props }: TextareaProps) {
  const autoId = useId();
  const inputId = id ?? autoId;

  return (
    <div>
      {label && (
        <label
          htmlFor={inputId}
          className="block text-base text-pencil-light mb-1"
        >
          {label}
        </label>
      )}
      <textarea
        id={inputId}
        className={`
          w-full px-4 py-3 bg-surface border border-muted text-pencil
          placeholder:text-muted-dark
          focus:outline-none focus:border-blue focus:ring-2 focus:ring-blue/20
          transition-colors resize-y
          ${className}
        `}
        style={{
          borderRadius: radius.md,
          fontSize: '0.95rem',
          ...style,
        }}
        {...props}
      />
    </div>
  );
}

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
}

const selectTriggerSizes = {
  sm: 'px-3 py-1.5 text-xs',
  md: 'px-4 py-2 text-sm',
};

export function Select({ label, value, onChange, options, className = '', size = 'md' }: SelectProps) {
  const [open, setOpen] = useState(false);
  const [focusIdx, setFocusIdx] = useState(-1);
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
        onClick={() => { setOpen(!open); setFocusIdx(options.findIndex((o) => o.value === value)); }}
        onKeyDown={handleKeyDown}
        className={`
          w-full bg-surface border text-pencil text-left
          flex items-center justify-between gap-2
          focus:outline-none focus:ring-2 focus:ring-pencil/10 focus:border-pencil-light
          transition-all duration-150 cursor-pointer
          ${selectTriggerSizes[size]}
          ${open ? 'border-pencil-light' : 'border-muted hover:border-muted-dark'}
        `}
        style={{ borderRadius: radius.sm }}
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
            absolute z-50 mt-1 w-full bg-surface border border-muted overflow-auto py-1 animate-dropdown-in
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
                    <span className="block text-xs text-pencil-light/60 truncate mt-0.5">
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

interface CheckboxProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  className?: string;
}

export function Checkbox({ label, checked, onChange, className = '' }: CheckboxProps) {
  return (
    <label
      className={`inline-flex items-center gap-2 cursor-pointer select-none ${className}`}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="sr-only"
      />
      <span
        className={`
          w-5 h-5 flex items-center justify-center border transition-colors
          ${checked ? 'bg-blue border-blue' : 'bg-surface border-muted'}
        `}
        style={{ borderRadius: radius.sm }}
      >
        {checked && <Check size={14} strokeWidth={3} className="text-white" />}
      </span>
      <span className="text-base text-pencil">{label}</span>
    </label>
  );
}
