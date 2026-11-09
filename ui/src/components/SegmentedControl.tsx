import type { ReactNode } from 'react';
import { segSlide } from '../lib/segSlide';

interface Option<T extends string> {
  value: T;
  label: ReactNode;
  count?: number;
  /** Tooltip; an icon-only option (its icon is aria-hidden) is also named by it. */
  title?: string;
  disabled?: boolean;
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
    <div ref={segSlide} className={`ss-seg flex-wrap ${className}`}>
      {options.map((opt) => {
        const on = value === opt.value;
        const color = on ? colorFn?.(opt.value) : undefined;
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            disabled={opt.disabled}
            className={`whitespace-nowrap ${on ? 'on' : ''}`}
            style={color ? { color } : undefined}
            aria-pressed={on}
            title={opt.title}
          >
            {opt.label}
            {opt.count != null && <span className="ss-cnt">{opt.count}</span>}
          </button>
        );
      })}
    </div>
  );
}
