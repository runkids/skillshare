import { describe, it, expect, beforeEach, vi } from 'vitest';
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
