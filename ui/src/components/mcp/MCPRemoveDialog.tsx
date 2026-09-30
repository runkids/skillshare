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
import type { MCPChange } from './mcpView';

interface Props {
  name: string;
  /** A root under mcp.projects, when the server is one of that project's. */
  project?: string;
  /** Changes of this scope. One name can be in the global source and in projects, and only one of them goes. */
  inScope?: (change: MCPChange) => boolean;
  onClose: () => void;
  /** `unmanaged` when the entries were kept and only forgotten. */
  onSaved: (unmanaged: boolean) => void;
}

export default function MCPRemoveDialog({ name, project, inScope = () => true, onClose, onSaved }: Props) {
  const t = useT();
  const { data: plan, error, isPending } = useQuery({ queryKey: ['mcp-remove-preview', project, name], queryFn: () => mcpApi.preview({ project, name, remove: true }), gcTime: 0 });
  const [busy, setBusy] = useState(false);
  const [saveError, setSaveError] = useState('');
  const changes = plan?.changes.filter((c) => c.name === name && inScope(c)) ?? [];
  const title = t('mcp.removeTitle', { name });

  // unmanage keeps the Agent entries and forgets them, so it never syncs.
  const save = async (sync: boolean, unmanage = false) => {
    if (!plan) return;
    setBusy(true);
    setSaveError('');
    try {
      const mutation = { project, name, remove: true, ...(unmanage && { unmanage }) };
      // Stopping managing plans nothing for the target files, so it has its own revision rather than the removal preview's.
      const revision = unmanage ? (await mcpApi.preview(mutation)).revision : plan.revision;
      await mcpApi.configure(mutation, revision, sync);
      onSaved(unmanage);
    } catch (e) {
      setSaveError((e as Error).message);
      setBusy(false);
    }
  };

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[600px]">
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
        <Button variant="secondary" disabled={!plan} loading={busy} onClick={() => save(false, true)}>{t('mcp.removeUnmanage')}</Button>
        <Button variant="secondary" disabled={!plan} loading={busy} onClick={() => save(false)}>{t('mcp.removeSourceOnly')}</Button>
        <Button variant="primary" disabled={!plan || plan.blocked} loading={busy} onClick={() => save(true)}>{t('mcp.removeSync')}</Button>
      </div>
    </DialogShell>
  );
}
