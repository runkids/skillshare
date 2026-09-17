import type { ReactNode, ButtonHTMLAttributes, Ref } from 'react';
import Spinner from './Spinner';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'danger' | 'warning' | 'ghost' | 'link';
  size?: 'xs' | 'sm' | 'md' | 'lg';
  loading?: boolean;
  ref?: Ref<HTMLButtonElement>;
}

const variantClasses = {
  primary: 'ss-btn pri',
  secondary: 'ss-btn',
  danger: 'ss-btn dng',
  warning: 'ss-btn text-warn border-warn',
  ghost: 'ss-btn ghost',
  link: 'inline-flex items-center gap-1.5 text-[13px] font-medium text-link hover:underline cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed',
};

const sizeClasses = { xs: 'sm', sm: 'sm', md: '', lg: 'lg' };

export default function Button({
  children,
  variant = 'primary',
  size = 'md',
  className = '',
  disabled,
  loading = false,
  type = 'button',
  ref,
  ...props
}: ButtonProps) {
  const isLink = variant === 'link';
  return (
    <button
      ref={ref}
      type={type}
      className={`${variantClasses[variant]} ${isLink ? '' : sizeClasses[size]} ${className}`}
      disabled={disabled || loading}
      {...props}
    >
      {loading && (isLink ? <Spinner size="sm" className="text-current" /> : <span className="spin animate-spin" aria-hidden="true" />)}
      {children}
    </button>
  );
}
