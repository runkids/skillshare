import { Check } from 'lucide-react';
import { wobbly } from '../design';

interface SelectionToggleProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  className?: string;
}

export default function SelectionToggle({
  label,
  checked,
  onChange,
  className = '',
}: SelectionToggleProps) {
  return (
    <label className={`inline-flex cursor-pointer items-center ${className}`} title={label}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        aria-label={label}
        className="sr-only"
      />
      <span
        className={`
          pointer-events-none flex h-5 w-5 items-center justify-center border-2 transition-colors
          ${checked ? 'border-blue bg-blue text-white' : 'border-muted-dark bg-surface text-transparent'}
        `}
        style={{ borderRadius: wobbly.sm }}
        aria-hidden="true"
      >
        <Check size={13} strokeWidth={3} />
      </span>
    </label>
  );
}
