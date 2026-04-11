/**
 * Minimal Design System Constants
 *
 * Simple border-radius, subtle shadows, and semantic colors
 * for inline styles where Tailwind classes aren't sufficient.
 */

/** Standard border-radius values (resolve via CSS custom properties for theme override) */
export const radius = {
  /** Small elements — badges, chips */
  sm: 'var(--radius-sm)',
  /** Medium elements — cards, containers */
  md: 'var(--radius-md)',
  /** Large elements — modals, panels */
  lg: 'var(--radius-lg)',
  /** Buttons — pill shape */
  btn: 'var(--radius-btn)',
  /** Full round — avatars, pills */
  full: 'var(--radius-full)',
} as const;

export const wobbly = radius;

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
