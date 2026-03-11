import { radius } from '../design';

interface BadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'danger' | 'info' | 'accent';
  size?: 'sm' | 'md';
  dot?: boolean;
}

const variants: Record<string, string> = {
  default: 'bg-muted text-pencil-light',
  success: 'bg-success-light text-success',
  warning: 'bg-warning-light text-warning',
  danger: 'bg-danger-light text-danger',
  info: 'bg-info-light text-blue',
  accent: 'bg-accent/10 text-accent',
};

const dotColors: Record<string, string> = {
  default: 'bg-pencil-light',
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger',
  info: 'bg-blue',
  accent: 'bg-accent',
};

const sizeClasses = {
  sm: 'px-1.5 py-0 text-[10px]',
  md: 'px-2 py-0.5 text-xs',
};

export default function Badge({ children, variant = 'default', size = 'sm', dot = false }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center gap-1 font-medium ${variants[variant]} ${sizeClasses[size]}`}
      style={{
        borderRadius: radius.sm,
      }}
    >
      {dot && (
        <span className={`w-1.5 h-1.5 rounded-full ${dotColors[variant]}`} />
      )}
      {children}
    </span>
  );
}
