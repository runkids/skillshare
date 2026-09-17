import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import PageHeader from '../components/PageHeader';
import { Select } from '../components/Select';
import { useToast } from '../components/Toast';
import { useAppContext } from '../context/AppContext';
import { useTheme, type ModePreference, type Style } from '../context/ThemeContext';
import { supportedLocales, useI18n, useT, type Locale } from '../i18n';
import { shortenHome } from '../lib/paths';
import { queryKeys, staleTimes } from '../lib/queryKeys';

const MODES = ['merge', 'copy', 'symlink'];
const SEVERITIES = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'];
const LOG_LIMITS = [100, 500, 1000, 5000, 0];

/** The parts of config.yaml this page edits. Go marshals the struct field names. */
interface ConfigShape {
  Mode?: string;
  Audit?: { BlockThreshold?: string };
  Log?: { MaxEntries?: number | null };
}

export const SETTINGS_TABS = [
  { key: 'general', to: '/settings', labelKey: 'settings.tab.general' },
  { key: 'backup', to: '/backup', labelKey: 'settings.tab.backup' },
  { key: 'log', to: '/log', labelKey: 'settings.tab.log' },
  { key: 'doctor', to: '/doctor', labelKey: 'settings.tab.doctor' },
  { key: 'extensions', to: '/config?tab=extensions', labelKey: 'settings.tab.extensions' },
  { key: 'files', to: '/config', labelKey: 'settings.tab.files' },
] as const;

export function SettingsTabs({ current }: { current: string }) {
  const t = useT();
  const { isProjectMode } = useAppContext();
  // A project keeps its skills in its own version control, so there is nothing to back up here.
  const tabs = isProjectMode ? SETTINGS_TABS.filter((tab) => tab.key !== 'backup') : SETTINGS_TABS;
  return (
    <nav className="ss-tabs" aria-label={t('layout.nav.settings')}>
      {tabs.map((tab) => (
        <Link key={tab.key} to={tab.to} className={current === tab.key ? 'on' : ''} aria-current={current === tab.key ? 'page' : undefined}>
          {t(tab.labelKey)}
        </Link>
      ))}
    </nav>
  );
}

export default function SettingsPage() {
  const t = useT();
  const { locale, setLocale } = useI18n();
  const { toast } = useToast();
  const { style, setStyle, modePreference, setModePreference } = useTheme();
  const { isProjectMode, projectRoot } = useAppContext();
  const queryClient = useQueryClient();

  const overview = useQuery({ queryKey: queryKeys.overview, queryFn: () => api.getOverview(), staleTime: staleTimes.overview });
  const config = useQuery({ queryKey: queryKeys.config, queryFn: () => api.getConfig(), staleTime: staleTimes.config });
  const cfg = (config.data?.config ?? {}) as ConfigShape;
  const [threshold, setThreshold] = useState<string | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: queryKeys.config });
  const patch = useMutation({
    mutationFn: (body: { mode?: string; logMaxEntries?: number }) => api.patchConfig(body),
    onSuccess: () => { invalidate(); toast(t('settings.toast.saved'), 'success'); },
    onError: (e: Error) => toast(e.message, 'error'),
  });
  const setAuditThreshold = useMutation({
    mutationFn: (value: string) => api.setAuditThreshold(value),
    onSuccess: (res) => { setThreshold(res.threshold); invalidate(); toast(t('settings.toast.saved'), 'success'); },
    onError: (e: Error) => toast(e.message, 'error'),
  });

  const home = isProjectMode ? projectRoot : overview.data?.configDir;
  const logLimit = cfg.Log?.MaxEntries ?? 1000;

  return (
    <div className="ss-wrap animate-fade-in">
      <PageHeader
        className="!mb-0"
        title={t('layout.nav.settings')}
        subtitle={`${t(isProjectMode ? 'app.project' : 'app.global')}${home ? ` · ${shortenHome(home)}` : ''}`}
      />
      <SettingsTabs current="general" />

      <div className="flex max-w-[860px] flex-col gap-8">
        <section>
          <div className="ss-sec"><h2 className="ss-h2">{t('settings.source.title')}</h2></div>
          <div className="ss-list">
            <div className="ss-setrow">
              <div className="l">
                <b>{t('settings.source.folder')}</b>
                <span>{t('settings.source.folderHint')}</span>
              </div>
              <span className="truncate font-mono text-[13px] text-ink-2" title={overview.data?.source}>{shortenHome(overview.data?.source ?? '')}</span>
              <Link to="/config" className="ss-btn sm shrink-0">{t('settings.source.change')}</Link>
            </div>
            {!isProjectMode && (
              <div className="ss-setrow">
                <div className="l">
                  <b>{t('settings.source.mode')}</b>
                  <span>{t('settings.source.modeHint')}</span>
                </div>
                <div className="ss-seg shrink-0" role="radiogroup" aria-label={t('settings.source.mode')}>
                  {MODES.map((m) => {
                    const on = (cfg.Mode || 'merge') === m;
                    return (
                      <button key={m} type="button" role="radio" aria-checked={on} className={on ? 'on' : ''} disabled={patch.isPending} onClick={() => patch.mutate({ mode: m })}>
                        {m}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        </section>

        <section>
          <div className="ss-sec"><h2 className="ss-h2">{t('settings.appearance.title')}</h2></div>
          <div className="ss-list">
            <div className="ss-setrow">
              <div className="l">
                <b>{t('settings.appearance.style')}</b>
                <span>{t('settings.appearance.styleHint')}</span>
              </div>
              <div className="ss-seg shrink-0" role="radiogroup" aria-label={t('settings.appearance.style')}>
                {(['clean', 'playful'] as Style[]).map((s) => (
                  <button key={s} type="button" role="radio" aria-checked={style === s} className={style === s ? 'on' : ''} onClick={() => setStyle(s)}>
                    {t(`settings.appearance.style.${s}`)}
                  </button>
                ))}
              </div>
            </div>
            <div className="ss-setrow">
              <div className="l"><b>{t('settings.appearance.theme')}</b></div>
              <div className="ss-seg shrink-0" role="radiogroup" aria-label={t('settings.appearance.theme')}>
                {(['light', 'dark', 'system'] as ModePreference[]).map((m) => (
                  <button key={m} type="button" role="radio" aria-checked={modePreference === m} className={modePreference === m ? 'on' : ''} onClick={() => setModePreference(m)}>
                    {t(`settings.appearance.theme.${m}`)}
                  </button>
                ))}
              </div>
            </div>
            <div className="ss-setrow">
              <div className="l"><b>{t('settings.appearance.language')}</b></div>
              <Select
                value={locale}
                onChange={(v) => setLocale(v as Locale)}
                className="w-[190px] shrink-0"
                options={supportedLocales.map((l) => ({ value: l.code, label: l.nativeName }))}
              />
            </div>
          </div>
        </section>

        <section>
          <div className="ss-sec"><h2 className="ss-h2">{t('settings.safety.title')}</h2></div>
          <div className="ss-list">
            <div className="ss-setrow">
              <div className="l">
                <b>{t('settings.safety.threshold')}</b>
                <span>{t('settings.safety.thresholdHint')}</span>
              </div>
              <Select
                value={threshold ?? cfg.Audit?.BlockThreshold ?? 'CRITICAL'}
                onChange={(v) => setAuditThreshold.mutate(v)}
                disabled={setAuditThreshold.isPending}
                className="w-[190px] shrink-0"
                options={SEVERITIES.map((s) => ({ value: s, label: t(s === 'CRITICAL' ? 'audit.threshold.criticalOnly' : 'audit.threshold.andAbove', { severity: s }) }))}
              />
            </div>
            {!isProjectMode && (
              <div className="ss-setrow">
                <div className="l">
                  <b>{t('settings.safety.logLimit')}</b>
                  <span>{t('settings.safety.logLimitHint')}</span>
                </div>
                <Select
                  value={String(logLimit)}
                  onChange={(v) => patch.mutate({ logMaxEntries: Number(v) })}
                  disabled={patch.isPending}
                  className="w-[190px] shrink-0"
                  options={LOG_LIMITS.map((n) => ({ value: String(n), label: n === 0 ? t('settings.safety.logUnlimited') : t('settings.safety.logLast', { count: n }) }))}
                />
              </div>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
