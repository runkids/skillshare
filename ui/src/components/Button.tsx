import type { ReactNode, ButtonHTMLAttributes } from 'react';
import { radius } from '../design';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  size?: 'sm' | 'md' | 'lg';
}

const variantClasses = {
  primary: 'bg-pencil text-paper border border-pencil hover:opacity-80',
  secondary: 'bg-transparent text-pencil border border-muted hover:border-pencil hover:text-pencil',
  danger: 'bg-danger text-white border border-danger hover:opacity-80',
  ghost: 'bg-transparent text-pencil-light hover:text-pencil',
};

const sizeClasses = {
  sm: 'px-3 py-1.5 text-base',
  md: 'px-5 py-2.5 text-base',
  lg: 'px-8 py-3.5 text-lg',
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
