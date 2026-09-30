import { PageSkeleton } from '../components/Skeleton';
import PageHeader from '../components/PageHeader';
import HooksScope from '../components/hooks/HooksScope';
import { useHooksQuery } from '../hooks/useSharedQueries';
import { useT } from '../i18n';

export default function HooksPage() {
  const t = useT();
  const { data, error, isPending } = useHooksQuery();
  if (isPending) return <PageSkeleton />;
  return (
    <div className="animate-fade-in">
      {error && <div className="ss-note bad mb-4"><span className="flex-1">{error.message}</span></div>}
      {data ? (
        <HooksScope data={data} header={(actions) => <PageHeader className="[&_.ss-ph]:flex-wrap" title={t('hooks.title')} subtitle={t('hooks.subtitle')} actions={actions} />} />
      ) : (
        <PageHeader title={t('hooks.title')} subtitle={t('hooks.subtitle')} />
      )}
    </div>
  );
}
