import { useT, useI18n, supportedLocales, type Locale } from '../i18n';
import PageHeader from '../components/PageHeader';
import { Select } from '../components/Select';
import CustomThemeSettings, { SharedThemeNote } from '../components/CustomThemeSettings';
import { useAppContext } from '../context/AppContext';
import { useTheme, type ModePreference, type Style } from '../context/ThemeContext';
import { useOverviewQuery } from '../hooks/useSharedQueries';
import { shortenHome } from '../lib/paths';
import { segSlide } from '../lib/segSlide';
import { SettingsTabs } from './SettingsPage';

/** Settings → Appearance: style, light or dark, language, and the custom theme editor. */
export default function AppearancePage() {
  const t = useT();
  const { locale, setLocale } = useI18n();
  const { style, setStyle, modePreference, setModePreference } = useTheme();
  const { isProjectMode } = useAppContext();
  const overview = useOverviewQuery();

  return (
    <div className="ss-wrap animate-page-in">
      <PageHeader
        className="!mb-0"
        title={t('layout.nav.settings')}
        subtitle={`${t(isProjectMode ? 'app.project' : 'app.global')}${overview.data?.configDir ? ` · ${shortenHome(overview.data.configDir)}` : ''}`}
      />
      <SettingsTabs current="appearance" />

      <div className="flex flex-col gap-8">
        <section>
          <SharedThemeNote />
          <div className="ss-list">
            <div className="ss-setrow">
              <div className="l">
                <b>{t('settings.appearance.style')}</b>
                <span>{t('settings.appearance.styleHint')}</span>
              </div>
              <div
                ref={segSlide}
                className="ss-seg shrink-0"
                role="radiogroup"
                aria-label={t('settings.appearance.style')}
              >
                {(['clean', 'playful', 'custom'] as Style[]).map((s) => (
                  <button
                    key={s}
                    type="button"
                    role="radio"
                    aria-checked={style === s}
                    className={style === s ? 'on' : ''}
                    onClick={() => setStyle(s)}
                  >
                    {t(`settings.appearance.style.${s}`)}
                  </button>
                ))}
              </div>
            </div>
            <div className="ss-setrow">
              <div className="l">
                <b>{t('settings.appearance.theme')}</b>
              </div>
              <div
                ref={segSlide}
                className="ss-seg shrink-0"
                role="radiogroup"
                aria-label={t('settings.appearance.theme')}
              >
                {(['light', 'dark', 'system'] as ModePreference[]).map((m) => (
                  <button
                    key={m}
                    type="button"
                    role="radio"
                    aria-checked={modePreference === m}
                    className={modePreference === m ? 'on' : ''}
                    onClick={() => setModePreference(m)}
                  >
                    {t(`settings.appearance.theme.${m}`)}
                  </button>
                ))}
              </div>
            </div>
            <div className="ss-setrow">
              <div className="l">
                <b>{t('settings.appearance.language')}</b>
              </div>
              <Select
                value={locale}
                onChange={(v) => setLocale(v as Locale)}
                className="w-[190px] shrink-0"
                options={supportedLocales.map((l) => ({ value: l.code, label: l.nativeName }))}
              />
            </div>
          </div>
        </section>
        {style === 'custom' && <CustomThemeSettings />}
      </div>
    </div>
  );
}
