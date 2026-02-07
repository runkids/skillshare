/**
 * Hand-Drawn Design System Constants
 *
 * Centralizes wobbly border-radius values, shadow styles, and other
 * design tokens that need to be applied via inline styles (since Tailwind
 * can't express complex border-radius shorthand).
 */

export const wobbly = {
  /** Full wobbly oval — buttons, hero elements */
  full: '255px 15px 225px 15px / 15px 225px 15px 255px',
  /** Subtle wobbly — cards, containers */
  md: '185px 12px 165px 12px / 12px 165px 12px 185px',
  /** Gentle wobbly — small elements, badges */
  sm: '125px 10px 115px 10px / 10px 115px 10px 125px',
  /** Button-specific wobbly */
  btn: '255px 25px 225px 25px / 25px 225px 25px 255px',
} as const;

export const shadows = {
  /** Subtle shadow for cards */
  sm: '3px 3px 0px 0px rgba(45, 45, 45, 0.12)',
  /** Standard hard offset shadow */
  md: '4px 4px 0px 0px #2d2d2d',
  /** Heavy emphasis shadow */
  lg: '8px 8px 0px 0px #2d2d2d',
  /** Hover state — slightly reduced offset */
  hover: '2px 2px 0px 0px #2d2d2d',
  /** Active — pressed flat, no shadow */
  active: 'none',
  /** Accent colored shadow */
  accent: '4px 4px 0px 0px #ff4d4d',
  /** Blue accent shadow */
  blue: '4px 4px 0px 0px #2d5da1',
} as const;

export const colors = {
  paper: '#fdfbf7',
  paperWarm: '#faf6ee',
  pencil: '#2d2d2d',
  pencilLight: '#5a5a5a',
  muted: '#e5e0d8',
  mutedDark: '#c5bfb4',
  accent: '#ff4d4d',
  blue: '#2d5da1',
  postit: '#fff9c4',
  success: '#2e8b57',
  warning: '#d4870e',
  danger: '#c0392b',
} as const;
