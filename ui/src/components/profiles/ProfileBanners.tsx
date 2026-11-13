import { useState } from 'react';
import type { ProfilesResponse } from '../../api/client';
import { useT } from '../../i18n';
import Button from '../Button';
import SyncPreviewModal from '../SyncPreviewModal';
import ProfileSwitcher from './ProfileSwitcher';

export default function ProfileBanners({ data, onChoose }: { data?: ProfilesResponse; onChoose: (name: string | null) => void }) {
  const t = useT();
  const [syncOpen, setSyncOpen] = useState(false);
  const [choosing, setChoosing] = useState(false);
  return <>
    {data?.state === 'unavailable' ? <div className="ss-note bad mb-6 flex-wrap">
      <span className="flex-1">{t('profiles.unavailable')} <code>{data.active}</code></span>
      <Button variant="ghost" size="sm" onClick={() => setChoosing((v) => !v)}>{t('profiles.choose')}</Button>
      <Button variant="ghost" size="sm" onClick={() => onChoose(null)}>{t('profiles.all')}</Button>
      {choosing && <ProfileSwitcher data={data} onChoose={onChoose} />}
    </div> : data?.pending && <div className="ss-note warn mb-6">
      <span className="flex-1">{t('profiles.pending')}</span>
      <Button variant="ghost" size="sm" onClick={() => setSyncOpen(true)}>{t('profiles.syncNow')}</Button>
    </div>}
    {syncOpen && <SyncPreviewModal open onClose={() => setSyncOpen(false)} kind="skill" />}
  </>;
}
