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
  /** Start the actions on the title row and stack them at the right, instead of beside the subtitle's last line */
  actionsOnTitleRow?: boolean;
}

export default function PageHeader({
  title,
  subtitle,
  actions,
  className = '',
  backTo,
  crumbs,
  mono,
  actionsOnTitleRow,
}: PageHeaderProps) {
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
                <Link to={c.to} className="hover:text-ink">
                  {c.label}
                </Link>
              ) : c.onClick ? (
                <button type="button" className="cursor-pointer hover:text-ink" onClick={c.onClick}>
                  {c.label}
                </button>
              ) : (
                <span aria-current="page">{c.label}</span>
              )}
            </Fragment>
          ))}
        </nav>
      )}
      <div className={actionsOnTitleRow ? 'ss-ph !items-start' : 'ss-ph'}>
        <div className="tt min-w-0">
          <h1 className="ss-h1">
            <span className={mono ? 'm' : undefined}>{title}</span>
          </h1>
          {subtitle && <p>{subtitle}</p>}
        </div>
        {actions && (
          // Stacked at the right, starting on the title row. The column takes the room the title leaves, down
          // to its narrowest item, so it stays beside the title on narrow screens too.
          <div
            className={
              actionsOnTitleRow
                ? 'ml-auto flex min-w-min max-w-max grow-[9999] basis-0 flex-col items-end gap-2'
                : 'flex shrink-0 items-center gap-2'
            }
          >
            {actions}
          </div>
        )}
      </div>
    </div>
  );
}
