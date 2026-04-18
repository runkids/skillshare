import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Languages } from 'lucide-react';
import { shadows } from '../design';
import { supportedLocales, useI18n, type Locale } from '../i18n';

export default function LanguagePopover() {
  const { locale, setLocale, t } = useI18n();
  const [open, setOpen] = useState(false);
  const [dropUp, setDropUp] = useState(true);
  const containerRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const localeOptions = useMemo(() => {
    if (locale !== 'zh-CN') return supportedLocales;
    const simplified = supportedLocales.find((entry) => entry.code === 'zh-CN');
    return simplified
      ? [simplified, ...supportedLocales.filter((entry) => entry.code !== 'zh-CN')]
      : supportedLocales;
  }, [locale]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [open]);

  useLayoutEffect(() => {
    if (!open || !containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    const panelHeight = 320;
    setDropUp(rect.top > panelHeight);
  }, [open]);

  useEffect(() => {
    if (!open || !panelRef.current) return;
    const selected = panelRef.current.querySelector('[aria-checked="true"]') as HTMLElement | null;
    selected?.focus();
  }, [open]);

  const selectLocale = (nextLocale: Locale) => {
    setLocale(nextLocale);
    setOpen(false);
    triggerRef.current?.focus();
  };

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={triggerRef}
        onClick={() => setOpen(!open)}
        className="flex items-center gap-3 px-3 py-1.5 text-sm text-pencil-light hover:text-pencil hover:bg-muted/20 transition-colors cursor-pointer w-full"
        aria-label={t('language.settings')}
        aria-expanded={open}
      >
        <Languages size={16} strokeWidth={2.5} />
        {t('language.settings')}
      </button>

      {open && (
        <div
          ref={panelRef}
          role="radiogroup"
          aria-label={t('language.settings')}
          className={`
            absolute left-0 z-50 w-64 max-h-[320px] overflow-y-auto bg-surface border border-muted p-2 rounded-[var(--radius-md)] animate-dropdown-in
            ${dropUp ? 'bottom-full mb-2' : 'top-full mt-2'}
          `}
          style={{ boxShadow: shadows.lg }}
        >
          {localeOptions.map((entry) => (
            <button
              key={entry.code}
              role="radio"
              aria-checked={locale === entry.code}
              onClick={() => selectLocale(entry.code)}
              className={`
                w-full flex items-center justify-between gap-3 px-3 py-2 text-sm rounded-lg transition-colors cursor-pointer
                focus-visible:ring-2 focus-visible:ring-pencil/20 focus-visible:outline-none
                ${locale === entry.code
                  ? 'bg-pencil text-paper font-medium'
                  : 'bg-transparent text-pencil-light hover:text-pencil hover:bg-muted/30'}
              `}
              tabIndex={locale === entry.code ? 0 : -1}
            >
              <span>{entry.nativeName}</span>
              <span className="text-xs opacity-75">{t(`language.${entry.code}`)}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
