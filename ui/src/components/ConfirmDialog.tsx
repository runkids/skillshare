import { useEffect, type ReactNode } from 'react';
import Card from './Card';
import HandButton from './HandButton';
import { wobbly } from '../design';

interface ConfirmDialogProps {
  open: boolean;
  onConfirm: () => void;
  onCancel: () => void;
  title: string;
  message: ReactNode;
  confirmText?: string;
  cancelText?: string;
  variant?: 'default' | 'danger';
  loading?: boolean;
}

export default function ConfirmDialog({
  open,
  onConfirm,
  onCancel,
  title,
  message,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  variant = 'default',
  loading = false,
}: ConfirmDialogProps) {
  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !loading) onCancel();
    };
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [open, loading, onCancel]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget && !loading) onCancel();
      }}
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-pencil/30" />

      {/* Dialog */}
      <div
        className="relative w-full max-w-sm animate-sketch-in"
        style={{ borderRadius: wobbly.md }}
      >
        <Card decoration="tape" className="text-center">
          <h3
            className="text-xl font-bold text-pencil mb-2"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            {title}
          </h3>
          <div
            className="text-pencil-light mb-6"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            {message}
          </div>
          <div className="flex gap-3 justify-center">
            <HandButton
              variant="ghost"
              size="sm"
              onClick={onCancel}
              disabled={loading}
            >
              {cancelText}
            </HandButton>
            <HandButton
              variant={variant === 'danger' ? 'danger' : 'primary'}
              size="sm"
              onClick={onConfirm}
              disabled={loading}
            >
              {loading ? 'Working...' : confirmText}
            </HandButton>
          </div>
        </Card>
      </div>
    </div>
  );
}
