import type { ReactNode, CSSProperties } from 'react';

interface CardProps {
  children: ReactNode;
  className?: string;
  variant?: 'default' | 'accent' | 'outlined';
  hover?: boolean;
  overflow?: boolean;
  /** @deprecated Cards no longer tilt. Kept so existing call sites compile. */
  tilt?: boolean;
  padding?: 'none' | 'sm' | 'md';
  style?: CSSProperties;
  /** @deprecated No visual effect. */
  skillCard?: boolean;
  onClick?: () => void;
}

const paddingClasses = { none: 'p-0', sm: 'p-3', md: '' };

export default function Card({
  children,
  className = '',
  variant = 'default',
  hover = false,
  overflow = false,
  padding = 'md',
  style,
  onClick,
}: CardProps) {
  const interactive = !!onClick;
  return (
    <div
      onClick={onClick}
      onKeyDown={interactive ? (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick!(); } } : undefined}
      role={interactive ? 'button' : undefined}
      tabIndex={interactive ? 0 : undefined}
      className={`ss-box relative ${paddingClasses[padding]} ${overflow ? '' : 'overflow-hidden'} ${variant === 'outlined' ? 'bg-transparent shadow-none' : ''} ${hover ? 'cursor-pointer transition-transform hover:-translate-y-px' : ''} ${className}`}
      style={style}
    >
      {children}
    </div>
  );
}
