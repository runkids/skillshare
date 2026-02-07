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
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div
        className="w-16 h-16 border-2 border-dashed border-muted-dark rounded-full flex items-center justify-center mb-4"
      >
        <Icon size={28} strokeWidth={2} className="text-muted-dark" />
      </div>
      <h3
        className="text-xl text-pencil mb-1"
        style={{ fontFamily: 'var(--font-heading)' }}
      >
        {title}
      </h3>
      {description && (
        <p className="text-pencil-light text-base max-w-xs">{description}</p>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
