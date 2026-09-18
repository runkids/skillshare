import { useSearchParams } from 'react-router-dom';
import HubBrowser from '../components/hub/HubBrowser';
import HubBuilder from '../components/hub/HubBuilder';
import PageHeader from '../components/PageHeader';
import { useT } from '../i18n';

/**
 * Both halves of hubs in one place: the ones you subscribe to and the ones you build.
 * Publishing a draft ends in the Browse tab, so you can see what a recipient sees.
 */
export default function HubPage() {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') === 'mine' ? 'mine' : 'browse';
  const preview = params.get('preview');

  const go = (next: 'browse' | 'mine') =>
    setParams((p) => {
      if (next === 'mine') p.set('tab', 'mine');
      else p.delete('tab');
      return p;
    }, { replace: true });

  return (
    <div className="ss-wrap animate-fade-in">
      <PageHeader title={t('hubs.title')} subtitle={t('hubs.subtitle')} backTo="/skills" />
      <nav className="ss-tabs" aria-label={t('hubs.title')}>
        <button type="button" className={tab === 'browse' ? 'on' : ''} aria-current={tab === 'browse'} onClick={() => go('browse')}>
          {t('hubs.tabs.browse')}
        </button>
        <button type="button" className={tab === 'mine' ? 'on' : ''} aria-current={tab === 'mine'} onClick={() => go('mine')}>
          {t('hubs.tabs.mine')}
        </button>
      </nav>
      {tab === 'browse' ? <HubBrowser previewURL={preview ?? undefined} /> : <HubBuilder />}
    </div>
  );
}
