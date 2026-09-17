import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: ReactNode;
}

export default function EmptyState({ icon: Icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="ss-empty">
      <Icon size={24} className="text-ink-3" />
      <h3 className="font-semibold text-ink">{title}</h3>
      {description && <p className="text-[13px] max-w-sm">{description}</p>}
      {action && <div className="mt-2 flex items-center gap-2">{action}</div>}
    </div>
  );
}
