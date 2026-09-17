import type { ReactNode } from 'react';

interface Option<T extends string> {
  value: T;
  label: ReactNode;
  count?: number;
  /** Accessible name for icon-only options. */
  title?: string;
}

interface SegmentedControlProps<T extends string> {
  value: T;
  onChange: (value: T) => void;
  options: Option<T>[];
  size?: 'sm' | 'md';
  /** @deprecated Every segmented control now uses the connected look. */
  connected?: boolean;
  /** Custom active text colour per option (for severity tabs, etc.) */
  colorFn?: (value: T) => string | undefined;
  className?: string;
}

export default function SegmentedControl<T extends string>({ value, onChange, options, colorFn, className = '' }: SegmentedControlProps<T>) {
  return (
    <div className={`ss-seg flex-wrap ${className}`}>
      {options.map((opt) => {
        const on = value === opt.value;
        const color = on ? colorFn?.(opt.value) : undefined;
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            className={`whitespace-nowrap ${on ? 'on' : ''}`}
            style={color ? { color } : undefined}
            aria-pressed={on}
            title={opt.title}
            aria-label={opt.title}
          >
            {opt.label}
            {opt.count != null && <span className="ss-cnt">{opt.count}</span>}
          </button>
        );
      })}
    </div>
  );
}
