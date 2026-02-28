import type { ReactNode, CSSProperties } from 'react';
import { wobbly, shadows } from '../design';

interface CardProps {
  children: ReactNode;
  className?: string;
  variant?: 'default' | 'postit' | 'accent' | 'outlined';
  decoration?: 'none' | 'tape' | 'tack';
  hover?: boolean;
  style?: CSSProperties;
}

const variantStyles = {
  default: 'bg-surface border-2 border-pencil',
  postit: 'bg-postit border-2 border-pencil',
  accent: 'bg-surface border-[3px] border-accent',
  outlined: 'bg-transparent border-2 border-dashed border-pencil-light',
};

export default function Card({
  children,
  className = '',
  variant = 'default',
  decoration = 'none',
  hover = false,
  style,
}: CardProps) {
  return (
    <div
      className={`relative p-4 overflow-hidden transition-all duration-100 ${variantStyles[variant]} ${
        hover
          ? 'cursor-pointer hover:translate-x-[2px] hover:translate-y-[2px] hover:rotate-[0.5deg]'
          : ''
      } ${className}`}
      style={{
        borderRadius: wobbly.md,
        boxShadow: hover ? shadows.md : shadows.sm,
        ...(hover
          ? {}
          : {}),
        ...style,
      }}
      onMouseEnter={
        hover
          ? (e) => {
              (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.hover;
            }
          : undefined
      }
      onMouseLeave={
        hover
          ? (e) => {
              (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.md;
            }
          : undefined
      }
    >
      {/* Tape decoration */}
      {decoration === 'tape' && (
        <div
          className="absolute -top-3 left-1/2 -translate-x-1/2 w-16 h-6 bg-muted/60 rotate-[-2deg] z-10"
          style={{ borderRadius: '2px' }}
        />
      )}

      {/* Tack decoration */}
      {decoration === 'tack' && (
        <div className="absolute -top-2 left-1/2 -translate-x-1/2 z-10">
          <div className="w-4 h-4 rounded-full bg-accent border-2 border-pencil shadow-sm" />
        </div>
      )}

      {children}
    </div>
  );
}
