import { createContext, useContext, useState, useEffect, useCallback, useMemo } from 'react';
import type { ReactNode } from 'react';

export type Style = 'clean' | 'playful';
export type ModePreference = 'light' | 'dark' | 'system';
export type ResolvedMode = 'light' | 'dark';

interface ThemeContextValue {
  style: Style;
  setStyle: (s: Style) => void;
  modePreference: ModePreference;
  setModePreference: (m: ModePreference) => void;
  resolvedMode: ResolvedMode;
  // Legacy compat
  theme: ResolvedMode;
  toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextValue>({
  style: 'playful',
  setStyle: () => {},
  modePreference: 'light',
  setModePreference: () => {},
  resolvedMode: 'light',
  theme: 'light',
  toggleTheme: () => {},
});

export function useTheme() {
  return useContext(ThemeContext);
}

/** Read ?theme= URL param and persist to localStorage (runs once on load). */
function applyUrlThemeParam(): void {
  const p = new URLSearchParams(window.location.search).get('theme')?.toLowerCase();
  if (!p) return;
  if (p === 'dark' || p === 'light') localStorage.setItem('skillshare-theme-preference', p);
  if (p === 'playful' || p === 'clean') localStorage.setItem('skillshare-style', p);
}
applyUrlThemeParam();

function getInitialStyle(): Style {
  const stored = localStorage.getItem('skillshare-style');
  if (stored === 'clean') return 'clean';
  return 'playful';
}

function getInitialModePreference(): ModePreference {
  // Migration: old 'skillshare-theme' key → new 'skillshare-theme-preference'
  const legacy = localStorage.getItem('skillshare-theme');
  if (legacy === 'light' || legacy === 'dark') {
    localStorage.setItem('skillshare-theme-preference', legacy);
    localStorage.removeItem('skillshare-theme');
    return legacy;
  }

  const stored = localStorage.getItem('skillshare-theme-preference');
  if (stored === 'light' || stored === 'dark' || stored === 'system') return stored;
  return 'light';
}

function resolveMode(pref: ModePreference): ResolvedMode {
  if (pref === 'light') return 'light';
  if (pref === 'dark') return 'dark';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

let transitionTimer: ReturnType<typeof setTimeout>;
function applyWithTransition(fn: () => void) {
  clearTimeout(transitionTimer);
  document.documentElement.classList.add('theme-transitioning');
  fn();
  transitionTimer = setTimeout(() => {
    document.documentElement.classList.remove('theme-transitioning');
  }, 300);
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [style, setStyleState] = useState<Style>(getInitialStyle);
  const [modePreference, setModePreferenceState] = useState<ModePreference>(getInitialModePreference);
  const [resolvedMode, setResolvedMode] = useState<ResolvedMode>(() => resolveMode(getInitialModePreference()));

  // Apply style to DOM
  useEffect(() => {
    const root = document.documentElement;
    if (style === 'playful') {
      root.setAttribute('data-theme', 'playful');
    } else {
      root.removeAttribute('data-theme');
    }
    localStorage.setItem('skillshare-style', style);
    window.parent.postMessage({ type: 'theme-change', theme: style }, '*');
  }, [style]);

  // Apply mode to DOM and handle system listener
  useEffect(() => {
    const root = document.documentElement;
    const resolved = resolveMode(modePreference);
    setResolvedMode(resolved);

    if (resolved === 'dark') {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }

    localStorage.setItem('skillshare-theme-preference', modePreference);
    window.parent.postMessage({ type: 'theme-change', theme: resolved }, '*');

    if (modePreference !== 'system') return;

    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = (e: MediaQueryListEvent) => {
      const next: ResolvedMode = e.matches ? 'dark' : 'light';
      setResolvedMode(next);
      if (next === 'dark') {
        root.classList.add('dark');
      } else {
        root.classList.remove('dark');
      }
      window.parent.postMessage({ type: 'theme-change', theme: next }, '*');
    };

    mq.addEventListener('change', handler);
    return () => {
      mq.removeEventListener('change', handler);
    };
  }, [modePreference]);

  // Sync theme from URL ?theme= param on popstate (back/forward navigation)
  useEffect(() => {
    function syncFromUrl() {
      const p = new URLSearchParams(window.location.search).get('theme')?.toLowerCase();
      if (!p) return;
      if (p === 'dark' || p === 'light') {
        applyWithTransition(() => setModePreferenceState(p));
      }
      if (p === 'playful' || p === 'clean') {
        applyWithTransition(() => setStyleState(p));
      }
    }

    window.addEventListener('popstate', syncFromUrl);
    return () => window.removeEventListener('popstate', syncFromUrl);
  }, []);

  const setStyle = useCallback((s: Style) => {
    applyWithTransition(() => setStyleState(s));
  }, []);

  const setModePreference = useCallback((m: ModePreference) => {
    applyWithTransition(() => setModePreferenceState(m));
  }, []);

  const toggleTheme = useCallback(() => {
    setModePreference(resolvedMode === 'light' ? 'dark' : 'light');
  }, [resolvedMode, setModePreference]);

  const value = useMemo(() => ({
    style,
    setStyle,
    modePreference,
    setModePreference,
    resolvedMode,
    theme: resolvedMode,
    toggleTheme,
  }), [style, setStyle, modePreference, setModePreference, resolvedMode, toggleTheme]);

  return (
    <ThemeContext.Provider value={value}>
      {children}
    </ThemeContext.Provider>
  );
}
