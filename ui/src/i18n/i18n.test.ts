import { describe, expect, it, beforeEach, vi } from 'vitest';
import {
  DEFAULT_LOCALE,
  LOCALE_STORAGE_KEY,
  detectBrowserLocale,
  getInitialLocale,
  interpolate,
  messagesByLocale,
  normalizeLocale,
  plural,
  supportedLocales,
  translate,
} from './index';

function placeholders(value: string): string[] {
  return [...new Set([...value.matchAll(/\{([a-zA-Z0-9_.-]+)\}/g)].map((m) => m[1]))].sort();
}

describe('i18n locale detection', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('detects simplified Chinese browser locales', () => {
    expect(detectBrowserLocale(['zh-CN'])).toBe('zh-CN');
    expect(detectBrowserLocale(['zh-Hans'])).toBe('zh-CN');
    expect(detectBrowserLocale(['zh-SG'])).toBe('zh-CN');
  });

  it('detects traditional Chinese browser locales', () => {
    expect(detectBrowserLocale(['zh-TW'])).toBe('zh-TW');
    expect(detectBrowserLocale(['zh-Hant'])).toBe('zh-TW');
    expect(detectBrowserLocale(['zh-HK'])).toBe('zh-TW');
  });

  it('falls back to English for unsupported locales', () => {
    expect(detectBrowserLocale(['tlh'])).toBe(DEFAULT_LOCALE);
    expect(normalizeLocale('pt-BR')).toBe('pt-BR');
    expect(normalizeLocale('fa-IR')).toBe('fa');
  });

  it('uses persisted locale before browser locale', () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'ja');
    expect(getInitialLocale()).toBe('ja');
  });
});

describe('i18n dictionaries', () => {
  it('keeps every locale in parity with English', () => {
    const canonicalKeys = Object.keys(messagesByLocale.en).sort();
    for (const { code } of supportedLocales) {
      expect(Object.keys(messagesByLocale[code]).sort(), code).toEqual(canonicalKeys);
    }
  });

  it('keeps placeholders in parity with English', () => {
    for (const [key, english] of Object.entries(messagesByLocale.en)) {
      const expected = placeholders(english);
      for (const { code } of supportedLocales) {
        expect(placeholders(messagesByLocale[code][key]), `${code}:${key}`).toEqual(expected);
      }
    }
  });

  it('interpolates named placeholders and preserves missing placeholders', () => {
    expect(interpolate('Updated {name}', { name: 'demo' })).toBe('Updated demo');
    expect(interpolate('Updated {name}')).toBe('Updated {name}');
  });

  it('picks the singular form for exactly one and the other form for everything else', () => {
    expect([0, 1, 2, undefined].map((count) => plural('sync.changes', count))).toEqual([
      'sync.changes.other',
      'sync.changes.one',
      'sync.changes.other',
      'sync.changes.other',
    ]);
    expect(translate('en', plural('sync.changes', 1), { count: 1 })).not.toBe(
      translate('en', plural('sync.changes', 2), { count: 1 }),
    );
  });

  it('falls back to English or the provided fallback', () => {
    expect(translate('zh-TW', 'theme.settings')).toBe('主題');
    expect(translate('zh-TW', 'missing.key', undefined, 'Fallback')).toBe('Fallback');
  });

  it('localizes resource, extra, and Kilo project status copy', () => {
    expect(translate('zh-TW', 'resources.status.enabled')).toBe('已啟用');
    expect(translate('zh-TW', 'extras.status.synced')).toBe('已同步');
    expect(translate('zh-TW', 'mcp.kilocodeProjectEnv', { name: 'mcp-test' })).toContain(
      'Kilo Code MCP mcp-test：Kilo 不允許',
    );
  });
});
