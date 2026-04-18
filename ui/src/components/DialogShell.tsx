import { useEffect, type ReactNode, type RefObject } from 'react';
import { createPortal } from 'react-dom';
import { useFocusTrap } from '../hooks/useFocusTrap';

const maxWidthClass = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-xl',
  '2xl': 'max-w-2xl',
  '3xl': 'max-w-3xl',
  '4xl': 'max-w-4xl',
  '5xl': 'max-w-5xl',
  '6xl': 'max-w-6xl',
  '7xl': 'max-w-7xl',
} as const;

const paddingClass = {
  none: '',
  sm: 'p-3',
  md: 'p-4',
  lg: 'p-6',
} as const;

interface DialogShellProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  maxWidth?: keyof typeof maxWidthClass;
  padding?: keyof typeof paddingClass;
  /** Prevent close on Escape / backdrop click (e.g. during loading) */
  preventClose?: boolean;
  className?: string;
}

export default function DialogShell({
  open,
  onClose,
  children,
  maxWidth = 'lg',
  padding = 'lg',
  preventClose = false,
  className = '',
}: DialogShellProps) {
  const trapRef = useFocusTrap(open);

  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !preventClose) onClose();
    };
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [open, preventClose, onClose]);

  if (!open) return null;

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !preventClose) onClose();
      }}
    >
      {/* Backdrop */}
      <div className="ss-dialog-backdrop absolute inset-0 bg-pencil/30 backdrop-blur-[2px]" />

      {/* Content */}
      <div
        ref={trapRef as RefObject<HTMLDivElement>}
        className={`ss-dialog relative w-full ${maxWidthClass[maxWidth]} bg-surface border-2 border-pencil ${paddingClass[padding]} animate-dialog-in rounded-[var(--radius-md)] ${className}`}
      >
        {children}
      </div>
    </div>,
    document.body,
  );
}
