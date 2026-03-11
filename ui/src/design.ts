/**
 * Minimal Design System Constants
 *
 * Simple border-radius, subtle shadows, and semantic colors
 * for inline styles where Tailwind classes aren't sufficient.
 */

/** Standard border-radius values */
export const radius = {
  /** Small elements — badges, chips */
  sm: '4px',
  /** Medium elements — cards, containers */
  md: '8px',
  /** Large elements — modals, panels */
  lg: '12px',
  /** Buttons */
  btn: '6px',
  /** Full round — avatars, pills */
  full: '9999px',
} as const;

/** Shadow presets (mirrors CSS variables for inline use) */
export const shadows = {
  sm: 'var(--shadow-sm)',
  md: 'var(--shadow-md)',
  lg: 'var(--shadow-lg)',
  hover: 'var(--shadow-hover)',
  active: 'none',
  accent: 'var(--shadow-accent)',
  blue: 'var(--shadow-blue)',
} as const;

/** Semantic colors for inline styles (audit helpers, charts) */
export const palette = {
  accent: '#dc4538',
  info: '#2d5da1',
  success: '#2e8b57',
  warning: '#d4870e',
  danger: '#c0392b',
} as const;

// ── Backward compatibility aliases ──────────────────────
// These will be removed after all files are migrated.
// DO NOT use in new code.
/** @deprecated Use `radius` instead */
export const wobbly = {
  full: radius.full,
  md: radius.md,
  sm: radius.sm,
  btn: radius.btn,
} as const;

/** @deprecated Use `palette` instead */
export const colors = {
  paper: '#f7f6f3',
  paperWarm: '#ffffff',
  pencil: '#141312',
  pencilLight: '#5a5750',
  muted: '#e2dfd8',
  mutedDark: '#9e9b94',
  accent: '#dc4538',
  blue: '#2d5da1',
  postit: '#fff9c4',
  success: '#2e8b57',
  warning: '#d4870e',
  danger: '#c0392b',
} as const;
