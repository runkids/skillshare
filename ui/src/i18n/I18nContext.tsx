import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import {
  getInitialLocale,
  LOCALE_STORAGE_KEY,
  supportedLocales,
  translate,
  type Locale,
  type TranslationParams,
} from './locales';

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string, params?: TranslationParams, fallback?: string) => string;
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => getInitialLocale());

  useEffect(() => {
    const meta = supportedLocales.find((entry) => entry.code === locale);
    document.documentElement.lang = locale;
    document.documentElement.dir = meta && 'dir' in meta ? meta.dir : 'ltr';
  }, [locale]);

  const setLocale = useCallback((nextLocale: Locale) => {
    setLocaleState(nextLocale);
    try {
      localStorage.setItem(LOCALE_STORAGE_KEY, nextLocale);
    } catch {
      // Locale switching should still work when storage is unavailable.
    }
  }, []);

  const t = useCallback(
    (key: string, params?: TranslationParams, fallback?: string) =>
      translate(locale, key, params, fallback),
    [locale],
  );

  const value = useMemo<I18nContextValue>(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) {
    throw new Error('useI18n must be used within I18nProvider');
  }
  return value;
}

export function useT() {
  return useI18n().t;
}
