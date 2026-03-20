import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, act } from '@testing-library/react';
import { renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { ThemeProvider, useTheme } from '../ThemeContext';

const mockMatchMedia = vi.fn().mockImplementation((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: vi.fn(),
  removeListener: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  dispatchEvent: vi.fn(),
}));

Object.defineProperty(window, 'matchMedia', { writable: true, value: mockMatchMedia });

function wrapper({ children }: { children: ReactNode }) {
  return <ThemeProvider>{children}</ThemeProvider>;
}

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.classList.remove('dark');
  mockMatchMedia.mockClear();
});

describe('ThemeContext', () => {
  it('defaults to playful style and system mode', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    expect(result.current.style).toBe('playful');
    expect(result.current.modePreference).toBe('light');
  });

  it('migrates old skillshare-theme key', () => {
    localStorage.setItem('skillshare-theme', 'dark');
    const { result } = renderHook(() => useTheme(), { wrapper });
    // After migration, preference should be 'dark'
    expect(result.current.modePreference).toBe('dark');
    // Old key should be removed
    expect(localStorage.getItem('skillshare-theme')).toBeNull();
    // New key should be set
    expect(localStorage.getItem('skillshare-theme-preference')).toBe('dark');
  });

  it('has data-theme=playful by default', () => {
    renderHook(() => useTheme(), { wrapper });
    expect(document.documentElement.getAttribute('data-theme')).toBe('playful');
  });

  it('removes data-theme when switching to clean', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    act(() => {
      result.current.setStyle('clean');
    });
    expect(document.documentElement.getAttribute('data-theme')).toBeNull();
  });

  it('restores data-theme when switching back to playful', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    act(() => {
      result.current.setStyle('clean');
    });
    expect(document.documentElement.getAttribute('data-theme')).toBeNull();
    act(() => {
      result.current.setStyle('playful');
    });
    expect(document.documentElement.getAttribute('data-theme')).toBe('playful');
  });

  it('toggles dark class based on mode preference', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    act(() => {
      result.current.setModePreference('dark');
    });
    expect(document.documentElement.classList.contains('dark')).toBe(true);

    act(() => {
      result.current.setModePreference('light');
    });
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('does not dynamically inject font link', () => {
    render(<ThemeProvider><div /></ThemeProvider>);
    const fontLinks = document.querySelectorAll('link[href*="Caveat"]');
    expect(fontLinks.length).toBe(0);
  });
});

describe('URL theme param', () => {
  afterEach(() => {
    window.history.pushState({}, '', window.location.pathname);
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    document.documentElement.classList.remove('dark');
  });

  it('?theme=dark persists mode preference to localStorage', async () => {
    window.history.pushState({}, '', '?theme=dark');
    vi.resetModules();
    await import('../ThemeContext');
    expect(localStorage.getItem('skillshare-theme-preference')).toBe('dark');
  });

  it('?theme=light persists mode preference to localStorage', async () => {
    window.history.pushState({}, '', '?theme=light');
    vi.resetModules();
    await import('../ThemeContext');
    expect(localStorage.getItem('skillshare-theme-preference')).toBe('light');
  });

  it('?theme=clean persists style to localStorage', async () => {
    window.history.pushState({}, '', '?theme=clean');
    vi.resetModules();
    await import('../ThemeContext');
    expect(localStorage.getItem('skillshare-style')).toBe('clean');
  });

  it('?theme=playful persists style to localStorage', async () => {
    window.history.pushState({}, '', '?theme=playful');
    vi.resetModules();
    await import('../ThemeContext');
    expect(localStorage.getItem('skillshare-style')).toBe('playful');
  });

  it('ignores unknown ?theme values', async () => {
    window.history.pushState({}, '', '?theme=neon');
    vi.resetModules();
    await import('../ThemeContext');
    expect(localStorage.getItem('skillshare-theme-preference')).toBeNull();
    expect(localStorage.getItem('skillshare-style')).toBeNull();
  });
});

describe('URL theme param — live popstate sync', () => {
  afterEach(() => {
    window.history.pushState({}, '', window.location.pathname);
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    document.documentElement.classList.remove('dark');
  });

  it('switches to dark on popstate with ?theme=dark', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    expect(result.current.resolvedMode).toBe('light');

    // Simulate back/forward navigation to a URL with ?theme=dark
    window.history.pushState({}, '', '?theme=dark');
    act(() => {
      window.dispatchEvent(new PopStateEvent('popstate'));
    });

    expect(result.current.modePreference).toBe('dark');
    expect(result.current.resolvedMode).toBe('dark');
  });

  it('switches style to clean on popstate with ?theme=clean', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    expect(result.current.style).toBe('playful');

    window.history.pushState({}, '', '?theme=clean');
    act(() => {
      window.dispatchEvent(new PopStateEvent('popstate'));
    });

    expect(result.current.style).toBe('clean');
  });

  it('does nothing on popstate without ?theme param', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    expect(result.current.style).toBe('playful');
    expect(result.current.resolvedMode).toBe('light');

    window.history.pushState({}, '', '?other=value');
    act(() => {
      window.dispatchEvent(new PopStateEvent('popstate'));
    });

    expect(result.current.style).toBe('playful');
    expect(result.current.resolvedMode).toBe('light');
  });
});
