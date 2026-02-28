import { useState, useRef, useEffect, useCallback } from 'react';
import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react';
import { Check, ChevronDown } from 'lucide-react';
import { wobbly, shadows } from '../design';

interface HandInputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
}

export function HandInput({ label, className = '', style, ...props }: HandInputProps) {
  return (
    <div>
      {label && (
        <label
          className="block text-base text-pencil-light mb-1"
          style={{ fontFamily: 'var(--font-hand)' }}
        >
          {label}
        </label>
      )}
      <input
        className={`
          w-full px-4 py-2.5 bg-surface border-2 border-pencil text-pencil
          placeholder:text-muted-dark
          focus:outline-none focus:border-blue focus:ring-2 focus:ring-blue/20
          transition-colors
          ${className}
        `}
        style={{
          borderRadius: wobbly.sm,
          fontFamily: 'var(--font-hand)',
          fontSize: '1rem',
          ...style,
        }}
        {...props}
      />
    </div>
  );
}

interface HandTextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
}

export function HandTextarea({ label, className = '', style, ...props }: HandTextareaProps) {
  return (
    <div>
      {label && (
        <label
          className="block text-base text-pencil-light mb-1"
          style={{ fontFamily: 'var(--font-hand)' }}
        >
          {label}
        </label>
      )}
      <textarea
        className={`
          w-full px-4 py-3 bg-surface border-2 border-pencil text-pencil
          placeholder:text-muted-dark
          focus:outline-none focus:border-blue focus:ring-2 focus:ring-blue/20
          transition-colors resize-y
          ${className}
        `}
        style={{
          borderRadius: wobbly.md,
          fontFamily: "'Courier New', monospace",
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
}

interface HandSelectProps {
  label?: string;
  value: string;
  onChange: (value: string) => void;
  options: SelectOption[];
  className?: string;
}

export function HandSelect({ label, value, onChange, options, className = '' }: HandSelectProps) {
  const [open, setOpen] = useState(false);
  const [focusIdx, setFocusIdx] = useState(-1);
  const containerRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  const selectedLabel = options.find((o) => o.value === value)?.label ?? value;

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
        <label
          className="block text-base text-pencil-light mb-1"
          style={{ fontFamily: 'var(--font-hand)' }}
        >
          {label}
        </label>
      )}
      <button
        type="button"
        onClick={() => { setOpen(!open); setFocusIdx(options.findIndex((o) => o.value === value)); }}
        onKeyDown={handleKeyDown}
        className={`
          w-full px-4 py-2.5 bg-surface border-2 border-pencil text-pencil text-left
          flex items-center justify-between gap-2
          focus:outline-none focus:border-blue focus:ring-2 focus:ring-blue/20
          transition-colors cursor-pointer
        `}
        style={{
          borderRadius: wobbly.sm,
          fontFamily: 'var(--font-hand)',
          fontSize: '1rem',
        }}
        role="combobox"
        aria-expanded={open}
        aria-haspopup="listbox"
      >
        <span className="truncate">{selectedLabel}</span>
        <ChevronDown
          size={16}
          strokeWidth={2.5}
          className={`shrink-0 text-pencil-light transition-transform duration-150 ${open ? 'rotate-180' : ''}`}
        />
      </button>
      {open && (
        <ul
          ref={listRef}
          role="listbox"
          className="absolute z-50 mt-1 w-full bg-surface border-2 border-pencil overflow-auto py-1"
          style={{
            borderRadius: wobbly.sm,
            boxShadow: shadows.md,
            fontFamily: 'var(--font-hand)',
            fontSize: '1rem',
            maxHeight: '15rem',
          }}
        >
          {options.map((opt, i) => (
            <li
              key={opt.value}
              role="option"
              aria-selected={opt.value === value}
              className={`
                px-4 py-2 cursor-pointer flex items-center gap-2 transition-colors
                ${i === focusIdx ? 'bg-blue/10' : ''}
                ${opt.value === value ? 'text-pencil font-medium' : 'text-pencil-light'}
                hover:bg-blue/10
              `}
              onMouseEnter={() => setFocusIdx(i)}
              onMouseDown={(e) => { e.preventDefault(); select(opt.value); }}
            >
              <span className="w-4 shrink-0">
                {opt.value === value && <Check size={14} strokeWidth={3} className="text-blue" />}
              </span>
              <span>{opt.label}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

interface HandCheckboxProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  className?: string;
}

export function HandCheckbox({ label, checked, onChange, className = '' }: HandCheckboxProps) {
  return (
    <label
      className={`inline-flex items-center gap-2 cursor-pointer select-none ${className}`}
      style={{ fontFamily: 'var(--font-hand)' }}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="sr-only"
      />
      <span
        className={`
          w-5 h-5 flex items-center justify-center border-2 transition-colors
          ${checked ? 'bg-blue border-blue' : 'bg-surface border-pencil'}
        `}
        style={{ borderRadius: wobbly.sm }}
      >
        {checked && <Check size={14} strokeWidth={3} className="text-white" />}
      </span>
      <span className="text-base text-pencil">{label}</span>
    </label>
  );
}
