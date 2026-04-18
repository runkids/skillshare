import en from './locales/en.json';
import zhTW from './locales/zh-TW.json';
import zhCN from './locales/zh-CN.json';
import ja from './locales/ja.json';
import ko from './locales/ko.json';
import es from './locales/es.json';
import fr from './locales/fr.json';
import de from './locales/de.json';
import fa from './locales/fa.json';
import ptBR from './locales/pt-BR.json';
import id from './locales/id.json';

export const DEFAULT_LOCALE = 'en';
export const LOCALE_STORAGE_KEY = 'skillshare:locale';

export const supportedLocales = [
  { code: 'en', nativeName: 'English' },
  { code: 'zh-TW', nativeName: '繁體中文' },
  { code: 'zh-CN', nativeName: '简体中文' },
  { code: 'ja', nativeName: '日本語' },
  { code: 'ko', nativeName: '한국어' },
  { code: 'es', nativeName: 'Español' },
  { code: 'fr', nativeName: 'Français' },
  { code: 'de', nativeName: 'Deutsch' },
  { code: 'fa', nativeName: 'فارسی', dir: 'rtl' },
  { code: 'pt-BR', nativeName: 'Português (BR)' },
  { code: 'id', nativeName: 'Bahasa Indonesia' },
] as const;

export type Locale = (typeof supportedLocales)[number]['code'];
export type Messages = Record<string, string>;
export type TranslationParams = Record<string, string | number | boolean | null | undefined>;

export const messagesByLocale: Record<Locale, Messages> = {
  en,
  'zh-TW': zhTW,
  'zh-CN': zhCN,
  ja,
  ko,
  es,
  fr,
  de,
  fa,
  'pt-BR': ptBR,
  id,
};

export const supportedLocaleCodes = supportedLocales.map((locale) => locale.code);

export function isLocale(value: string | null | undefined): value is Locale {
  return supportedLocaleCodes.includes(value as Locale);
}

export function normalizeLocale(value: string | null | undefined): Locale | null {
  if (!value) return null;
  const canonical = value.trim().replace('_', '-');
  if (isLocale(canonical)) return canonical;

  const lower = canonical.toLowerCase();
  if (lower === 'zh' || lower.startsWith('zh-hans') || lower === 'zh-cn' || lower === 'zh-sg') {
    return 'zh-CN';
  }
  if (
    lower.startsWith('zh-hant') ||
    lower === 'zh-tw' ||
    lower === 'zh-hk' ||
    lower === 'zh-mo'
  ) {
    return 'zh-TW';
  }
  if (lower === 'pt-br') return 'pt-BR';

  const base = lower.split('-')[0];
  if (isLocale(base)) return base;
  return null;
}

export function detectBrowserLocale(languages: readonly string[] | undefined): Locale {
  for (const language of languages ?? []) {
    const locale = normalizeLocale(language);
    if (locale) return locale;
  }
  return DEFAULT_LOCALE;
}

export function getInitialLocale(): Locale {
  try {
    const stored = localStorage.getItem(LOCALE_STORAGE_KEY);
    const storedLocale = normalizeLocale(stored);
    if (storedLocale) return storedLocale;
  } catch {
    // Ignore storage access failures and fall back to browser detection.
  }
  return detectBrowserLocale(navigator.languages?.length ? navigator.languages : [navigator.language]);
}

export function interpolate(template: string, params?: TranslationParams): string {
  if (!params) return template;
  return template.replace(/\{([a-zA-Z0-9_.-]+)\}/g, (match, name: string) => {
    const value = params[name];
    return value === null || value === undefined ? match : String(value);
  });
}

export function translate(
  locale: Locale,
  key: string,
  params?: TranslationParams,
  fallback?: string,
): string {
  const template = messagesByLocale[locale][key] ?? messagesByLocale[DEFAULT_LOCALE][key] ?? fallback ?? key;
  return interpolate(template, params);
}
