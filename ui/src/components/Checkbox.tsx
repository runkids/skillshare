import { useId } from 'react';
import { Check, Minus } from 'lucide-react';

interface CheckboxProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  className?: string;
  indeterminate?: boolean;
  disabled?: boolean;
  size?: 'sm' | 'md';
  /** Keep the label for screen readers only (row selection boxes). */
  hideLabel?: boolean;
}

export function Checkbox({
  label,
  checked,
  onChange,
  className = '',
  indeterminate = false,
  disabled = false,
  size = 'md',
  hideLabel = false,
}: CheckboxProps) {
  const id = useId();
  return (
    <label
      htmlFor={id}
      className={`inline-flex items-center gap-2 select-none ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'} ${className}`}
    >
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={(e) => !disabled && onChange(e.target.checked)}
        disabled={disabled}
        className="sr-only ss-chk-input"
      />
      <span className={`ss-chk ${checked || indeterminate ? 'on' : ''}`}>
        {indeterminate ? <Minus size={12} strokeWidth={3} /> : checked ? <Check size={12} strokeWidth={3} /> : null}
      </span>
      <span className={hideLabel ? 'sr-only' : `${size === 'sm' ? 'text-[13px]' : 'text-sm'} text-ink`}>{label}</span>
    </label>
  );
}
