import type { ReactNode } from 'react';

interface EditorSegmentOption<T extends string> {
  value: T;
  label: ReactNode;
}

interface EditorSegmentProps<T extends string> {
  value: T;
  onChange: (next: T) => void;
  options: EditorSegmentOption<T>[];
  className?: string;
  title?: string;
  role?: 'radiogroup';
}

export default function EditorSegment<T extends string>({
  value,
  onChange,
  options,
  className = '',
  title,
  role,
}: EditorSegmentProps<T>) {
  return (
    <div className={`seg-group ${className}`.trim()} title={title} role={role}>
      {options.map((opt) => {
        const active = value === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            className={`seg-btn ${active ? 'active' : ''}`}
            onClick={() => onChange(opt.value)}
            aria-pressed={active}
            {...(role === 'radiogroup' ? { role: 'radio', 'aria-checked': active } : {})}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
