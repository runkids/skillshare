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
  if (lower.startsWith('zh-hant') || lower === 'zh-tw' || lower === 'zh-hk' || lower === 'zh-mo') {
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

/** The `?lang=` URL parameter, which an embedding app uses to carry the user's choice. */
export const LOCALE_URL_PARAM = 'lang';

// The locale `?lang=` asked for, for when storage cannot keep it.
let urlLocale: Locale | null = null;

/**
 * Save the locale `?lang=` asks for, then drop the parameter from the address (runs once on load).
 * The desktop app embeds this UI in an iframe whose storage WebKit does not keep between launches,
 * so it remembers the choice itself and passes it back in the URL. Left in the address, the
 * parameter would come back on a reload inside the frame and undo a language picked since.
 */
export function applyUrlLocaleParam(): void {
  const url = new URL(window.location.href);
  if (!url.searchParams.has(LOCALE_URL_PARAM)) return;
  const locale = normalizeLocale(url.searchParams.get(LOCALE_URL_PARAM));
  url.searchParams.delete(LOCALE_URL_PARAM);
  window.history.replaceState(window.history.state, '', url);
  if (!locale) return;
  urlLocale = locale;
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {
    // The URL still decides the locale when storage is unavailable.
  }
}

/** The locale to start with: the saved choice, then the one `?lang=` gave, then the browser's. */
export function getInitialLocale(): Locale {
  try {
    const stored = localStorage.getItem(LOCALE_STORAGE_KEY);
    const storedLocale = normalizeLocale(stored);
    if (storedLocale) return storedLocale;
  } catch {
    // Ignore storage access failures and fall back to browser detection.
  }
  if (urlLocale) return urlLocale;
  return detectBrowserLocale(
    navigator.languages?.length ? navigator.languages : [navigator.language],
  );
}

export function interpolate(template: string, params?: TranslationParams): string {
  if (!params) return template;
  return template.replace(/\{([a-zA-Z0-9_.-]+)\}/g, (match, name: string) => {
    const value = params[name];
    return value === null || value === undefined ? match : String(value);
  });
}

/**
 * The key of the form of `key` that fits `count`: `key.one` for exactly 1, otherwise `key.other`.
 * Every locale defines both forms, and the same split is used for all of them.
 */
export function plural(key: string, count: number | undefined): string {
  return `${key}.${count === 1 ? 'one' : 'other'}`;
}

export function translate(
  locale: Locale,
  key: string,
  params?: TranslationParams,
  fallback?: string,
): string {
  const template =
    messagesByLocale[locale][key] ?? messagesByLocale[DEFAULT_LOCALE][key] ?? fallback ?? key;
  return interpolate(template, params);
}
