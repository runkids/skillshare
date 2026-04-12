import { forwardRef, useId } from 'react';
import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react';

// Re-export split components for backward compatibility
export { Checkbox } from './Checkbox';
export { Select, type SelectOption } from './Select';

interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  label?: string;
  uiSize?: 'sm' | 'md';
  size?: InputHTMLAttributes<HTMLInputElement>['size'] | 'sm' | 'md';
}

const inputSizeClasses = {
  sm: 'px-3 py-2 text-sm',
  md: 'px-4 py-2.5 text-base',
};

export function Input({ label, className = '', style, id, uiSize, size, ...props }: InputProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const resolvedUiSize = uiSize ?? (size === 'sm' || size === 'md' ? size : 'md');
  const nativeSize = typeof size === 'number' ? size : undefined;

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
        size={nativeSize}
        className={`
          ss-input
          w-full bg-surface border-2 border-muted text-pencil
          placeholder:text-muted-dark
          hover:border-muted-dark
          focus:outline-none focus:border-pencil
          transition-all
          rounded-[var(--radius-md)]
          ${inputSizeClasses[resolvedUiSize]}
          ${className}
        `}
        style={{
          ...style,
        }}
        {...props}
      />
    </div>
  );
}

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  wrapperClassName?: string;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { label, className = '', style, id, wrapperClassName = '', ...props },
  ref,
) {
  const autoId = useId();
  const inputId = id ?? autoId;

  return (
    <div className={wrapperClassName}>
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
        ref={ref}
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
});
