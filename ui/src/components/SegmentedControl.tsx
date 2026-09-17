import type { ReactNode } from 'react';
import { radius, shadows } from '../design';

interface Option<T extends string> {
  value: T;
  label: ReactNode;
  count?: number;
}

interface SegmentedControlProps<T extends string> {
  value: T;
  onChange: (value: T) => void;
  options: Option<T>[];
  size?: 'sm' | 'md';
  /** Connected mode: buttons share a border container (no gaps) */
  connected?: boolean;
  /** Custom active color per option (for severity tabs, etc.) */
  colorFn?: (value: T) => string | undefined;
}

const sizeClasses = {
  sm: 'px-3 py-1.5 text-sm',
  md: 'px-4 py-2 text-sm',
};

export default function SegmentedControl<T extends string>({
  value,
  onChange,
  options,
  size = 'sm',
  connected = false,
  colorFn,
}: SegmentedControlProps<T>) {
  if (connected) {
    return (
      <div
        className="ss-segmented ss-segmented-connected inline-flex flex-wrap items-center gap-0.5 p-1 border border-muted bg-muted/40"
        style={{ borderRadius: radius.sm }}
      >
        {options.map((opt) => {
          const isActive = value === opt.value;
          const color = colorFn?.(opt.value);
          return (
            <button
              key={opt.value}
              onClick={() => onChange(opt.value)}
              className={`
                ss-segmented-item inline-flex items-center whitespace-nowrap ${sizeClasses[size]} transition-colors cursor-pointer font-medium
                ${isActive
                  ? color ? '' : 'bg-surface text-pencil shadow-sm border border-muted'
                  : 'text-pencil-light hover:text-pencil border border-transparent'
                }
              `}
              style={{
                borderRadius: radius.sm,
                ...(isActive && color ? { backgroundColor: color, color: 'var(--color-paper)' } : {}),
              }}
              aria-pressed={isActive}
            >
              {opt.label}
              {opt.count != null && (
                <span className={`ml-1 ${isActive ? 'opacity-80' : 'opacity-50'}`}>
                  {opt.count}
                </span>
              )}
            </button>
          );
        })}
      </div>
    );
  }

  return (
    <div className="ss-segmented inline-flex flex-wrap items-center gap-1">
      {options.map((opt) => {
        const isActive = value === opt.value;
        const color = colorFn?.(opt.value);
        return (
          <button
            key={opt.value}
            onClick={() => onChange(opt.value)}
            className={`
              ss-segmented-item inline-flex items-center whitespace-nowrap ${sizeClasses[size]} border transition-all duration-150 cursor-pointer font-medium
              ${isActive
                ? color ? '' : 'bg-surface text-pencil border-muted-dark'
                : 'bg-transparent text-pencil-light border-muted hover:border-muted-dark hover:text-pencil'
              }
            `}
            style={{
              borderRadius: radius.sm,
              ...(isActive
                ? color
                  ? { backgroundColor: color, borderColor: color, color: 'var(--color-paper)', boxShadow: shadows.sm }
                  : {}
                : {}),
            }}
            aria-pressed={isActive}
          >
            {opt.label}
            {opt.count != null && (
              <span className={`ml-1 ${isActive ? 'opacity-80' : 'opacity-50'}`}>
                {opt.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
