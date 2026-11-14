import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import { useT } from '../../i18n';
import { invalidate } from '../../lib/queryEvents';
import { queryKeys } from '../../lib/queryKeys';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { useToast } from '../Toast';
import { profileReason } from './profileView';

/** Receives resolved source flat names from either selection owner, never folders or agents. */
export default function ProfileMembershipMenu({ names }: { names: string[] }) {
  const t = useT();
  const client = useQueryClient();
  const { toast } = useToast();
  const [operation, setOperation] = useState<'add' | 'remove' | null>(null);
  const [busy, setBusy] = useState(false);
  const profiles = useQuery({ queryKey: queryKeys.profiles, queryFn: () => api.profiles.list(), enabled: operation !== null });
  const assign = async (name: string) => {
    if (!operation) return;
    const body = { [operation]: [...new Set(names)] };
    setOperation(null);
    setBusy(true);
    try {
      const result = await api.profiles.assign(name, body);
      toast(result.skills?.map((row) => `${row.name}: ${t(`profileEdit.result.${row.result}`)}`).join('\n') || t('profileEdit.membershipSaved', { name }), 'success');
      await invalidate(client, 'profilesChanged');
    } catch (cause: unknown) {
      toast(profileReason(cause instanceof Error ? cause.message : String(cause), t, name), 'error');
    } finally { setBusy(false); }
  };
  return <>
    <Button variant="secondary" size="sm" disabled={busy || names.length === 0} onClick={() => setOperation('add')}>{t('profileEdit.add')}</Button>
    <Button variant="secondary" size="sm" disabled={busy || names.length === 0} onClick={() => setOperation('remove')}>{t('profileEdit.remove')}</Button>
    <DialogShell open={operation !== null} onClose={() => setOperation(null)} ariaLabel={t(operation === 'remove' ? 'profileEdit.remove' : 'profileEdit.add')} maxWidth="sm">
      <div className="flex flex-col gap-4">
        <h2 className="ss-h2">{t(operation === 'remove' ? 'profileEdit.remove' : 'profileEdit.add')}</h2>
        {profiles.isPending ? <p>{t('profileEdit.loading')}</p> : profiles.error ? <div className="ss-note bad">{profiles.error.message}</div> : <div role="menu" className="ss-menu !static !w-full max-h-80 overflow-y-auto">
          {[...(profiles.data?.profiles ?? [])].sort((a, b) => a.name.localeCompare(b.name)).map((profile) => <button
            key={profile.name} type="button" role="menuitem" aria-label={profile.name} disabled={!profile.usable}
            className="!h-auto flex-col !items-start gap-1 py-2 disabled:opacity-50" onClick={() => void assign(profile.name)}
          >
            <span className="font-mono">{profile.name}</span>
            {!profile.usable && <span className="text-xs font-normal text-ink-3">{profileReason(profile.reason ?? '', t, profile.name)}</span>}
          </button>)}
        </div>}
        {profiles.data?.profiles.length === 0 && <Link className="ss-btn" to="/profiles" onClick={() => setOperation(null)}>{t('profileEdit.create')}</Link>}
        <Button variant="secondary" onClick={() => setOperation(null)}>{t('common.cancel')}</Button>
      </div>
    </DialogShell>
  </>;
}
