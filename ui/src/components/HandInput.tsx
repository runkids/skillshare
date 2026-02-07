import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react';
import { Check } from 'lucide-react';
import { wobbly } from '../design';

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
          w-full px-4 py-2.5 bg-white border-2 border-pencil text-pencil
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
          w-full px-4 py-3 bg-white border-2 border-pencil text-pencil
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

interface HandSelectProps {
  label?: string;
  value: string;
  onChange: (e: React.ChangeEvent<HTMLSelectElement>) => void;
  children: React.ReactNode;
  className?: string;
}

export function HandSelect({ label, value, onChange, children, className = '' }: HandSelectProps) {
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
      <select
        value={value}
        onChange={onChange}
        className={`
          w-full px-4 py-2.5 bg-white border-2 border-pencil text-pencil
          focus:outline-none focus:border-blue focus:ring-2 focus:ring-blue/20
          transition-colors cursor-pointer
          ${className}
        `}
        style={{
          borderRadius: wobbly.sm,
          fontFamily: 'var(--font-hand)',
          fontSize: '1rem',
        }}
      >
        {children}
      </select>
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
          ${checked ? 'bg-blue border-blue' : 'bg-white border-pencil'}
        `}
        style={{ borderRadius: wobbly.sm }}
      >
        {checked && <Check size={14} strokeWidth={3} className="text-white" />}
      </span>
      <span className="text-base text-pencil">{label}</span>
    </label>
  );
}
