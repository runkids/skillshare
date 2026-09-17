import { useState } from 'react';
import { Trash2, X } from 'lucide-react';
import { api, type Target } from '../../api/client';
import { shortenHome } from '../../lib/paths';
import { useT } from '../../i18n';
import Button from '../Button';
import DialogShell from '../DialogShell';

/** What removal does mirrors handleRemoveTarget: merge links become copies, a whole-folder link is deleted, copies stay. */
export default function RemoveTargetDialog({ target, onClose, onRemoved }: { target: Target; onClose: () => void; onRemoved: () => void }) {
  const t = useT();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const title = t('targets.remove.title', { name: target.name });
  const remove = async () => {
    setBusy(true);
    setError('');
    try {
      await api.removeTarget(target.name);
      onRemoved();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  };
  const effect = target.mode === 'symlink'
    ? t('targets.remove.symlink', { path: shortenHome(target.path) })
    : target.mode === 'copy'
      ? t('targets.remove.copy')
      : t(target.linkedCount === 1 ? 'targets.remove.merge.one' : 'targets.remove.merge.other', { count: target.linkedCount, name: target.name });
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[460px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t('targets.remove.subtitle')}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <div className="db !gap-2.5 text-[13.5px] leading-relaxed">
        <p>{effect}</p>
        <p>{t('targets.remove.untouched')}</p>
        {error && <div className="ss-note bad"><span className="flex-1">{error}</span></div>}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
        <Button variant="secondary" onClick={remove} loading={busy}>{!busy && <Trash2 size={15} />}{t('targets.remove.confirm')}</Button>
      </div>
    </DialogShell>
  );
}
