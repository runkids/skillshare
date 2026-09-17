import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ShieldCheck, Trash2, X } from 'lucide-react';
import { mcpApi } from '../../api/mcp';
import Badge from '../Badge';
import Button from '../Button';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import Spinner from '../Spinner';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import { describeMessage, statusVariant } from './mcpView';

interface Props {
  name: string;
  targets: string[];
  onClose: () => void;
  onSaved: (backups: string[]) => void;
}

export default function MCPRemoveDialog({ name, targets, onClose, onSaved }: Props) {
  const t = useT();
  const { data: plan, error, isPending } = useQuery({ queryKey: ['mcp-remove-preview', name], queryFn: () => mcpApi.preview({ name, remove: true }), gcTime: 0 });
  const [busy, setBusy] = useState(false);
  const [saveError, setSaveError] = useState('');
  const changes = plan?.changes.filter(change => change.name === name) ?? [];
  const untouched = targets.filter(target => !changes.some(change => change.target === target));

  const save = async (sync: boolean) => {
    if (!plan) return;
    setBusy(true); setSaveError('');
    try { onSaved((await mcpApi.configure({ name, remove: true }, plan.revision, sync)).backupIds ?? []); }
    catch (e) { setSaveError(e instanceof Error ? e.message : t('common.error.generic')); }
    finally { setBusy(false); }
  };

  return <DialogShell open onClose={onClose} preventClose={busy} maxWidth="lg" ariaLabel={t('mcp.removeTitle', { name })}>
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div className="w-11 h-11 rounded-full bg-danger-light text-danger flex items-center justify-center"><Trash2 size={20} aria-hidden="true" /></div>
        <IconButton icon={<X size={16} strokeWidth={2.5} />} label={t('common.close')} disabled={busy} onClick={onClose} />
      </div>
      <div className="space-y-1">
        <h2 className="text-xl font-semibold">{t('mcp.removeTitle', { name })}</h2>
        <p className="text-sm text-pencil-light">{t('mcp.removeDesc')}</p>
      </div>
      {error || saveError ? <p role="alert" className="text-sm text-danger">{error?.message ?? saveError}</p> : null}
      {isPending ? <Spinner /> : <ul className="border border-muted rounded-[var(--radius-md)] divide-y divide-dashed divide-pencil-light/30">
        {changes.map(change => <li key={change.target} className="flex flex-wrap items-center gap-3 px-3 py-2.5">
          <Badge size="md" variant={statusVariant[change.action]}>{t(`mcp.status.${change.action}`)}</Badge>
          <span className="w-24 inline-flex items-center gap-1.5 font-semibold"><AgentIcon target={change.target} />{change.target}</span>
          {change.message
            ? <span className="text-sm text-pencil-light">{describeMessage(t, change.message)}</span>
            : <span className="font-mono text-xs text-pencil-light break-all">{change.path}</span>}
        </li>)}
        {untouched.map(target => <li key={target} className="flex flex-wrap items-center gap-3 px-3 py-2.5 text-pencil-light">
          <span aria-hidden="true" className="w-12 text-center">—</span>
          <span className="w-24 inline-flex items-center gap-1.5 font-semibold"><AgentIcon target={target} />{target}</span>
          <span className="text-xs">{t('mcp.notWritten')}</span>
        </li>)}
      </ul>}
      {plan?.blocked ? <p className="text-sm text-warning">{t('mcp.conflictHint')}</p> : null}
      <p className="flex items-center gap-1.5 text-xs text-pencil-light"><ShieldCheck size={14} className="text-success" aria-hidden="true" />{t('mcp.backupNote')}</p>
      <div className="flex flex-wrap justify-end gap-2 pt-4 border-t border-dashed border-pencil-light/30">
        <Button variant="ghost" disabled={busy} onClick={onClose}>{t('common.cancel')}</Button>
        <Button variant="secondary" disabled={!plan} loading={busy} onClick={() => save(false)}>{t('mcp.removeSourceOnly')}</Button>
        <Button variant="danger" disabled={!plan || plan.blocked} loading={busy} onClick={() => save(true)}>{t('mcp.removeSync')}</Button>
      </div>
    </div>
  </DialogShell>;
}
