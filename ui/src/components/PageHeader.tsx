import { Fragment } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, ChevronRight } from 'lucide-react';
import { useT } from '../i18n';

interface PageHeaderProps {
  title: string;
  subtitle?: React.ReactNode;
  /** @deprecated Page headers no longer show an icon. Kept so existing call sites compile. */
  icon?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
  /** Show a back link to this path above the title */
  backTo?: string;
  /** Breadcrumb trail above the title; the last crumb is the current page */
  crumbs?: { label: string; to?: string; onClick?: () => void }[];
  /** Set the title in monospace, for resource names */
  mono?: boolean;
}

export default function PageHeader({ title, subtitle, actions, className = '', backTo, crumbs, mono }: PageHeaderProps) {
  const t = useT();
  return (
    <div className={`ss-pgh flex flex-col gap-3 ${className}`}>
      {backTo && (
        <Link to={backTo} className="ss-crumb !mb-0 w-fit hover:text-ink">
          <ArrowLeft size={14} />
          {t('common.back')}
        </Link>
      )}
      {crumbs && (
        <nav className="ss-crumb !mb-0" aria-label="Breadcrumb">
          {crumbs.map((c, i) => (
            <Fragment key={i}>
              {i > 0 && <ChevronRight size={13} />}
              {c.to ? (
                <Link to={c.to} className="hover:text-ink">{c.label}</Link>
              ) : c.onClick ? (
                <button type="button" className="cursor-pointer hover:text-ink" onClick={c.onClick}>{c.label}</button>
              ) : (
                <span aria-current="page">{c.label}</span>
              )}
            </Fragment>
          ))}
        </nav>
      )}
      <div className="ss-ph">
        <div className="tt min-w-0">
          <h1 className="ss-h1"><span className={mono ? 'm' : undefined}>{title}</span></h1>
          {subtitle && <p>{subtitle}</p>}
        </div>
        {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
      </div>
    </div>
  );
}
