import { wobbly } from '../design';

interface BadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'danger' | 'info' | 'accent';
}

const variants: Record<string, string> = {
  default: 'bg-muted text-pencil-light border-pencil-light',
  success: 'bg-success-light text-success border-success',
  warning: 'bg-warning-light text-warning border-warning',
  danger: 'bg-danger-light text-danger border-danger',
  info: 'bg-info-light text-blue border-blue',
  accent: 'bg-accent/10 text-accent border-accent',
};

export default function Badge({ children, variant = 'default' }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center px-2.5 py-1 border text-sm font-[var(--font-hand)] ${variants[variant]}`}
      style={{
        borderRadius: wobbly.sm,
        fontFamily: 'var(--font-hand)',
      }}
    >
      {children}
    </span>
  );
}
