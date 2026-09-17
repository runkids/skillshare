import { useEffect, useMemo, useRef, useState } from 'react';
import { Check, Languages } from 'lucide-react';
import { messagesByLocale, supportedLocales, useI18n, type Locale } from '../i18n';

export default function LanguagePopover() {
  const { locale, setLocale, t } = useI18n();
  const [open, setOpen] = useState(false);
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
        type="button"
        onClick={() => setOpen(!open)}
        className={`ss-ib ${open ? 'bg-sel text-sel-ink' : ''}`}
        aria-label={t('language.settings')}
        title={t('language.settings')}
        aria-expanded={open}
      >
        <Languages size={16} />
      </button>

      {open && (
        <div
          ref={panelRef}
          role="radiogroup"
          aria-label={t('language.settings')}
          className="ss-menu absolute left-0 bottom-full mb-3 z-50 !w-[300px] animate-dropdown-in"
        >
          {localeOptions.map((entry) => {
            const on = locale === entry.code;
            const english = messagesByLocale.en[`language.${entry.code}`];
            return (
              <button
                key={entry.code}
                type="button"
                role="radio"
                aria-checked={on}
                onClick={() => selectLocale(entry.code)}
                className={on ? 'hv' : ''}
                tabIndex={on ? 0 : -1}
              >
                <span className="whitespace-nowrap">{entry.nativeName}</span>
                <span className="ml-auto text-xs text-ink-3 whitespace-nowrap">{english !== entry.nativeName ? t(`language.${entry.code}`) : ''}</span>
                {on ? <Check size={14} /> : <span className="w-3.5" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
