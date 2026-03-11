import type { ReactNode, CSSProperties } from 'react';
import { radius, shadows } from '../design';

interface CardProps {
  children: ReactNode;
  className?: string;
  variant?: 'default' | 'accent' | 'outlined';
  hover?: boolean;
  overflow?: boolean;
  tilt?: boolean;
  padding?: 'none' | 'sm' | 'md';
  style?: CSSProperties;
}

const variantStyles = {
  default: 'bg-surface border border-muted',
  accent: 'bg-surface border-2 border-muted-dark/30',
  outlined: 'border border-muted',
};

const variantShadows = {
  default: shadows.sm,
  accent: shadows.sm,
  outlined: 'none',
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
}: CardProps) {
  return (
    <div
      className={`
        relative ${paddingClasses[padding]}
        ${overflow ? 'overflow-visible' : 'overflow-hidden'}
        transition-all duration-150
        ${variantStyles[variant]}
        ${hover ? 'cursor-pointer hover:shadow-md hover:translate-y-[-1px]' : ''}
        ${tilt ? 'card-tilt' : ''}
        ${className}
      `}
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
