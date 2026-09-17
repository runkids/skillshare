import type { ReactNode, ButtonHTMLAttributes } from 'react';

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon: ReactNode;
  /** Required accessible label */
  label: string;
  size?: 'sm' | 'md' | 'lg';
  variant?: 'ghost' | 'outline' | 'danger-outline';
}

const sizeClasses = { sm: 'w-6 h-6', md: '', lg: 'w-9 h-9' };

export default function IconButton({
  icon,
  label,
  size = 'md',
  variant = 'ghost',
  className = '',
  type = 'button',
  ...props
}: IconButtonProps) {
  return (
    <button
      type={type}
      aria-label={label}
      title={label}
      className={`ss-ib shrink-0 ${sizeClasses[size]} ${variant === 'danger-outline' ? 'hover:text-bad hover:bg-bad-bg' : ''} ${className}`}
      {...props}
    >
      {icon}
    </button>
  );
}
