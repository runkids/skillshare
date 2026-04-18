import { useState, useEffect } from 'react';
import { Plus, Trash2, Server } from 'lucide-react';
import Button from './Button';
import IconButton from './IconButton';
import DialogShell from './DialogShell';
import { Input } from './Input';
import { radius } from '../design';
import { useT } from '../i18n';

export interface SavedHub {
  label: string;
  url: string;
  builtIn?: boolean;
}

interface HubManagerModalProps {
  open: boolean;
  hubs: SavedHub[];
  onSave: (hubs: SavedHub[]) => void;
  onClose: () => void;
}

function normalizeURL(url: string): string {
  return url.trim().replace(/\/+$/, '');
}

export default function HubManagerModal({
  open,
  hubs,
  onSave,
  onClose,
}: HubManagerModalProps) {
  const t = useT();
  const [localHubs, setLocalHubs] = useState<SavedHub[]>([]);
  const [newLabel, setNewLabel] = useState('');
  const [newURL, setNewURL] = useState('');
  const [error, setError] = useState('');
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  // Sync local state when modal opens
  useEffect(() => {
    if (open) {
      setLocalHubs([...hubs]);
      setNewLabel('');
      setNewURL('');
      setError('');
      setConfirmDelete(null);
    }
  }, [open, hubs]);

  // Close confirmDelete on Escape before closing modal
  useEffect(() => {
    if (!open || !confirmDelete) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        setConfirmDelete(null);
      }
    };
    // Use capture to intercept before DialogShell's handler
    document.addEventListener('keydown', handleKey, true);
    return () => document.removeEventListener('keydown', handleKey, true);
  }, [open, confirmDelete]);

  const handleAdd = () => {
    const url = normalizeURL(newURL);
    if (!url) {
      setError(t('hubManager.error.urlRequired'));
      return;
    }
    if (localHubs.some((h) => normalizeURL(h.url) === url)) {
      setError(t('hubManager.error.urlExists'));
      return;
    }
    const label = newLabel.trim() || url.split('/').pop() || 'Untitled';
    const updated = [...localHubs, { label, url }];
    setLocalHubs(updated);
    onSave(updated);
    setNewLabel('');
    setNewURL('');
    setError('');
  };

  const handleDelete = (url: string) => {
    if (confirmDelete !== url) {
      setConfirmDelete(url);
      return;
    }
    const updated = localHubs.filter((h) => normalizeURL(h.url) !== normalizeURL(url));
    setLocalHubs(updated);
    onSave(updated);
    setConfirmDelete(null);
  };

  return (
    <DialogShell open={open} onClose={onClose} maxWidth="md">
          {/* Header */}
          <div className="flex items-center gap-2 mb-4">
            <Server size={18} strokeWidth={2.5} className="text-pencil-light" />
            <h3 className="text-xl font-bold text-pencil">
              {t('hubManager.title')}
            </h3>
          </div>

          {/* Hub list */}
          {localHubs.length > 0 ? (
            <div className="space-y-2 mb-4 max-h-48 overflow-y-auto">
              {localHubs.map((hub) => (
                <div
                  key={hub.url}
                  className="flex items-center gap-2 py-2 px-3 border-2 border-muted bg-paper-warm"
                  style={{ borderRadius: radius.sm }}
                >
                  <div className="flex-1 min-w-0">
                    <span className="font-bold text-pencil text-base block">
                      {hub.label}
                    </span>
                    <span className="text-xs text-muted-dark block truncate font-mono">
                      {hub.url}
                    </span>
                  </div>
                  {hub.builtIn ? (
                    <span className="text-xs text-muted-dark shrink-0 px-1.5">{t('hubManager.builtIn')}</span>
                  ) : confirmDelete === hub.url ? (
                    <div className="flex items-center gap-1 shrink-0">
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => handleDelete(hub.url)}
                      >
                        {t('hubManager.confirmDelete')}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setConfirmDelete(null)}
                      >
                        {t('hubManager.cancelDelete')}
                      </Button>
                    </div>
                  ) : (
                    <IconButton
                      icon={<Trash2 size={16} strokeWidth={2.5} />}
                      label={t('hubManager.removeHub')}
                      size="sm"
                      variant="ghost"
                      onClick={() => handleDelete(hub.url)}
                      className="hover:text-danger"
                    />
                  )}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-base text-muted-dark mb-4 text-center py-4">
              {t('hubManager.noHubs')}
            </p>
          )}

          {/* Add hub form */}
          <div className="border-t border-dashed border-pencil-light/30 pt-3">
            <p className="text-base font-bold text-pencil mb-2">
              {t('hubManager.addHub')}
            </p>
            <div className="space-y-2 mb-2">
              <Input
                type="text"
                placeholder={t('hubManager.labelPlaceholder')}
                value={newLabel}
                onChange={(e) => setNewLabel(e.target.value)}
              />
              <Input
                type="text"
                placeholder={t('hubManager.urlPlaceholder')}
                value={newURL}
                onChange={(e) => {
                  setNewURL(e.target.value);
                  setError('');
                }}
                onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
              />
            </div>
            {error && (
              <p className="text-sm text-danger mb-2">{error}</p>
            )}
            <p className="text-xs text-muted-dark mb-3">
              {t('hubManager.hubHint')}
            </p>
            <div className="flex justify-between">
              <Button variant="primary" size="sm" onClick={handleAdd}>
                <Plus size={14} strokeWidth={2.5} />
                {t('hubManager.addButton')}
              </Button>
              <Button variant="ghost" size="sm" onClick={onClose}>
                {t('hubManager.closeButton')}
              </Button>
            </div>
          </div>
    </DialogShell>
  );
}
