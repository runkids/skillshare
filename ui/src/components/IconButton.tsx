import type { ReactNode, ButtonHTMLAttributes } from 'react';
import { radius } from '../design';

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon: ReactNode;
  /** Required accessible label */
  label: string;
  size?: 'sm' | 'md' | 'lg';
  variant?: 'ghost' | 'outline' | 'danger-outline';
}

const sizeClasses = {
  sm: 'w-6 h-6',
  md: 'w-8 h-8',
  lg: 'w-9 h-9',
};

const variantClasses = {
  ghost: 'text-pencil-light hover:text-pencil border border-transparent hover:border-muted hover:bg-muted/30',
  outline: 'border-2 border-transparent hover:border-muted-dark text-pencil-light hover:text-pencil',
  'danger-outline': 'border-2 border-transparent hover:border-danger text-muted-dark hover:text-danger',
};

export default function IconButton({
  icon,
  label,
  size = 'md',
  variant = 'ghost',
  className = '',
  style,
  ...props
}: IconButtonProps) {
  return (
    <button
      aria-label={label}
      title={label}
      className={`
        inline-flex items-center justify-center shrink-0
        transition-all duration-150 cursor-pointer
        active:scale-95
        focus-visible:ring-2 focus-visible:ring-pencil/20
        disabled:opacity-50 disabled:cursor-not-allowed
        ${sizeClasses[size]}
        ${variantClasses[variant]}
        ${className}
      `}
      style={{
        borderRadius: radius.sm,
        ...style,
      }}
      {...props}
    >
      {icon}
    </button>
  );
}
