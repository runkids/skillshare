import type { ReactNode, CSSProperties } from 'react';
import { shadows } from '../design';

interface CardProps {
  children: ReactNode;
  className?: string;
  variant?: 'default' | 'accent' | 'outlined' | 'postit';
  hover?: boolean;
  overflow?: boolean;
  tilt?: boolean;
  padding?: 'none' | 'sm' | 'md';
  style?: CSSProperties;
  skillCard?: boolean;
  decoration?: 'tape' | 'none';
}

const variantStyles = {
  default: 'bg-surface border border-muted',
  accent: 'bg-surface border-2 border-muted-dark/30',
  outlined: 'border border-muted',
  postit: 'bg-surface border border-muted',
};

const variantShadows = {
  default: shadows.sm,
  accent: shadows.sm,
  outlined: 'none',
  postit: shadows.sm,
};

const paddingClasses = {
  none: 'p-0',
  sm: 'p-3',
  md: 'p-4',
};

export default function Card({
  children,
  className = '',
  variant = 'default',
  hover = false,
  overflow = false,
  tilt = false,
  padding = 'md',
  style,
  skillCard = false,
  decoration = 'none',
}: CardProps) {
  const showTape = skillCard || decoration === 'tape';

  return (
    <div
      className={`
        ss-card
        ${showTape ? 'ss-skill-card' : ''}
        relative ${paddingClasses[padding]}
        ${overflow ? 'overflow-visible' : 'overflow-hidden'}
        transition-all duration-150
        rounded-[var(--radius-md)]
        ${variantStyles[variant]}
        ${hover ? 'cursor-pointer hover:shadow-md hover:translate-y-[-1px]' : ''}
        ${tilt ? 'card-tilt' : ''}
        ${className}
      `}
      style={{
        boxShadow: variantShadows[variant],
        ...style,
      }}
    >
      {children}
    </div>
  );
}
