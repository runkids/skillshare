import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, ArrowUpCircle, CheckCircle2, ChevronDown, ChevronRight, Info, Pencil, RefreshCw, XCircle } from 'lucide-react';
import { api, type DoctorCheck } from '../api/client';
import Button from '../components/Button';
import CopyButton from '../components/CopyButton';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { useAppContext } from '../context/AppContext';
import { useT } from '../i18n';
import { shortenHome } from '../lib/paths';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { SettingsTabs } from './SettingsPage';

type Filter = 'all' | 'error' | 'warning' | 'pass';

const FILTERS: Filter[] = ['all', 'error', 'warning', 'pass'];
const ICON = { pass: CheckCircle2, warning: AlertTriangle, error: XCircle, info: Info } as const;
const TONE = { pass: 'text-ok', warning: 'text-warn', error: 'text-bad', info: 'text-accent' } as const;

/** Check names come from the CLI; a missing translation still reads as a phrase. */
const checkLabel = (name: string) => name.replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase());

export default function DoctorPage() {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<Filter>('all');
  const [open, setOpen] = useState<string | null>(null);
  const [upgrading, setUpgrading] = useState(false);
  const [upgradeMessage, setUpgradeMessage] = useState<string | null>(null);

  const overview = useQuery({ queryKey: queryKeys.overview, queryFn: () => api.getOverview(), staleTime: staleTimes.overview });
  const { data, isPending, error, isFetching, refetch } = useQuery({ queryKey: queryKeys.doctor, queryFn: () => api.doctor(), staleTime: staleTimes.doctor });

  const checks = useMemo(() => {
    const all = data?.checks ?? [];
    if (filter === 'all') return all;
    if (filter === 'pass') return all.filter((c) => c.status === 'pass' || c.status === 'info');
    return all.filter((c) => c.status === filter);
  }, [data, filter]);

  const report = useMemo(
    () => (data?.checks ?? []).map((c) => [`[${c.status}] ${checkLabel(c.name)}: ${c.message}`, ...(c.details ?? []).map((d) => `  ${d}`)].join('\n')).join('\n'),
    [data],
  );

  const upgrade = async () => {
    setUpgrading(true);
    setUpgradeMessage(t('updateDialog.updating'));
    try {
      const result = await api.upgradeApp();
      if (result.devMode) {
        setUpgradeMessage(t('updateDialog.restartDev'));
        await Promise.all([refetch(), queryClient.invalidateQueries({ queryKey: queryKeys.versionCheck })]);
        setUpgrading(false);
        return;
      }
      setUpgradeMessage(t('updateDialog.restarting'));
      await api.restartApp({ clearCache: true });
      // The server is coming back up; poll until it answers, then show the new build.
      for (let i = 0; i < 40; i++) {
        await new Promise((resolve) => setTimeout(resolve, 500));
        try {
          await api.health();
          window.location.reload();
          return;
        } catch { /* still restarting */ }
      }
      setUpgradeMessage(t('updateDialog.restartManual'));
      setUpgrading(false);
    } catch (e) {
      setUpgradeMessage((e as Error).message);
      setUpgrading(false);
    }
  };

  const header = (
    <>
      <PageHeader
        className="!mb-0"
        title={t('layout.nav.settings')}
        subtitle={`${t(isProjectMode ? 'app.project' : 'app.global')}${overview.data?.configDir ? ` · ${shortenHome(overview.data.configDir)}` : ''}`}
      />
      <SettingsTabs current="doctor" />
    </>
  );

  if (isPending) return <div className="ss-wrap animate-fade-in">{header}<PageSkeleton /></div>;
  if (error) {
    return (
      <div className="ss-wrap animate-fade-in">
        {header}
        <div className="ss-note bad"><span className="flex-1">{t('doctor.error.failedToLoad', { error: error.message })}</span></div>
      </div>
    );
  }

  const summary = data!.summary;
  const counts: Record<Filter, number> = { all: summary.total, error: summary.errors, warning: summary.warnings, pass: summary.pass };
  // Only the parts that have something to report, so a healthy setup reads "13 passed."
  const summaryLine = [
    summary.errors > 0 && t(summary.errors === 1 ? 'doctor.summary.errors.one' : 'doctor.summary.errors.other', { count: summary.errors }),
    summary.warnings > 0 && t(summary.warnings === 1 ? 'doctor.summary.warnings.one' : 'doctor.summary.warnings.other', { count: summary.warnings }),
    t('doctor.summary.passed', { count: summary.pass }),
  ].filter(Boolean).join(', ');

  return (
    <div className="ss-wrap animate-fade-in">
      {header}

      <div className="flex items-center justify-between gap-6">
        <p className="max-w-[560px] text-[13px] leading-relaxed text-ink-2">
          {t('doctor.subtitle')} <b className="font-semibold text-ink">{summaryLine}</b>
        </p>
        <div className="flex items-center gap-2">
          <CopyButton unstyled value={report} size={14} className="ss-btn sm" title={t('doctor.copyReport')} label={t('doctor.copyReport')} copiedLabel={t('doctor.copied')} />
          <Button variant="secondary" size="sm" onClick={() => refetch()} loading={isFetching}><RefreshCw size={14} />{t('doctor.recheck')}</Button>
        </div>
      </div>

      <div className="ss-seg self-start" role="radiogroup" aria-label={t('doctor.filter.all')}>
        {FILTERS.map((f) => (
          <button
            key={f}
            type="button"
            role="radio"
            aria-checked={filter === f}
            className={filter === f ? 'on' : ''}
            disabled={counts[f] === 0 && f !== 'all'}
            onClick={() => { setFilter(f); setOpen(null); }}
          >
            {t(`doctor.filter.${f}`)}
            <span className="ss-cnt">{counts[f]}</span>
          </button>
        ))}
      </div>

      <div className="ss-list">
        {checks.length === 0 ? (
          <p className="px-4 py-8 text-center text-[13px] text-ink-3">{t('doctor.filter.noMatch')}</p>
        ) : (
          checks.map((check, i) => <CheckRow key={`${check.name}-${i}`} check={check} open={open === `${check.name}-${i}`} onToggle={() => setOpen(open === `${check.name}-${i}` ? null : `${check.name}-${i}`)} />)
        )}
      </div>

      {data!.version && (
        <div className="ss-box flex items-center gap-4 !py-3.5">
          {data!.version.update_available
            ? <ArrowUpCircle size={18} className="shrink-0 text-accent" />
            : <CheckCircle2 size={18} className="shrink-0 text-ok" />}
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="text-[13px] font-semibold">{t('doctor.version.title')}</span>
            <span className="font-mono text-[12.5px] text-ink-2">
              {data!.version.current}
              {data!.version.latest && data!.version.latest !== data!.version.current ? ` → ${data!.version.latest}` : ''}
            </span>
          </span>
          {upgradeMessage && <span className="text-[13px] text-ink-2">{upgradeMessage}</span>}
          {data!.version.update_available ? (
            <Button variant="primary" size="sm" onClick={upgrade} loading={upgrading}><ArrowUpCircle size={14} />{t('updateDialog.updateNow')}</Button>
          ) : (
            <span className="ss-st ok">{t('dashboard.version.upToDate')}</span>
          )}
        </div>
      )}
    </div>
  );
}

function CheckRow({ check, open, onToggle }: { check: DoctorCheck; open: boolean; onToggle: () => void }) {
  const t = useT();
  const details = check.details ?? [];
  const suggestions = check.suggestions ?? [];
  const expandable = details.length > 0 || suggestions.length > 0;
  const Icon = ICON[check.status];
  const label = t(`doctor.check.${check.name}`, {}, checkLabel(check.name));

  const row = (
    <>
      <Icon size={16} className={`shrink-0 ${TONE[check.status]}`} />
      <span className="nm m w-[210px] shrink-0 truncate">{label}</span>
      <span className="min-w-0 flex-1 truncate text-[13px] text-ink-2">{check.message}</span>
      {expandable && (open ? <ChevronDown size={15} className="shrink-0 text-ink-3" /> : <ChevronRight size={15} className="shrink-0 text-ink-3" />)}
    </>
  );

  if (!expandable) return <div className="ss-r">{row}</div>;

  return (
    <div>
      <button type="button" className="ss-r link w-full text-left" aria-expanded={open} onClick={onToggle}>{row}</button>
      {open && (
        <div className="ss-r fold !items-start !py-3.5 !pl-[44px]">
          <div className="flex min-w-0 flex-1 flex-col gap-3">
            {details.length > 0 && <DetailList name={check.name} details={details} />}
            {check.name === 'skillignore' && (
              <Link to="/config?tab=skillignore" className="ss-btn sm self-start"><Pencil size={14} />{t('sync.ignored.edit')}</Link>
            )}
            {suggestions.length > 0 && (
              <div className="flex flex-col gap-1.5">
                <span className="text-[12px] font-semibold text-ink-3">{t('doctor.suggestions')}</span>
                <ul className="flex flex-col gap-1 text-[13px] text-ink-2">
                  {suggestions.map((s) => <li key={s}>→ {s}</li>)}
                </ul>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function DetailList({ name, details }: { name: string; details: string[] }) {
  const t = useT();
  // The skillignore check packs two lists into one array, separated by "---".
  const sep = details.indexOf('---');
  if (name === 'skillignore' && sep !== -1) {
    const groups: [string, string[]][] = [
      [t('doctor.skillignore.patterns'), details.slice(0, sep)],
      [t('doctor.skillignore.ignoredSkills'), details.slice(sep + 1)],
    ];
    return (
      <div className="flex flex-col gap-3">
        {groups.filter(([, items]) => items.length > 0).map(([title, items]) => (
          <div key={title} className="flex flex-col gap-1.5">
            <span className="text-[12px] font-semibold text-ink-3">{title}</span>
            <div className="flex flex-wrap gap-1.5">
              {items.map((item) => <span key={item} className="ss-tag font-mono">{item}</span>)}
            </div>
          </div>
        ))}
      </div>
    );
  }
  return (
    <ul className="flex flex-col gap-1 text-[13px] text-ink-2">
      {details.map((d) => <li key={d}>· {d}</li>)}
    </ul>
  );
}
