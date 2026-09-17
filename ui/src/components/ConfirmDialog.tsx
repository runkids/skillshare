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
    <DialogShell open={open} onClose={onCancel} maxWidth={wide ? '2xl' : 'md'} padding="none" preventClose={loading} ariaLabel={title}>
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
      </div>
      <div className="db text-ink-2 overflow-y-auto">{message}</div>
      <div className="df">
        {resolvedCancelText && (
          <Button variant="ghost" onClick={onCancel} disabled={loading}>
            {resolvedCancelText}
          </Button>
        )}
        <Button variant={variant === 'danger' ? 'danger' : 'primary'} onClick={onConfirm} loading={loading}>
          {resolvedConfirmText}
        </Button>
      </div>
    </DialogShell>
  );
}
