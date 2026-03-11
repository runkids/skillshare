import type { ReactNode, ButtonHTMLAttributes } from 'react';
import { radius } from '../design';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  size?: 'sm' | 'md' | 'lg';
}

const variantClasses = {
  primary: 'bg-accent text-white border border-accent hover:brightness-90',
  secondary: 'bg-transparent text-pencil border border-muted hover:bg-muted/30',
  danger: 'bg-danger text-white border border-danger hover:brightness-90',
  ghost: 'bg-transparent text-pencil hover:bg-muted/30',
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
        transition-all duration-100 cursor-pointer
        active:translate-x-[4px] active:translate-y-[4px]
        disabled:opacity-50 disabled:cursor-not-allowed disabled:translate-x-0 disabled:translate-y-0
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
