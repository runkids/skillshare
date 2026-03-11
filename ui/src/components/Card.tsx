import type { ReactNode, CSSProperties } from 'react';
import { radius, shadows } from '../design';

interface CardProps {
  children: ReactNode;
  className?: string;
  variant?: 'default' | 'accent' | 'outlined';
  hover?: boolean;
  overflow?: boolean;
  style?: CSSProperties;
}

const variantStyles = {
  default: 'bg-surface border border-muted',
  accent: 'bg-surface border border-muted border-l-[3px] border-l-accent',
  outlined: 'border border-muted',
};

const variantShadows = {
  default: shadows.sm,
  accent: shadows.sm,
  outlined: 'none',
};

export default function Card({
  children,
  className = '',
  variant = 'default',
  hover = false,
  overflow = false,
  style,
}: CardProps) {
  return (
    <div
      className={`relative p-4 ${overflow ? 'overflow-visible' : 'overflow-hidden'} transition-shadow duration-150 ${variantStyles[variant]} ${
        hover ? 'cursor-pointer hover:shadow-md' : ''
      } ${className}`}
      style={{
        borderRadius: radius.md,
        boxShadow: variantShadows[variant],
        ...style,
      }}
    >
      {children}
    </div>
  );
}
