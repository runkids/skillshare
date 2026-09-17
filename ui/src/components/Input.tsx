import { useId } from 'react';
import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react';

// Re-export split components for backward compatibility
export { Checkbox } from './Checkbox';
export { Select, type SelectOption } from './Select';

interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  label?: string;
  size?: 'sm' | 'md';
}

const control = 'ss-inp w-full outline-none placeholder:text-ink-3 disabled:opacity-50';

export function Input({ label, className = '', id, size = 'md', ...props }: InputProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  return (
    <div className={label ? 'ss-fld' : undefined}>
      {label && <label htmlFor={inputId}>{label}</label>}
      <input id={inputId} className={`${control} ${size === 'sm' ? 'h-[30px] text-xs' : ''} ${className}`} {...props} />
    </div>
  );
}

interface TextareaProps extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'size'> {
  label?: string;
  size?: 'sm' | 'md';
}

export function Textarea({ label, className = '', id, size = 'md', ...props }: TextareaProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  return (
    <div className={label ? 'ss-fld' : undefined}>
      {label && <label htmlFor={inputId}>{label}</label>}
      <textarea id={inputId} className={`${control} area resize-y ${size === 'sm' ? 'text-xs' : ''} ${className}`} {...props} />
    </div>
  );
}
