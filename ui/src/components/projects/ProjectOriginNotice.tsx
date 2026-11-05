import type { Project } from '../../api/client';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';

export default function ProjectOriginNotice({ origin, originPath }: Pick<Project, 'origin' | 'originPath'>) {
  const t = useT();
  if (!origin) return null;
  return <>
    <div className="ss-note"><span className="flex-1">{t(`projects.origin.${origin}`)}{originPath && <> · <span className="font-mono" title={originPath}>{shortenHome(originPath)}</span></>}</span></div>
    {origin === 'shared' && <div className="ss-note warn"><span className="flex-1">{t('projects.origin.sharedEdit')}</span></div>}
  </>;
}
