import { useId } from 'react';
import { Check, Minus } from 'lucide-react';
import { radius } from '../design';

interface CheckboxProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  className?: string;
  indeterminate?: boolean;
  disabled?: boolean;
  size?: 'sm' | 'md';
}

const sizeMap = {
  sm: { box: 'w-4 h-4', icon: 12, text: 'text-sm' },
  md: { box: 'w-5 h-5', icon: 14, text: 'text-base' },
};

export function Checkbox({
  label,
  checked,
  onChange,
  className = '',
  indeterminate = false,
  disabled = false,
  size = 'md',
}: CheckboxProps) {
  const id = useId();
  const s = sizeMap[size];

  return (
    <label
      htmlFor={id}
      className={`
        inline-flex items-center gap-2 select-none
        ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
        ${className}
      `}
    >
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={(e) => !disabled && onChange(e.target.checked)}
        disabled={disabled}
        className="sr-only"
      />
      <span
        className={`
          ${s.box} flex items-center justify-center border transition-all duration-150
          ${disabled ? '' : 'active:scale-95'}
          focus-visible:ring-2 focus-visible:ring-blue/30 focus-visible:ring-offset-1
          ${
            checked || indeterminate
              ? 'bg-blue border-blue'
              : 'bg-surface border-muted-dark hover:border-pencil-light'
          }
        `}
        style={{ borderRadius: radius.sm }}
      >
        {indeterminate ? (
          <Minus size={s.icon} strokeWidth={3} className="text-white" />
        ) : checked ? (
          <Check size={s.icon} strokeWidth={3} className="text-white" />
        ) : null}
      </span>
      <span className={`${s.text} text-pencil`}>{label}</span>
    </label>
  );
}
