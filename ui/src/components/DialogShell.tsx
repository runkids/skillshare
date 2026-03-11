import { useEffect, type ReactNode, type RefObject } from 'react';
import { radius } from '../design';
import { useFocusTrap } from '../hooks/useFocusTrap';

const maxWidthClass = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-xl',
  '2xl': 'max-w-2xl',
  '3xl': 'max-w-3xl',
} as const;

interface DialogShellProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  maxWidth?: keyof typeof maxWidthClass;
  /** Prevent close on Escape / backdrop click (e.g. during loading) */
  preventClose?: boolean;
  className?: string;
}

export default function DialogShell({
  open,
  onClose,
  children,
  maxWidth = 'lg',
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

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      onClick={(e) => {
        if (e.target === e.currentTarget && !preventClose) onClose();
      }}
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-pencil/30" />

      {/* Content */}
      <div
        ref={trapRef as RefObject<HTMLDivElement>}
        className={`relative w-full ${maxWidthClass[maxWidth]} animate-fade-in ${className}`}
        style={{ borderRadius: radius.md }}
      >
        {children}
      </div>
    </div>
  );
}
