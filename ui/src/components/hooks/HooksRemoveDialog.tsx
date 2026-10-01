import { useId, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Info, X } from 'lucide-react';
import { hooksApi } from '../../api/hooks';
import { useI18n } from '../../i18n';
import Button from '../Button';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import Tooltip from '../Tooltip';
import HooksPreview from './HooksPreview';
import { joinList } from '../targets/targetView';
import { hookLabel, rootPlan, writes } from './hooksView';

interface Props { name: string; project?: string; canUnmanage?: boolean; onClose: () => void; onSaved: (unmanaged: boolean) => void }

/** Removing prunes only the outputs Skillshare owns and that are still unchanged; the preview shows which. */
export default function HooksRemoveDialog({ name, project, canUnmanage = true, onClose, onSaved }: Props) {
  const { t, locale } = useI18n();
  const { data: plan, error, isPending } = useQuery({ queryKey: ['hooks-remove-preview', project, name], queryFn: () => hooksApi.preview({ project, name, remove: true }), gcTime: 0, retry: false });
  const [busy, setBusy] = useState(false);
  const [saveError, setSaveError] = useState('');
  const title = t('hooks.removeTitle', { name });
  // Sync writes the whole scope (a project's root, or every file globally), so the preview shows and is blocked by all of it; the revision stays the full plan's.
  const shown = plan && rootPlan(plan, project);
  const blocked = shown?.blocked;
  // The targets whose files hold what this hook wrote; each choice says what happens to them.
  const targets = [...new Set(shown?.changes.filter((c) => c.name === name && c.action === 'remove').map((c) => hookLabel(c.target)))];
  // Many targets make each choice line a long list; past three, a count says the same.
  const targetText = targets.length > 3 ? t('hooks.removeTargetCount', { count: targets.length }) : joinList(targets, locale);
  // "Remove and sync" writes the whole scope, so other hooks' pending changes go out with it.
  const others = [...new Set(shown?.changes.filter((c) => c.name !== name && writes(c)).map((c) => c.name))];
  const tipId = useId();

  // unmanage keeps the target entries and forgets them, so it never syncs.
  const save = async (sync: boolean, unmanage = false) => {
    if (!plan) return;
    setBusy(true);
    setSaveError('');
    try {
      // A project mutation syncs only its own root on the server, so one call serves both scopes.
      const mutation = { project, name, remove: true, ...(unmanage && { unmanage }) };
      // Stopping managing plans nothing for the target files, so it has its own revision rather than the removal preview's.
      const revision = unmanage ? (await hooksApi.preview(mutation)).revision : plan.revision;
      await hooksApi.configure(mutation, revision, sync);
      onSaved(unmanage);
    } catch (e) {
      setSaveError((e as Error).message);
      setBusy(false);
    }
  };

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[880px]">
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <div className="db">
        <p className="text-[13px]">{t('hooks.removeDesc', { name })}</p>
        {others.length > 0 && <p className="text-[13px] text-ink-2">{t(others.length === 1 ? 'hooks.removeOthers.one' : 'hooks.removeOthers.other', { count: others.length, names: joinList(others, locale) })}</p>}
        {isPending ? <Spinner size="sm" /> : shown && <HooksPreview plan={shown} />}
        {(error || saveError) && <div className="ss-note bad" role="alert"><span className="flex-1">{error?.message ?? saveError}</span></div>}
        <div className={`ss-note ${blocked ? 'warn' : ''}`}><Info size={16} /><span className="flex-1">{blocked ? t('hooks.removeBlocked') : t('mcp.backupNote')}</span></div>
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
        <span className="flex-1" />
        {([
          ['unmanage', 'mcp.removeUnmanage', 'hooks.removeChoiceUnmanage', !plan, () => save(false, true)],
          ['source', 'mcp.removeSourceOnly', 'hooks.removeChoiceSource', !plan, () => save(false)],
          ['sync', 'mcp.removeSync', 'hooks.removeChoiceSync', !plan || blocked, () => save(true)],
        ] as const).filter(([key]) => key !== 'unmanage' || canUnmanage).map(([key, label, consequence, disabled, run]) => {
          // What each choice does to the target files, on hover and focus; screen readers get it as the button's description.
          // The wrapper stays while the preview loads, so the button is not remounted when the text arrives.
          const note = targets.length > 0 ? t(consequence, { targets: targetText }) : '';
          return (
            <Tooltip key={key} content={note}>
              <Button variant={key === 'sync' ? 'primary' : 'secondary'} disabled={disabled} loading={busy} aria-describedby={note ? `${tipId}-${key}` : undefined} onClick={() => void run()}>{t(label)}</Button>
              {note && <span id={`${tipId}-${key}`} className="sr-only">{note}</span>}
            </Tooltip>
          );
        })}
      </div>
    </DialogShell>
  );
}
