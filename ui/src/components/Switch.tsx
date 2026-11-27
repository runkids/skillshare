import type { ButtonHTMLAttributes } from 'react';

interface MarkProps {
  checked: boolean;
  /** Only some of what it covers is on. */
  mixed?: boolean;
  className?: string;
}

const markClass = ({ checked, mixed, className = '' }: MarkProps) =>
  `ss-sw ${mixed ? 'mix' : checked ? 'on' : ''} ${className}`.trim();

/** The look of a switch alone, for a button that is the switch itself and holds more than the track. */
export function SwitchMark(props: MarkProps) {
  return (
    <span className={markClass(props)}>
      <i />
    </span>
  );
}

type SwitchProps = MarkProps &
  Omit<
    ButtonHTMLAttributes<HTMLButtonElement>,
    'type' | 'role' | 'aria-checked' | 'onClick' | 'onChange' | 'children'
  > & {
    onChange: (checked: boolean) => void;
  };

/** Name it with `aria-label` or `aria-labelledby`. */
export default function Switch({ checked, mixed, className, onChange, ...rest }: SwitchProps) {
  return (
    <button
      {...rest}
      type="button"
      role="switch"
      aria-checked={mixed ? 'mixed' : checked}
      className={markClass({ checked, mixed, className })}
      onClick={() => onChange(mixed ? true : !checked)}
    >
      <i />
    </button>
  );
}
