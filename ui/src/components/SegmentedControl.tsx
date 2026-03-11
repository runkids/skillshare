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
        className="inline-flex items-center border-2 border-muted overflow-hidden"
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
                ${sizeClasses[size]} transition-colors cursor-pointer font-medium
                ${isActive
                  ? color ? '' : 'bg-pencil text-paper'
                  : 'bg-surface text-pencil-light hover:text-pencil'
                }
              `}
              style={isActive && color ? { backgroundColor: color, color: 'var(--color-paper)' } : undefined}
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
    <div className="inline-flex items-center gap-1">
      {options.map((opt) => {
        const isActive = value === opt.value;
        const color = colorFn?.(opt.value);
        return (
          <button
            key={opt.value}
            onClick={() => onChange(opt.value)}
            className={`
              ${sizeClasses[size]} border-2 transition-all duration-150 cursor-pointer font-medium
              ${isActive
                ? color ? '' : 'bg-pencil text-paper border-pencil'
                : 'bg-transparent text-pencil border-muted-dark hover:border-pencil'
              }
            `}
            style={{
              borderRadius: radius.sm,
              ...(isActive
                ? color
                  ? { backgroundColor: color, borderColor: color, color: 'var(--color-paper)', boxShadow: shadows.sm }
                  : { boxShadow: shadows.sm }
                : {}),
            }}
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
