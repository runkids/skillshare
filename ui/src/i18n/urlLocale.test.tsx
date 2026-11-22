import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { I18nProvider, LOCALE_STORAGE_KEY, applyUrlLocaleParam, useI18n } from './index';

// A page load: the module-level step reads the address, then the provider starts.
function load() {
  applyUrlLocaleParam();
  return renderHook(useI18n, { wrapper: I18nProvider });
}

describe('?lang= URL parameter', () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => window.history.replaceState(null, '', '/'));

  it('wins over the saved locale, is saved, and leaves the address', () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'ja');
    window.history.replaceState(null, '', '/skills?theme=clean&lang=en#top');
    const { result } = load();
    expect(result.current.locale).toBe('en');
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en');
    expect(window.location.pathname + window.location.search + window.location.hash).toBe(
      '/skills?theme=clean#top',
    );
  });

  it('ignores an unsupported value', () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'ja');
    window.history.replaceState(null, '', '/?lang=tlh');
    expect(load().result.current.locale).toBe('ja');
    expect(window.location.search).toBe('');
  });

  it('keeps a language picked after load across a reload inside the frame', () => {
    window.history.replaceState(null, '', '/?theme=clean&lang=fr');
    const first = load();
    expect(first.result.current.locale).toBe('fr');
    act(() => first.result.current.setLocale('en'));
    first.unmount();

    // window.location.reload() reopens the current address.
    expect(load().result.current.locale).toBe('en');
  });
});
