import type { ReactNode } from 'react';
import Card from './Card';
import Button from './Button';
import DialogShell from './DialogShell';

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
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  variant = 'default',
  loading = false,
  wide = false,
}: ConfirmDialogProps) {
  return (
    <DialogShell
      open={open}
      onClose={onCancel}
      maxWidth={wide ? '2xl' : 'lg'}
      preventClose={loading}
    >
      <Card>
        <h3 className="text-lg font-bold text-pencil mb-2">
          {title}
        </h3>
        <div className="text-pencil-light mb-6">
          {message}
        </div>
        <div className="flex gap-3 justify-end">
          {cancelText && (
            <Button
              variant="secondary"
              size="md"
              onClick={onCancel}
              disabled={loading}
            >
              {cancelText}
            </Button>
          )}
          <Button
            variant={variant === 'danger' ? 'danger' : 'primary'}
            size="md"
            onClick={onConfirm}
            loading={loading}
          >
            {confirmText}
          </Button>
        </div>
      </Card>
    </DialogShell>
  );
}
