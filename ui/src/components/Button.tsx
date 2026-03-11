import type { ReactNode, ButtonHTMLAttributes } from 'react';
import { radius } from '../design';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost' | 'link';
  size?: 'sm' | 'md' | 'lg';
}

const variantClasses = {
  primary: 'bg-pencil text-paper border-2 border-pencil hover:opacity-80',
  secondary: 'bg-transparent text-pencil border-2 border-muted-dark hover:bg-muted/30 hover:border-pencil',
  danger: 'bg-transparent text-danger border-2 border-danger hover:bg-danger hover:text-white',
  ghost: 'bg-transparent text-pencil-light hover:text-pencil hover:bg-muted/20',
  link: 'bg-transparent text-blue hover:underline border-none',
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
  const isLink = variant === 'link';
  return (
    <button
      className={`
        inline-flex items-center justify-center gap-2
        font-medium
        transition-all duration-150 cursor-pointer
        disabled:opacity-50 disabled:cursor-not-allowed
        ${variantClasses[variant]}
        ${isLink ? 'text-sm p-0' : sizeClasses[size]}
        ${className}
      `}
      style={{
        borderRadius: isLink ? 0 : radius.btn,
        ...style,
      }}
      disabled={disabled}
      {...props}
    >
      {children}
    </button>
  );
}
