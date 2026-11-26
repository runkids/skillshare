import { createContext, useContext, useState, useEffect, useCallback, useMemo } from 'react';
import type { ReactNode } from 'react';
import { themeFromSearch } from '../lib/themeParams';
import { isFromApp, postToApp } from '../lib/embeddingApp';
import {
  DEFAULT_CUSTOM_THEME,
  fontUrl,
  parseCustomTheme,
  rootAttributes,
  serializeCustomTheme,
  themeCss,
  type CustomTheme,
} from '../lib/customTheme';

export type Style = 'clean' | 'playful' | 'custom';
/** The built-in style a page is drawn with; a custom theme recolours one of these. */
export type BaseStyle = 'clean' | 'playful';
export type ModePreference = 'light' | 'dark' | 'system';
export type ResolvedMode = 'light' | 'dark';

interface ThemeContextValue {
  style: Style;
  setStyle: (s: Style) => void;
  baseStyle: BaseStyle;
  customTheme: CustomTheme;
  setCustomTheme: (t: CustomTheme) => void;
  modePreference: ModePreference;
  setModePreference: (m: ModePreference) => void;
  resolvedMode: ResolvedMode;
  // Legacy compat
  theme: ResolvedMode;
  toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextValue>({
  style: 'clean',
  setStyle: () => {},
  baseStyle: 'clean',
  customTheme: DEFAULT_CUSTOM_THEME,
  setCustomTheme: () => {},
  modePreference: 'light',
  setModePreference: () => {},
  resolvedMode: 'light',
  theme: 'light',
  toggleTheme: () => {},
});

export function useTheme() {
  return useContext(ThemeContext);
}

/** Persist the theme the URL asks for to localStorage (runs once on load). */
function applyUrlThemeParam(): void {
  const { style, mode } = themeFromSearch(window.location.search);
  if (mode) localStorage.setItem('skillshare-theme-preference', mode);
  if (style) localStorage.setItem('skillshare-style', style);
}
applyUrlThemeParam();

// Tell an embedding app (the desktop shell) that a new page is starting. It arrives before the
// first theme-change reports, so the app can tell this page's starting theme from a user's choice.
postToApp({ type: 'theme-init' });

// The custom theme as JSON, and the stylesheet derived from it for index.html to apply before
// first paint (the derivation lives in a module the inline script cannot load).
const CUSTOM_KEY = 'skillshare-custom-theme';
const CUSTOM_CSS_KEY = 'skillshare-custom-css';
const CUSTOM_FONT_KEY = 'skillshare-custom-font';
// Playful headings only; index.html adds it before first paint when the saved style needs it.
const KALAM_URL = 'https://fonts.googleapis.com/css2?family=Kalam:wght@400;700&display=swap';

function getInitialStyle(): Style {
  // Clean is the default; Playful or Custom only when someone picked it.
  const stored = localStorage.getItem('skillshare-style');
  return stored === 'playful' || stored === 'custom' ? stored : 'clean';
}

function getInitialCustomTheme(): CustomTheme {
  try {
    return (
      parseCustomTheme(JSON.parse(localStorage.getItem(CUSTOM_KEY) ?? 'null')) ??
      DEFAULT_CUSTOM_THEME
    );
  } catch {
    return DEFAULT_CUSTOM_THEME;
  }
}

/** Apply the base style and, for Custom, its stylesheet and layout to <html>. */
function applyStyle(style: Style, custom: CustomTheme) {
  const root = document.documentElement;
  for (const [name, value] of Object.entries(rootAttributes(style, custom))) {
    if (value === null) root.removeAttribute(name);
    else root.setAttribute(name, value);
  }
  if (root.dataset.theme === 'playful' && !document.getElementById('ss-kalam')) {
    const kalam = document.createElement('link');
    kalam.id = 'ss-kalam';
    kalam.rel = 'stylesheet';
    kalam.href = KALAM_URL;
    document.head.appendChild(kalam);
  }
  let sheet = document.getElementById('ss-custom-theme');
  let font = document.getElementById('ss-custom-font') as HTMLLinkElement | null;
  const lang = custom.cjk ? root.lang : null;
  const url = style === 'custom' ? fontUrl(custom.font, lang) : null;
  if (!url) font?.remove();
  else if (font?.href !== url) {
    font?.remove();
    font = document.createElement('link');
    font.id = 'ss-custom-font';
    font.rel = 'stylesheet';
    font.href = url;
    document.head.appendChild(font);
  }
  if (style !== 'custom') {
    sheet?.remove();
    return;
  }
  const css = themeCss(custom);
  if (!sheet) {
    sheet = document.createElement('style');
    sheet.id = 'ss-custom-theme';
    document.head.appendChild(sheet);
  }
  // Cached for index.html to paint before React loads; rewritten only when it changes, since this
  // runs on every step of a colour drag.
  if (sheet.textContent !== css) {
    sheet.textContent = css;
    localStorage.setItem(CUSTOM_CSS_KEY, css);
  }
  if (url) localStorage.setItem(CUSTOM_FONT_KEY, url);
  else localStorage.removeItem(CUSTOM_FONT_KEY);
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
  const [customTheme, setCustomThemeState] = useState<CustomTheme>(getInitialCustomTheme);
  const [modePreference, setModePreferenceState] =
    useState<ModePreference>(getInitialModePreference);
  const [resolvedMode, setResolvedMode] = useState<ResolvedMode>(() =>
    resolveMode(getInitialModePreference()),
  );

  // Apply style to DOM. An embedding app gets the custom theme with the report, since it keeps
  // its own copy (an iframe's storage does not survive on every platform).
  // The CJK font follows the interface language, which the i18n provider sets on <html>.
  useEffect(() => {
    if (style !== 'custom' || !customTheme.cjk) return;
    const watch = new MutationObserver(() => applyStyle(style, customTheme));
    watch.observe(document.documentElement, { attributes: true, attributeFilter: ['lang'] });
    return () => watch.disconnect();
  }, [style, customTheme]);

  useEffect(() => {
    applyStyle(style, customTheme);
    localStorage.setItem('skillshare-style', style);
    const json = serializeCustomTheme(customTheme);
    localStorage.setItem(CUSTOM_KEY, json);
    postToApp(
      style === 'custom'
        ? { type: 'theme-change', theme: style, custom: JSON.parse(json) }
        : { type: 'theme-change', theme: style },
    );
  }, [style, customTheme]);

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
    postToApp({ type: 'theme-change', theme: resolved });

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
      postToApp({ type: 'theme-change', theme: next });
    };

    mq.addEventListener('change', handler);
    return () => {
      mq.removeEventListener('change', handler);
    };
  }, [modePreference]);

  // The browser tints its own bar (mobile address bar, Safari tab bar) with theme-color, so it
  // follows the page edge: the shell around Clean's inset panel, otherwise the page background.
  // The panel is inset only from 768px, so a resize or rotation across it updates the bar too.
  useEffect(() => {
    const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
    if (!meta) return;
    const root = document.documentElement;
    const wide = window.matchMedia('(min-width: 768px)');
    const update = () => {
      const inset = !root.dataset.theme && root.dataset.layout !== 'full' && wide.matches;
      const color = getComputedStyle(root)
        .getPropertyValue(inset ? '--shell' : '--bg')
        .trim();
      if (color) meta.content = color;
    };
    update();
    wide.addEventListener('change', update);
    return () => wide.removeEventListener('change', update);
  }, [style, customTheme, resolvedMode]);

  // Follow the theme in the URL on back/forward navigation, as on load.
  useEffect(() => {
    function syncFromUrl() {
      const { style: s, mode } = themeFromSearch(window.location.search);
      if (mode) applyWithTransition(() => setModePreferenceState(mode));
      if (s) applyWithTransition(() => setStyleState(s));
    }

    window.addEventListener('popstate', syncFromUrl);
    return () => window.removeEventListener('popstate', syncFromUrl);
  }, []);

  // Listen for theme-push from parent window (Tauri shell)
  useEffect(() => {
    function handleMessage(event: MessageEvent) {
      if (!isFromApp(event) || event.data?.type !== 'theme-push') return;
      const { mode, style: newStyle, custom } = event.data;
      if (newStyle === 'playful' || newStyle === 'clean') {
        applyWithTransition(() => setStyleState(newStyle));
      }
      if (newStyle === 'custom') {
        const theme = parseCustomTheme(custom);
        applyWithTransition(() => {
          // A theme equal to the current one keeps its identity, so it is not reported back.
          if (theme) {
            setCustomThemeState((current) =>
              serializeCustomTheme(current) === serializeCustomTheme(theme) ? current : theme,
            );
          }
          setStyleState('custom');
        });
      }
      if (mode === 'dark' || mode === 'light') {
        applyWithTransition(() => setModePreferenceState(mode));
      }
    }
    window.addEventListener('message', handleMessage);
    return () => window.removeEventListener('message', handleMessage);
  }, []);

  const setStyle = useCallback((s: Style) => {
    applyWithTransition(() => setStyleState(s));
  }, []);

  const setCustomTheme = useCallback((t: CustomTheme) => {
    applyWithTransition(() => setCustomThemeState(t));
  }, []);

  const setModePreference = useCallback((m: ModePreference) => {
    applyWithTransition(() => setModePreferenceState(m));
  }, []);

  const toggleTheme = useCallback(() => {
    setModePreference(resolvedMode === 'light' ? 'dark' : 'light');
  }, [resolvedMode, setModePreference]);

  const value = useMemo(
    () => ({
      style,
      setStyle,
      baseStyle: style === 'custom' ? customTheme.base : style,
      customTheme,
      setCustomTheme,
      modePreference,
      setModePreference,
      resolvedMode,
      theme: resolvedMode,
      toggleTheme,
    }),
    [
      style,
      setStyle,
      customTheme,
      setCustomTheme,
      modePreference,
      setModePreference,
      resolvedMode,
      toggleTheme,
    ],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
