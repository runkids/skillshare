import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Info, X } from 'lucide-react';
import { mcpApi } from '../../api/mcp';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { describeMessage } from './mcpView';

interface Props {
  name: string;
  onClose: () => void;
  onSaved: () => void;
}

export default function MCPRemoveDialog({ name, onClose, onSaved }: Props) {
  const t = useT();
  const { data: plan, error, isPending } = useQuery({ queryKey: ['mcp-remove-preview', name], queryFn: () => mcpApi.preview({ name, remove: true }), gcTime: 0 });
  const [busy, setBusy] = useState(false);
  const [saveError, setSaveError] = useState('');
  const changes = plan?.changes.filter((c) => c.name === name) ?? [];
  const title = t('mcp.removeTitle', { name });

  const save = async (sync: boolean) => {
    if (!plan) return;
    setBusy(true);
    setSaveError('');
    try {
      await mcpApi.configure({ name, remove: true }, plan.revision, sync);
      onSaved();
    } catch (e) {
      setSaveError((e as Error).message);
      setBusy(false);
    }
  };

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[460px]">
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <div className="db">
        <p className="text-[13px]">{t('mcp.removeDesc')}</p>
        {isPending ? (
          <Spinner size="sm" />
        ) : changes.length > 0 ? (
          <div className="ss-list !shadow-none">
            {changes.map((c) => (
              <div key={c.path} className="ss-r !min-h-10">
                <span className="ss-at"><AgentIcon target={c.target} size={17} /></span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate font-mono text-[13px]" title={c.path}>{shortenHome(c.path)}</span>
                  {c.message && <span className="text-xs text-warn">{describeMessage(t, c.message)}</span>}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-[13px] text-ink-3">{t('mcp.notWritten')}</p>
        )}
        {(error || saveError) && <div className="ss-note bad" role="alert"><span className="flex-1">{error?.message ?? saveError}</span></div>}
        {plan?.blocked ? (
          <div className="ss-note warn"><Info size={16} /><span className="flex-1">{t('mcp.removeBlocked')}</span></div>
        ) : (
          <div className="ss-note inf"><Info size={16} /><span className="flex-1">{t('mcp.backupNote')}</span></div>
        )}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
        <span className="flex-1" />
        <Button variant="secondary" disabled={!plan} loading={busy} onClick={() => save(false)}>{t('mcp.removeSourceOnly')}</Button>
        <Button variant="primary" disabled={!plan || plan.blocked} loading={busy} onClick={() => save(true)}>{t('mcp.removeSync')}</Button>
      </div>
    </DialogShell>
  );
}
