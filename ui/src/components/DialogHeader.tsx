import type { HTMLAttributes, ReactNode, Ref } from 'react';
import { X } from 'lucide-react';
import { useT } from '../i18n';

interface DialogHeaderProps {
  title: ReactNode;
  /** Pass null, not undefined, while a conditional subtitle is hidden: the title then keeps its column */
  subtitle?: ReactNode;
  /** Shows the close button */
  onClose?: () => void;
  closeDisabled?: boolean;
  /** Extra header actions, before the close button */
  children?: ReactNode;
  /** Extra classes on `.dh` */
  className?: string;
  titleClassName?: string;
  /** Attributes for the subtitle paragraph; `className` replaces its default text style */
  subtitleProps?: HTMLAttributes<HTMLParagraphElement>;
  ref?: Ref<HTMLDivElement>;
}

/** The `.dh` row of a `DialogShell` with `padding="none"` */
export default function DialogHeader({
  title,
  subtitle,
  onClose,
  closeDisabled,
  children,
  className,
  titleClassName,
  subtitleProps,
  ref,
}: DialogHeaderProps) {
  const t = useT();
  const heading = <h2 className={titleClassName ? `ss-h2 ${titleClassName}` : 'ss-h2'}>{title}</h2>;

  return (
    <div ref={ref} className={className ? `dh ${className}` : 'dh'}>
      {subtitle === undefined ? (
        heading
      ) : (
        <div className="flex min-w-0 flex-col gap-1">
          {heading}
          {subtitle && (
            <p className="text-[13px] text-ink-2" {...subtitleProps}>
              {subtitle}
            </p>
          )}
        </div>
      )}
      {children}
      {onClose && (
        <button
          type="button"
          className="ss-ib shrink-0"
          aria-label={t('common.close')}
          onClick={onClose}
          disabled={closeDisabled}
        >
          <X size={16} />
        </button>
      )}
    </div>
  );
}
