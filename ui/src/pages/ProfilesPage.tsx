import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Plus, SwatchBook } from 'lucide-react';
import { api } from '../api/client';
import { useT } from '../i18n';
import { queryKeys } from '../lib/queryKeys';
import Button from '../components/Button';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import ProfileEditor from '../components/profiles/ProfileEditor';
import { profileReason } from '../components/profiles/profileView';

export default function ProfilesPage() {
  const t = useT();
  const navigate = useNavigate();
  const { name } = useParams();
  const [creating, setCreating] = useState(false);
  const [creatingBusy, setCreatingBusy] = useState(false);
  const { data, isPending, error } = useQuery({ queryKey: queryKeys.profiles, queryFn: () => api.profiles.list() });
  const profile = data?.profiles.find((p) => p.name === name);
  const create = <Button onClick={() => setCreating(true)}><Plus size={15} />{t('profileEdit.create')}</Button>;
  return <div className="ss-wrap animate-page-in">
    <PageHeader title={name ?? t('profileEdit.title')} subtitle={name ? profile?.description : t('profileEdit.subtitle')} backTo={name ? '/profiles' : undefined} actions={!name && (data?.profiles.length ?? 0) > 0 ? create : undefined} mono={!!name} />
    {isPending ? <PageSkeleton /> : error ? <div className="ss-note bad">{error.message}</div> : name ? (
      profile ? <ProfileEditor key={profile.name} profile={profile} active={data?.active === name} /> : <div className="ss-note bad">{t('profileEdit.missing', { name })}</div>
    ) : data?.profiles.length === 0 ? (
      <EmptyState icon={SwatchBook} title={t('profileEdit.empty')} description={t('profileEdit.subtitle')} action={create} />
    ) : <div className="ss-list">
      <div className="ss-lh"><span className="flex-1">{t('profileEdit.name')} / {t('profileEdit.description')}</span><span>{t('profileEdit.selected')}</span></div>
      {[...(data?.profiles ?? [])].sort((a, b) => a.name.localeCompare(b.name)).map((p) => <div key={p.name} className="ss-r flex-wrap gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <Link className="font-mono font-semibold hover:underline" to={`/profiles/${encodeURIComponent(p.name)}`} aria-label={p.name}>{p.name}</Link>
            {data?.active === p.name && <span className="ss-tag">{t('profileEdit.active')}</span>}
          </div>
          <p className="text-sm text-ink-3">{p.usable ? p.description : profileReason(p.reason ?? '', t, p.name)}</p>
        </div>
        <span className="ss-tag">{t('profiles.count', { count: p.selected })}</span>
        {p.usable && p.skills === null && <span className="text-xs text-ink-3">{t('profileEdit.passThrough')}</span>}
      </div>)}
    </div>}
    <DialogShell open={creating} preventClose={creatingBusy} onClose={() => setCreating(false)} ariaLabel={t('profileEdit.create')} maxWidth="2xl">
      <div className="flex flex-col gap-5"><h2 className="ss-h2">{t('profileEdit.create')}</h2><ProfileEditor onBusyChange={setCreatingBusy} onCreated={(created) => { setCreating(false); navigate(`/profiles/${encodeURIComponent(created)}`); }} /></div>
    </DialogShell>
  </div>;
}
