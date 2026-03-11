import type { ReactNode, ButtonHTMLAttributes } from 'react';
import { radius } from '../design';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  size?: 'sm' | 'md' | 'lg';
}

const variantClasses = {
  primary: 'bg-pencil text-paper border border-pencil hover:opacity-80',
  secondary: 'bg-transparent text-pencil border border-muted hover:bg-muted/30 hover:border-muted-dark',
  danger: 'bg-transparent text-danger border border-danger hover:bg-danger hover:text-white',
  ghost: 'bg-transparent text-pencil-light hover:text-pencil hover:bg-muted/20',
};

const sizeClasses = {
  sm: 'px-3 py-1.5 text-sm',
  md: 'px-5 py-2.5 text-sm',
  lg: 'px-6 py-3 text-base',
};

export default function Button({
  children,
  variant = 'primary',
  size = 'md',
  className = '',
  disabled,
  style,
  ...props
}: ButtonProps) {
  return (
    <button
      className={`
        inline-flex items-center justify-center gap-2
        font-medium
        transition-all duration-150 cursor-pointer
        disabled:opacity-50 disabled:cursor-not-allowed
        ${variantClasses[variant]}
        ${sizeClasses[size]}
        ${className}
      `}
      style={{
        borderRadius: radius.btn,
        ...style,
      }}
      disabled={disabled}
      {...props}
    >
      {children}
    </button>
  );
}
