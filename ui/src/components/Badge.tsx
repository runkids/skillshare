interface BadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'danger' | 'info' | 'accent';
  size?: 'sm' | 'md';
  dot?: boolean;
}

const variants = { default: '', success: 'ok', warning: 'warn', danger: 'bad', info: 'inf', accent: 'warn' };

export default function Badge({ children, variant = 'default', size = 'sm', dot = false }: BadgeProps) {
  return (
    <span className={`ss-tag ${variants[variant]} ${size === 'md' ? 'h-[22px] px-2 text-xs' : ''}`}>
      {dot && <span className="w-1.5 h-1.5 rounded-full bg-current" />}
      {children}
    </span>
  );
}
