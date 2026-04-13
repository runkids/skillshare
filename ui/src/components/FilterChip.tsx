import type { ReactNode } from 'react';
import { shadows, wobbly } from '../design';

interface FilterChipProps {
  label: string;
  icon: ReactNode;
  active: boolean;
  count: number;
  onClick: () => void;
}

export default function FilterChip({
  label,
  icon,
  active,
  count,
  onClick,
}: FilterChipProps) {
  return (
    <button
      onClick={onClick}
      className={`
        inline-flex items-center gap-1.5 px-3 py-1.5 border-2 text-sm
        transition-all duration-150 cursor-pointer select-none
        ${
          active
            ? 'bg-pencil text-white border-pencil dark:bg-blue dark:border-blue'
            : 'bg-surface text-pencil-light border-muted hover:border-pencil hover:text-pencil'
        }
      `}
      style={{
        borderRadius: wobbly.full,
        fontFamily: 'var(--font-hand)',
        boxShadow: active ? shadows.hover : 'none',
      }}
    >
      {icon}
      <span>{label}</span>
      <span
        className={`
          text-xs px-1.5 py-0.5 rounded-full min-w-[20px] text-center
          ${active ? 'bg-white/20 text-white' : 'bg-muted text-pencil-light'}
        `}
      >
        {count}
      </span>
    </button>
  );
}
