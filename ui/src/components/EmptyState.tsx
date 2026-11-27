import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';

interface EmptyStateProps {
  icon?: LucideIcon;
  title?: string;
  /** With no icon or title, it is the whole message, in body text. */
  description?: ReactNode;
  action?: ReactNode;
  /** Something failed to load. */
  tone?: 'bad';
  /** A smaller one, for a section of a page. */
  compact?: boolean;
  className?: string;
}

export default function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  tone,
  compact,
  className = '',
}: EmptyStateProps) {
  const descriptionClass = compact
    ? 'max-w-[460px] text-[12.5px] leading-normal text-ink-3'
    : !Icon && !title
      ? ''
      : // An error message keeps the full width.
        `text-[13px] ${tone === 'bad' ? '' : 'max-w-sm'}`;
  return (
    <div className={`ss-empty ${compact ? '!gap-1.5 !p-[26px]' : ''} ${className}`}>
      {Icon && <Icon size={24} className={tone === 'bad' ? 'text-bad' : 'text-ink-3'} />}
      {title && (
        <h3 className={`font-semibold text-ink ${compact ? 'text-[13.5px]' : ''}`}>{title}</h3>
      )}
      {description && <p className={descriptionClass}>{description}</p>}
      {action && <div className="mt-2 flex items-center gap-2">{action}</div>}
    </div>
  );
}
