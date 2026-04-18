import type { ReactNode } from 'react';
import Button from './Button';
import DialogShell from './DialogShell';
import { useT } from '../i18n';

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
  wide?: boolean;
}

export default function ConfirmDialog({
  open,
  onConfirm,
  onCancel,
  title,
  message,
  confirmText,
  cancelText,
  variant = 'default',
  loading = false,
  wide = false,
}: ConfirmDialogProps) {
  const t = useT();
  const resolvedCancelText = cancelText ?? t('common.cancel');
  const resolvedConfirmText = confirmText ?? t('common.confirm');
  return (
    <DialogShell
      open={open}
      onClose={onCancel}
      maxWidth={wide ? '2xl' : 'lg'}
      preventClose={loading}
    >
        <h3 className="text-lg font-bold text-pencil mb-2">
          {title}
        </h3>
        <div className="text-pencil-light mb-6">
          {message}
        </div>
        <div className="flex gap-3 justify-end">
          {resolvedCancelText && (
            <Button
              variant="secondary"
              size="md"
              onClick={onCancel}
              disabled={loading}
            >
              {resolvedCancelText}
            </Button>
          )}
          <Button
            variant={variant === 'danger' ? 'danger' : 'primary'}
            size="md"
            onClick={onConfirm}
            loading={loading}
          >
            {resolvedConfirmText}
          </Button>
        </div>
    </DialogShell>
  );
}
