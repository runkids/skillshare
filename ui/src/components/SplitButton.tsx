import { useState, useRef, useEffect, useCallback, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { ChevronDown } from 'lucide-react';
import Spinner from './Spinner';

export interface SplitButtonItem {
  label: string;
  icon?: ReactNode;
  onClick: () => void;
  /** Show an inline "Confirm?" step before executing onClick */
  confirm?: boolean;
  /** Custom confirm label (default: "Confirm?") */
  confirmLabel?: string;
}

interface SplitButtonProps {
  children: ReactNode;
  onClick: () => void;
  items: SplitButtonItem[];
  variant?: 'primary' | 'secondary';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
  disabled?: boolean;
  className?: string;
  /** Align dropdown to the right edge instead of left */
  dropdownAlign?: 'left' | 'right';
}

/* ── Variant maps ─────────────────────────────────────────────── */

const variantShell = {
  primary: 'bg-pencil text-paper border-2 border-pencil',
  secondary: 'bg-transparent text-pencil border-2 border-muted-dark hover:border-pencil',
};

const variantHover = {
  primary: 'hover:bg-paper/10',
  secondary: 'hover:bg-muted/30',
};

const variantDivider = {
  primary: 'bg-paper/25',
  secondary: 'bg-muted-dark',
};

const variantMenu = {
  primary: 'bg-pencil text-paper',
  secondary: 'bg-surface/95 text-pencil',
};

const variantMenuStyle: Record<string, React.CSSProperties> = {
  primary: {
    border: '2px solid var(--color-pencil)',
    boxShadow: '0 8px 24px rgba(0, 0, 0, 0.12), 0 2px 6px rgba(0, 0, 0, 0.06)',
  },
  secondary: {
    border: '1px solid var(--color-muted-dark)',
    boxShadow: '0 8px 24px rgba(0, 0, 0, 0.1), 0 2px 4px rgba(0, 0, 0, 0.04)',
  },
};

const variantMenuItem = {
  primary: 'hover:bg-paper/15',
  secondary: 'hover:bg-muted/50',
};

/* ── Size maps ────────────────────────────────────────────────── */

const sizeMain = {
  sm: 'px-3 py-1.5 text-sm',
  md: 'px-5 py-2.5 text-sm',
  lg: 'px-6 py-3 text-base',
};

const sizeChevron = {
  sm: 'px-1.5',
  md: 'px-2',
  lg: 'px-2.5',
};

const sizeMenu = {
  sm: 'text-sm py-1',
  md: 'text-sm py-1',
  lg: 'text-base py-1.5',
};

const sizeMenuItem = {
  sm: 'px-3.5 py-2',
  md: 'px-4 py-2.5',
  lg: 'px-4 py-3',
};

/* ── Portal dropdown ──────────────────────────────────────────── */

function DropdownPortal({
  anchorRef,
  align,
  variant,
  size,
  items,
  onClose,
}: {
  anchorRef: React.RefObject<HTMLDivElement | null>;
  align: 'left' | 'right';
  variant: 'primary' | 'secondary';
  size: 'sm' | 'md' | 'lg';
  items: SplitButtonItem[];
  onClose: () => void;
}) {
  const menuRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left?: number; right?: number } | null>(null);
  const [confirmIdx, setConfirmIdx] = useState<number | null>(null);

  // Calculate position synchronously before paint
  useEffect(() => {
    if (!anchorRef.current) return;
    const rect = anchorRef.current.getBoundingClientRect();
    setPos({
      top: rect.bottom + 6,
      left: align === 'right' ? undefined : rect.left,
      right: align === 'right' ? window.innerWidth - rect.right : undefined,
    });

    // Close on scroll (cleaner than trying to track position)
    const dismiss = () => onClose();
    window.addEventListener('scroll', dismiss, true);
    window.addEventListener('resize', dismiss);
    return () => {
      window.removeEventListener('scroll', dismiss, true);
      window.removeEventListener('resize', dismiss);
    };
  }, [anchorRef, align, onClose]);

  // Close on click outside
  useEffect(() => {
    const handle = (e: MouseEvent) => {
      const target = e.target as Node;
      if (
        menuRef.current && !menuRef.current.contains(target) &&
        anchorRef.current && !anchorRef.current.contains(target)
      ) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handle);
    return () => document.removeEventListener('mousedown', handle);
  }, [anchorRef, onClose]);

  // Close on Escape
  useEffect(() => {
    const handle = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handle);
    return () => document.removeEventListener('keydown', handle);
  }, [onClose]);

  if (!pos) return null;

  return createPortal(
    <div
      ref={menuRef}
      className={`
        fixed ss-split-menu min-w-[160px]
        ${variantMenu[variant]}
        ${sizeMenu[size]}
        overflow-hidden
      `}
      style={{
        zIndex: 99999,
        top: pos.top,
        left: pos.left,
        right: pos.right,
        borderRadius: 'var(--radius-lg)',
        ...variantMenuStyle[variant],
      }}
      role="menu"
    >
      {items.map((item, i) => {
        const isConfirming = confirmIdx === i;
        return (
          <button
            key={i}
            className={`
              ss-split-menu-item
              w-full flex items-center gap-2.5 ${sizeMenuItem[size]}
              text-left cursor-pointer font-medium
              ${isConfirming
                ? (variant === 'primary' ? 'bg-paper/10 text-warning' : 'bg-warning/10 text-warning')
                : variantMenuItem[variant]
              }
            `}
            role="menuitem"
            onClick={() => {
              if (item.confirm && !isConfirming) {
                setConfirmIdx(i);
                return;
              }
              onClose();
              item.onClick();
            }}
          >
            {item.icon && (
              <span className="ss-split-menu-icon shrink-0">{item.icon}</span>
            )}
            {isConfirming
              ? (item.confirmLabel ?? 'Are you sure?')
              : item.label
            }
          </button>
        );
      })}
    </div>,
    document.body,
  );
}

/* ── Component ────────────────────────────────────────────────── */

export default function SplitButton({
  children,
  onClick,
  items,
  variant = 'primary',
  size = 'md',
  loading = false,
  disabled = false,
  className = '',
  dropdownAlign = 'left',
}: SplitButtonProps) {
  const [open, setOpen] = useState(false);
  const shellRef = useRef<HTMLDivElement>(null);

  const isDisabled = disabled || loading;
  // Stable close callback for DropdownPortal
  const close = useCallback(() => setOpen(false), []);

  return (
    <div className={`inline-flex ${className}`}>
      {/* Single shell — one ss-btn, one border, one pill */}
      <div
        ref={shellRef}
        className={`
          ss-btn
          inline-flex items-center
          rounded-[var(--radius-btn)] overflow-hidden
          transition-all duration-150
          active:scale-[0.98]
          ${variantShell[variant]}
          ${isDisabled ? 'opacity-50 pointer-events-none' : ''}
        `}
      >
        {/* Main action */}
        <button
          className={`
            inline-flex items-center justify-center gap-2
            font-medium cursor-pointer
            transition-colors duration-150
            focus-visible:ring-2 focus-visible:ring-pencil/20 focus-visible:ring-offset-2
            ${variantHover[variant]}
            ${sizeMain[size]}
          `}
          onClick={onClick}
          disabled={isDisabled}
        >
          {loading && <Spinner size="sm" className="text-current" />}
          {children}
        </button>

        {/* Divider */}
        <div className={`w-px self-stretch ${variantDivider[variant]}`} />

        {/* Chevron */}
        <button
          className={`
            inline-flex items-center justify-center self-stretch
            cursor-pointer
            transition-colors duration-150
            focus-visible:ring-2 focus-visible:ring-pencil/20 focus-visible:ring-offset-2
            ${variantHover[variant]}
            ${sizeChevron[size]}
          `}
          onClick={() => setOpen((v) => !v)}
          disabled={isDisabled}
          aria-haspopup="true"
          aria-expanded={open}
          aria-label="More options"
        >
          <ChevronDown
            size={size === 'lg' ? 18 : 14}
            strokeWidth={2.5}
            className={`transition-transform duration-150 ${open ? 'rotate-180' : ''}`}
          />
        </button>
      </div>

      {/* Portal dropdown — escapes any overflow-hidden parent */}
      {open && (
        <DropdownPortal
          anchorRef={shellRef}
          align={dropdownAlign}
          variant={variant}
          size={size}
          items={items}
          onClose={close}
        />
      )}
    </div>
  );
}
