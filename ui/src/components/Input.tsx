import { useId } from 'react';
import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react';

// Re-export split components for backward compatibility
export { Checkbox } from './Checkbox';
export { Select, type SelectOption } from './Select';

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
          ss-input
          w-full px-4 py-2.5 bg-surface border-2 border-muted text-pencil
          placeholder:text-muted-dark
          hover:border-muted-dark
          focus:outline-none focus:border-pencil
          transition-all
          rounded-[var(--radius-md)]
          ${className}
        `}
        style={{
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
          ss-input
          w-full px-4 py-3 bg-surface border-2 border-muted text-pencil
          placeholder:text-muted-dark
          hover:border-muted-dark
          focus:outline-none focus:border-pencil
          transition-all resize-y
          rounded-[var(--radius-md)]
          ${className}
        `}
        style={{
          fontSize: '0.95rem',
          ...style,
        }}
        {...props}
      />
    </div>
  );
}
