import type { ReactNode, ButtonHTMLAttributes, Ref } from 'react';
import Spinner from './Spinner';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost' | 'link';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
  ref?: Ref<HTMLButtonElement>;
}

const variantClasses = {
  primary: 'bg-pencil text-paper border-2 border-pencil hover:bg-pencil/85',
  secondary: 'bg-transparent text-pencil border-2 border-muted-dark hover:bg-muted/30 hover:border-pencil hover:shadow-sm',
  danger: 'bg-transparent text-danger border-2 border-danger hover:bg-danger hover:text-white',
  ghost: 'bg-transparent text-pencil-light hover:text-pencil hover:bg-muted/30',
  link: 'bg-transparent text-pencil-light hover:text-pencil hover:underline border-none',
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
  loading = false,
  style,
  ref,
  ...props
}: ButtonProps) {
  const isLink = variant === 'link';
  const isGhostOrLink = variant === 'ghost' || variant === 'link';
  const isDisabled = disabled || loading;
  return (
    <button
      ref={ref}
      className={`
        ${isGhostOrLink ? '' : 'ss-btn'}
        inline-flex items-center justify-center gap-2
        font-medium
        transition-all duration-150 cursor-pointer
        active:scale-[0.98]
        focus-visible:ring-2 focus-visible:ring-pencil/20 focus-visible:ring-offset-2
        disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100
        ${variantClasses[variant]}
        ${isLink ? 'text-sm p-0' : `${sizeClasses[size]} rounded-[var(--radius-btn)]`}
        ${className}
      `}
      style={style}
      disabled={isDisabled}
      {...props}
    >
      {loading && <Spinner size="sm" className="text-current" />}
      {children}
    </button>
  );
}
