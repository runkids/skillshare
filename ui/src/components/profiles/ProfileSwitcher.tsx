import { SwatchBook } from 'lucide-react';
import type { ProfilesResponse } from '../../api/client';
import { useT } from '../../i18n';
import Select from '../Select';
import { profileOptions, profileVisible } from './profileView';

export default function ProfileSwitcher({ data, onChoose }: { data?: ProfilesResponse; onChoose: (name: string | null) => void }) {
  const t = useT();
  if (!data || !profileVisible(data)) return null;
  const options = profileOptions(data, t);
  return <div className="flex items-center gap-1 min-w-0 text-xs text-ink-3">
    <SwatchBook size={14} className="shrink-0" />
    <Select quiet size="sm" value={data.active} options={options} ariaLabel={t('profiles.choose')} onChange={(value) => { if (value !== data.active) onChoose(value || null); }} />
    {(data.pending || data.state === 'unavailable') && <span className={`ss-st ${data.state === 'unavailable' ? 'bad' : 'warn'}`} title={t(data.state === 'unavailable' ? 'profiles.unavailable' : 'profiles.pending')} />}
  </div>;
}
