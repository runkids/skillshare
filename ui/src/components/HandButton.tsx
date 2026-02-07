import type { ReactNode, ButtonHTMLAttributes } from 'react';
import { wobbly, shadows } from '../design';

interface HandButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  size?: 'sm' | 'md' | 'lg';
}

const variantClasses = {
  primary:
    'bg-white border-[3px] border-pencil text-pencil hover:bg-accent hover:text-white hover:border-accent',
  secondary:
    'bg-muted border-2 border-pencil-light text-pencil hover:bg-blue hover:text-white hover:border-blue',
  danger:
    'bg-white border-2 border-danger text-danger hover:bg-danger hover:text-white',
  ghost:
    'bg-transparent border-2 border-dashed border-pencil-light text-pencil-light hover:border-pencil hover:text-pencil',
};

const sizeClasses = {
  sm: 'px-3 py-1.5 text-base',
  md: 'px-5 py-2.5 text-base',
  lg: 'px-8 py-3.5 text-lg',
};

export default function HandButton({
  children,
  variant = 'primary',
  size = 'md',
  className = '',
  disabled,
  style,
  ...props
}: HandButtonProps) {
  return (
    <button
      className={`
        inline-flex items-center justify-center gap-2
        font-[var(--font-hand)] font-medium
        transition-all duration-100 cursor-pointer
        active:translate-x-[4px] active:translate-y-[4px]
        disabled:opacity-50 disabled:cursor-not-allowed disabled:translate-x-0 disabled:translate-y-0
        ${variantClasses[variant]}
        ${sizeClasses[size]}
        ${className}
      `}
      style={{
        borderRadius: wobbly.btn,
        boxShadow: disabled ? shadows.sm : shadows.md,
        fontFamily: 'var(--font-hand)',
        ...style,
      }}
      onMouseEnter={(e) => {
        if (!disabled) {
          e.currentTarget.style.boxShadow = shadows.hover;
          e.currentTarget.style.transform = 'translate(2px, 2px)';
        }
      }}
      onMouseLeave={(e) => {
        if (!disabled) {
          e.currentTarget.style.boxShadow = shadows.md;
          e.currentTarget.style.transform = 'translate(0, 0)';
        }
      }}
      onMouseDown={(e) => {
        if (!disabled) {
          e.currentTarget.style.boxShadow = shadows.active;
        }
      }}
      onMouseUp={(e) => {
        if (!disabled) {
          e.currentTarget.style.boxShadow = shadows.hover;
        }
      }}
      disabled={disabled}
      {...props}
    >
      {children}
    </button>
  );
}
