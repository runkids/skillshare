import { createElement } from 'react';
import { act, cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PRESETS, encodeShare, themeVars } from '../lib/customTheme';

afterEach(() => vi.restoreAllMocks());

it('tells an embedding app that a new page is starting', async () => {
  const post = vi.fn();
  vi.spyOn(window, 'parent', 'get').mockReturnValue({ postMessage: post } as unknown as Window);
  vi.resetModules();
  await import('./ThemeContext');
  expect(post).toHaveBeenCalledWith({ type: 'theme-init' }, '*');
});

describe('the saved custom theme', () => {
  const ember = { ...PRESETS.ember, name: 'Mine' };

  async function mount() {
    localStorage.setItem('skillshare-style', 'custom');
    localStorage.setItem('skillshare-custom-theme', JSON.stringify(ember));
    vi.resetModules();
    const { ThemeProvider } = await import('./ThemeContext');
    render(createElement(ThemeProvider, null, null));
  }
  // An app's message comes from the window that embeds this page.
  const push = (data: unknown, source: MessageEventSource | null = window.parent) =>
    act(() => window.dispatchEvent(new MessageEvent('message', { data, source })));
  const saved = () => JSON.parse(localStorage.getItem('skillshare-custom-theme') ?? 'null');

  afterEach(() => {
    cleanup();
    localStorage.clear();
    document.getElementById('ss-kalam')?.remove();
    document.getElementById('ss-custom-font')?.remove();
  });

  it('loads the Noto Sans for the interface language, and follows it when it changes', async () => {
    document.documentElement.lang = 'zh-TW';
    localStorage.setItem('skillshare-style', 'custom');
    localStorage.setItem('skillshare-custom-theme', JSON.stringify({ ...ember, cjk: true }));
    vi.resetModules();
    const { ThemeProvider } = await import('./ThemeContext');
    render(createElement(ThemeProvider, null, null));
    const font = () => (document.getElementById('ss-custom-font') as HTMLLinkElement).href;
    const before = font();
    await act(async () => {
      document.documentElement.lang = 'ja';
    });
    expect([before.includes('Noto+Sans+TC'), font().includes('Noto+Sans+JP')]).toEqual([
      true,
      true,
    ]);
    document.documentElement.lang = 'en';
  });

  it('survives an app switching the dashboard to Clean', async () => {
    await mount();
    await push({ type: 'theme-push', style: 'clean', mode: 'light' });
    expect(saved()).toMatchObject({ name: 'Mine', accent: ember.accent });
  });

  it('does not report again when an app pushes the theme it already has', async () => {
    const post = vi.fn();
    vi.spyOn(window, 'parent', 'get').mockReturnValue({ postMessage: post } as unknown as Window);
    await mount();
    post.mockClear();
    await push({ type: 'theme-push', style: 'custom', custom: JSON.parse(JSON.stringify(ember)) });
    expect(post).not.toHaveBeenCalled();
  });

  it('ignores a theme from a window other than the embedding app', async () => {
    await mount();
    await push({ type: 'theme-push', style: 'clean' }, null);
    expect(localStorage.getItem('skillshare-style')).toBe('custom');
  });

  it('follows ?style= and ?mode= on back and forward', async () => {
    await mount();
    window.history.pushState({}, '', '/?style=playful&mode=dark');
    await act(() => window.dispatchEvent(new PopStateEvent('popstate')));
    window.history.replaceState({}, '', '/');
    expect([
      localStorage.getItem('skillshare-style'),
      localStorage.getItem('skillshare-theme-preference'),
    ]).toEqual(['playful', 'dark']);
  });

  it('loads the Playful heading font only for a Playful base', async () => {
    await mount();
    expect(document.getElementById('ss-kalam')).toBeNull();
    await push({ type: 'theme-push', style: 'playful' });
    expect(document.getElementById('ss-kalam')).not.toBeNull();
  });

  it('tells an embedding app the colours of each mode', async () => {
    const post = vi.fn();
    vi.spyOn(window, 'parent', 'get').mockReturnValue({ postMessage: post } as unknown as Window);
    await mount();
    expect(post).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'theme-change', theme: 'custom', vars: themeVars(ember) }),
      '*',
    );
  });

  it('starts with the theme an embedding app hands over in the URL', async () => {
    vi.spyOn(window, 'parent', 'get').mockReturnValue({
      postMessage: vi.fn(),
    } as unknown as Window);
    window.history.replaceState(
      {},
      '',
      `/?style=custom&custom-theme=${encodeShare(PRESETS.indigo)}`,
    );
    await mount();
    window.history.replaceState({}, '', '/');
    expect(saved()).toEqual(PRESETS.indigo);
  });

  it('ignores a theme in the URL when no app embeds the page', async () => {
    window.history.replaceState(
      {},
      '',
      `/?style=custom&custom-theme=${encodeShare(PRESETS.indigo)}`,
    );
    await mount();
    window.history.replaceState({}, '', '/');
    expect(saved()).toMatchObject({ name: 'Mine' });
  });

  it('is not replaced by an invalid theme from an app', async () => {
    await mount();
    await push({ type: 'theme-push', style: 'custom', custom: { v: 1, accent: 'red; }' } });
    expect(saved()).toMatchObject({ name: 'Mine', accent: ember.accent });
  });
});
