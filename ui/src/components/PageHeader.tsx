import { Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { radius, shadows } from '../design';
import { useT } from '../i18n';

interface PageHeaderProps {
  title: string;
  subtitle?: React.ReactNode;
  icon: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
  /** Show a styled back button linking to this path */
  backTo?: string;
}

export default function PageHeader({ title, subtitle, icon, actions, className = '', backTo }: PageHeaderProps) {
  const t = useT();
  const heading = (
    <div className="flex items-center gap-3">
      {backTo && (
        <Link
          to={backTo}
          className="inline-flex items-center justify-center shrink-0 w-9 h-9 border-2 border-transparent hover:border-muted-dark text-pencil-light hover:text-pencil bg-surface transition-all duration-150 active:scale-95"
          aria-label={t('common.back')}
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <ArrowLeft size={18} strokeWidth={2.5} />
        </Link>
      )}
      <div>
        <h2 className="text-2xl md:text-3xl font-bold text-pencil flex items-center gap-2">
          {icon}
          {title}
        </h2>
        {subtitle && <p className="text-pencil-light mt-1">{subtitle}</p>}
      </div>
    </div>
  );

  return (
    <div
      className={`mb-6 ${actions ? 'flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4' : ''} ${className}`.trim()}
    >
      {heading}
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  );
}
