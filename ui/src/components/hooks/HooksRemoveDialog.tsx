import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Info, X } from 'lucide-react';
import { hooksApi } from '../../api/hooks';
import { useT } from '../../i18n';
import Button from '../Button';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import HooksPreview from './HooksPreview';
import { rootPlan } from './hooksView';

interface Props { name: string; project?: string; onClose: () => void; onSaved: () => void }

/** Removing prunes only the outputs Skillshare owns and that are still unchanged; the preview shows which. */
export default function HooksRemoveDialog({ name, project, onClose, onSaved }: Props) {
  const t = useT();
  const { data: plan, error, isPending } = useQuery({ queryKey: ['hooks-remove-preview', project, name], queryFn: () => hooksApi.preview({ project, name, remove: true }), gcTime: 0, retry: false });
  const [busy, setBusy] = useState(false);
  const [saveError, setSaveError] = useState('');
  const title = t('hooks.removeTitle', { name });
  // Sync writes the whole scope (a project's root, or every file globally), so the preview shows and is blocked by all of it; the revision stays the full plan's.
  const shown = plan && rootPlan(plan, project);
  const blocked = shown?.blocked;

  const save = async (sync: boolean) => {
    if (!plan) return;
    setBusy(true);
    setSaveError('');
    try {
      // A project mutation syncs only its own root on the server, so one call serves both scopes.
      await hooksApi.configure({ project, name, remove: true }, plan.revision, sync);
      onSaved();
    } catch (e) {
      setSaveError((e as Error).message);
      setBusy(false);
    }
  };

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[640px]">
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <div className="db">
        <p className="text-[13px]">{t('hooks.removeDesc', { name })}</p>
        {isPending ? <Spinner size="sm" /> : shown && <HooksPreview plan={shown} />}
        {(error || saveError) && <div className="ss-note bad" role="alert"><span className="flex-1">{error?.message ?? saveError}</span></div>}
        <div className={`ss-note ${blocked ? 'warn' : 'inf'}`}><Info size={16} /><span className="flex-1">{blocked ? t('hooks.removeBlocked') : t('mcp.backupNote')}</span></div>
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
        <span className="flex-1" />
        <Button variant="secondary" disabled={!plan} loading={busy} onClick={() => void save(false)}>{t('mcp.removeSourceOnly')}</Button>
        <Button variant="primary" disabled={!plan || blocked} loading={busy} onClick={() => void save(true)}>{t('mcp.removeSync')}</Button>
      </div>
    </DialogShell>
  );
}
