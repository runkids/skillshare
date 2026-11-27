/** Custom theme: a few validated options turned into the CSS variables of one base style.
 *  Neutrals are derived in OKLCH from the background choice; every text colour is then pushed
 *  lighter or darker until it reaches 4.5:1 on the surfaces it sits on. Untrusted input (an imported
 *  file, a shared link, a message from an embedding app) only reaches CSS through parseCustomTheme. */

export type CustomBase = 'clean' | 'playful';
// Near-greys first, then the tints in hue order around the wheel, so neighbouring chips blend.
export const BACKGROUNDS = [
  'neutral',
  'warm',
  'paper',
  'peach',
  'rose',
  'mauve',
  'lavender',
  'slate',
  'cool',
  'sky',
  'teal',
  'mint',
  'sage',
  'olive',
  'accent',
] as const;
export type Background = (typeof BACKGROUNDS)[number];
export type Corners = 'sharp' | 'default' | 'round';
export type FontChoice = 'system' | 'inter' | 'geist' | 'plex' | 'dm-sans';

export interface CustomTheme {
  v: 1;
  name: string;
  /** The preset the theme started from, so an edited one can say so and be restored. */
  from?: PresetId;
  base: CustomBase;
  accent: string;
  /** Dark-mode accent. Derived from `accent` when absent. */
  accentDark?: string;
  /** Link and focus colour. Same as the accent when absent. */
  link?: string;
  linkDark?: string;
  background: Background;
  corners: Corners;
  font: FontChoice;
  /** Also load Noto Sans for Chinese, Japanese or Korean, matching the interface language. Only
   *  with a web font: the system font already covers those scripts. */
  cjk?: boolean;
  /** Clean base only. */
  layout: 'inset' | 'full';
  headings: 'default' | 'tight';
  buttons: 'flat' | 'raised';
  /** Clean base only, like the options below: Playful keeps its own dots, shadows, borders and
   *  icons. */
  texture: 'none' | 'dots';
  depth: 'flat' | 'default' | 'lifted';
  borders: 'default' | 'bold';
  icons: 'light' | 'default' | 'bold';
  /** Frosted: menus and the sticky toolbar. Surfaces: cards, lists and the sidebar too. */
  glass: 'frosted' | 'surfaces' | 'solid';
}

export const ACCENT_SWATCHES = [
  '#141312',
  '#475569',
  '#1e293b',
  '#0369a1',
  '#2f5bd3',
  '#4f46e5',
  '#6d28d9',
  '#a21caf',
  '#db2777',
  '#be123c',
  '#dc2626',
  '#b53d0b',
  '#b45309',
  '#a16207',
  '#4d7c0f',
  '#15803d',
  '#047857',
  '#0f766e',
  '#0e7490',
];

const PRESET_DEFAULTS = {
  v: 1,
  base: 'clean',
  corners: 'default',
  layout: 'inset',
  headings: 'default',
  buttons: 'flat',
  texture: 'none',
  depth: 'default',
  borders: 'default',
  icons: 'default',
  glass: 'frosted',
} as const;

export const PRESETS = {
  pine: {
    ...PRESET_DEFAULTS,
    name: 'Pine',
    accent: '#0f766e',
    accentDark: '#2dd4bf',
    background: 'accent',
    font: 'inter',
  },
  indigo: {
    ...PRESET_DEFAULTS,
    name: 'Indigo',
    accent: '#4f46e5',
    accentDark: '#9b98ff',
    background: 'neutral',
    font: 'geist',
    headings: 'tight',
    buttons: 'raised',
  },
  ember: {
    ...PRESET_DEFAULTS,
    name: 'Ember',
    accent: '#b53d0b',
    accentDark: '#fb923c',
    background: 'warm',
    corners: 'round',
    font: 'dm-sans',
  },
  graphite: {
    ...PRESET_DEFAULTS,
    name: 'Graphite',
    accent: '#1e293b',
    accentDark: '#e2e8f0',
    link: '#3d5a80',
    linkDark: '#8fb4e0',
    background: 'cool',
    corners: 'sharp',
    font: 'plex',
  },
  tide: {
    ...PRESET_DEFAULTS,
    name: 'Tide',
    accent: '#0369a1',
    accentDark: '#38bdf8',
    background: 'cool',
    font: 'inter',
  },
  sakura: {
    ...PRESET_DEFAULTS,
    name: 'Sakura',
    accent: '#db2777',
    accentDark: '#f472b6',
    background: 'accent',
    corners: 'round',
    font: 'dm-sans',
    buttons: 'raised',
  },
  plum: {
    ...PRESET_DEFAULTS,
    name: 'Plum',
    accent: '#7e22ce',
    accentDark: '#c084fc',
    background: 'neutral',
    font: 'geist',
    headings: 'tight',
  },
  moss: {
    ...PRESET_DEFAULTS,
    name: 'Moss',
    accent: '#4d7c0f',
    accentDark: '#a3e635',
    background: 'warm',
    corners: 'sharp',
    font: 'plex',
  },
  ink: {
    ...PRESET_DEFAULTS,
    name: 'Ink',
    accent: '#141312',
    accentDark: '#e7e5e4',
    background: 'neutral',
    corners: 'sharp',
    font: 'system',
    layout: 'full',
    headings: 'tight',
  },
  midnight: {
    ...PRESET_DEFAULTS,
    name: 'Midnight',
    accent: '#1d4ed8',
    accentDark: '#93c5fd',
    background: 'cool',
    corners: 'round',
    font: 'inter',
    buttons: 'raised',
  },
  sunny: {
    ...PRESET_DEFAULTS,
    name: 'Sunny',
    base: 'playful',
    accent: '#b45309',
    accentDark: '#fbbf24',
    background: 'warm',
    corners: 'round',
    font: 'dm-sans',
  },
  lagoon: {
    ...PRESET_DEFAULTS,
    name: 'Lagoon',
    base: 'playful',
    accent: '#0e7490',
    accentDark: '#22d3ee',
    background: 'cool',
    font: 'system',
  },
  bubblegum: {
    ...PRESET_DEFAULTS,
    name: 'Bubblegum',
    base: 'playful',
    accent: '#c026d3',
    accentDark: '#f0abfc',
    background: 'accent',
    corners: 'round',
    font: 'dm-sans',
  },
  crayon: {
    ...PRESET_DEFAULTS,
    name: 'Crayon',
    base: 'playful',
    accent: '#dc2626',
    accentDark: '#f87171',
    background: 'warm',
    corners: 'sharp',
    font: 'system',
  },
} satisfies Record<string, CustomTheme>;

export type PresetId = keyof typeof PRESETS;
export const DEFAULT_CUSTOM_THEME: CustomTheme = { ...PRESETS.pine, from: 'pine' };

const OPTIONAL_HEX = ['accentDark', 'link', 'linkDark'] as const;

/** The preset a theme still matches, ignoring its name. */
export function presetOf(theme: CustomTheme): PresetId | null {
  const key = (t: CustomTheme) =>
    serializeCustomTheme({ ...t, name: '', from: undefined, cjk: undefined });
  const mine = key(theme);
  return (Object.keys(PRESETS) as PresetId[]).find((id) => key(PRESETS[id]) === mine) ?? null;
}

/** The preset an edited theme started from, or null when it still matches one or names none. */
export function editedFrom(theme: CustomTheme): PresetId | null {
  return theme.from && !presetOf(theme) ? theme.from : null;
}

/** The name to show: the matching preset's, the theme's own unless it borrows a preset's name, or
 *  null for an edited preset (callers show a generic "Custom"). */
export function themeName(theme: CustomTheme): string | null {
  const preset = presetOf(theme);
  if (preset) return PRESETS[preset].name;
  const borrowed = Object.values(PRESETS).some((p) => p.name === theme.name);
  return borrowed || !theme.name ? null : theme.name;
}

// ---- Validation ----

const HEX = /^#[0-9a-f]{6}$/;
const ENUMS = {
  base: ['clean', 'playful'],
  background: BACKGROUNDS,
  corners: ['sharp', 'default', 'round'],
  font: ['system', 'inter', 'geist', 'plex', 'dm-sans'],
  layout: ['inset', 'full'],
  headings: ['default', 'tight'],
  buttons: ['flat', 'raised'],
  texture: ['none', 'dots'],
  depth: ['flat', 'default', 'lifted'],
  borders: ['default', 'bold'],
  icons: ['light', 'default', 'bold'],
  glass: ['frosted', 'surfaces', 'solid'],
} as const;

function hex(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const v = value.trim().toLowerCase();
  const short = /^#([0-9a-f])([0-9a-f])([0-9a-f])$/.exec(v);
  if (short) return `#${short[1]}${short[1]}${short[2]}${short[2]}${short[3]}${short[3]}`;
  return HEX.test(v) ? v : null;
}

/** Accepts `#abc` or `#aabbcc` as typed into a hex field. */
export function parseHex(value: string): string | null {
  return hex(value.startsWith('#') ? value : `#${value}`);
}

/** A theme from untrusted JSON, or null. Unknown keys are dropped; missing enums take defaults. */
export function parseCustomTheme(input: unknown): CustomTheme | null {
  if (!input || typeof input !== 'object' || Array.isArray(input)) return null;
  const o = input as Record<string, unknown>;
  if (o.v !== 1) return null;
  const accent = hex(o.accent);
  if (!accent) return null;
  const theme: CustomTheme = {
    ...PRESET_DEFAULTS,
    name:
      typeof o.name === 'string'
        ? o.name
            .replace(/[\p{C}]/gu, '')
            .trim()
            .slice(0, 40)
        : '',
    accent,
    background: 'neutral',
    font: 'system',
  };
  for (const [key, allowed] of Object.entries(ENUMS)) {
    const value = o[key];
    if (value === undefined) continue;
    if (!(allowed as readonly unknown[]).includes(value)) return null;
    (theme as unknown as Record<string, unknown>)[key] = value;
  }
  // A preset this version does not have only loses the label; the colours still apply.
  if (typeof o.from === 'string' && Object.hasOwn(PRESETS, o.from)) theme.from = o.from as PresetId;
  if (o.cjk !== undefined) {
    if (typeof o.cjk !== 'boolean') return null;
    if (o.cjk) theme.cjk = true;
  }
  for (const key of OPTIONAL_HEX) {
    if (o[key] === undefined) continue;
    const value = hex(o[key]);
    if (!value) return null;
    theme[key] = value;
  }
  return theme;
}

/** Stable JSON for export and storage. */
export function serializeCustomTheme(t: CustomTheme): string {
  return JSON.stringify({
    v: 1,
    name: t.name,
    ...(t.from ? { from: t.from } : {}),
    base: t.base,
    accent: t.accent,
    ...(t.accentDark ? { accentDark: t.accentDark } : {}),
    ...(t.link ? { link: t.link } : {}),
    ...(t.linkDark ? { linkDark: t.linkDark } : {}),
    background: t.background,
    corners: t.corners,
    font: t.font,
    ...(t.cjk ? { cjk: true } : {}),
    layout: t.layout,
    headings: t.headings,
    buttons: t.buttons,
    texture: t.texture,
    depth: t.depth,
    borders: t.borders,
    icons: t.icons,
    glass: t.glass,
  });
}

/** base64url of the JSON, for a `?custom=` share link. */
export function encodeShare(theme: CustomTheme): string {
  const bytes = new TextEncoder().encode(serializeCustomTheme(theme));
  let bin = '';
  bytes.forEach((b) => (bin += String.fromCharCode(b)));
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function decodeShare(value: string): CustomTheme | null {
  try {
    const b64 = value.replace(/-/g, '+').replace(/_/g, '/');
    const bin = atob(b64 + '='.repeat((4 - (b64.length % 4)) % 4));
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
    return parseCustomTheme(JSON.parse(new TextDecoder().decode(bytes)));
  } catch {
    return null;
  }
}

// ---- Colour ----

type Lch = [l: number, c: number, h: number];

function toLinear(c: number) {
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}
function fromLinear(c: number) {
  return c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055;
}
function rgbOf(h: string): [number, number, number] {
  return [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16) / 255) as [number, number, number];
}

export function toOklch(h: string): Lch {
  const [r, g, b] = rgbOf(h).map(toLinear);
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  const L = 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s;
  const A = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s;
  const B = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s;
  return [L, Math.hypot(A, B), ((Math.atan2(B, A) * 180) / Math.PI + 360) % 360];
}

function lchToRgb([L, C, H]: Lch): [number, number, number] {
  const a = C * Math.cos((H * Math.PI) / 180);
  const b = C * Math.sin((H * Math.PI) / 180);
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ].map(fromLinear) as [number, number, number];
}

/** The sRGB hex nearest an OKLCH colour, lowering chroma until it fits the gamut. */
export function fromOklch([L, C, H]: Lch): string {
  const l = Math.min(1, Math.max(0, L));
  let c = C;
  let rgb = lchToRgb([l, c, H]);
  while (c > 0 && rgb.some((v) => v < -0.0005 || v > 1.0005)) {
    c = Math.max(0, c - 0.002);
    rgb = lchToRgb([l, c, H]);
  }
  return (
    '#' +
    rgb
      .map((v) =>
        Math.round(Math.min(1, Math.max(0, v)) * 255)
          .toString(16)
          .padStart(2, '0'),
      )
      .join('')
  );
}

function luminance(h: string) {
  const [r, g, b] = rgbOf(h).map(toLinear);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

export function contrast(a: string, b: string): number {
  const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

export const MIN_CONTRAST = 4.5;

/** Move a colour's lightness away from its backgrounds until it reads at 4.5:1 on all of them. */
function guard(color: string, backgrounds: string[], dark: boolean): string {
  const worst = (c: string) => Math.min(...backgrounds.map((b) => contrast(c, b)));
  let [l, c, h] = toOklch(color);
  let out = color;
  // Stop at the end it moves towards; black and white still have one way to go.
  while (worst(out) < MIN_CONTRAST && (dark ? l < 1 : l > 0)) {
    l = dark ? l + 0.01 : l - 0.01;
    out = fromOklch([l, c, h]);
  }
  return out;
}

function alpha(h: string, a: number) {
  const [r, g, b] = rgbOf(h).map((v) => Math.round(v * 255));
  return `rgba(${r}, ${g}, ${b}, ${a})`;
}

// ---- Derivation ----

type Mode = 'light' | 'dark';
type Neutral =
  | 'bg'
  | 'side'
  | 'surface'
  | 'sunken'
  | 'ink'
  | 'ink-2'
  | 'ink-3'
  | 'line'
  | 'line-2'
  | 'sep'
  | 'frame'
  | 'sel';

// Lightness per role, averaged from Clean and the approved preset palettes; chroma is the
// background tint scaled per role.
const NEUTRALS: Record<Mode, Record<Neutral, [l: number, chroma: number]>> = {
  light: {
    bg: [0.977, 0.6],
    side: [0.955, 1],
    surface: [1, 0],
    sunken: [0.965, 0.9],
    ink: [0.195, 1.6],
    'ink-2': [0.447, 2],
    'ink-3': [0.515, 2],
    line: [0.92, 1.2],
    'line-2': [0.867, 1.6],
    sep: [0.937, 1],
    frame: [0.911, 1.3],
    sel: [0.93, 1.6],
  },
  dark: {
    bg: [0.18, 1.2],
    side: [0.155, 1],
    surface: [0.218, 1.5],
    sunken: [0.198, 1.3],
    ink: [0.945, 1],
    'ink-2': [0.74, 2.2],
    'ink-3': [0.66, 2.4],
    line: [0.275, 1.9],
    'line-2': [0.33, 2.2],
    sep: [0.258, 1.8],
    frame: [0.287, 2],
    sel: [0.28, 2.5],
  },
};

const TINTS: Record<Exclude<Background, 'accent'>, [chroma: number, hue: number]> = {
  neutral: [0.006, 88],
  warm: [0.013, 70],
  paper: [0.02, 88],
  peach: [0.016, 45],
  rose: [0.014, 12],
  mauve: [0.014, 335],
  lavender: [0.016, 295],
  slate: [0.018, 255],
  cool: [0.008, 255],
  sky: [0.016, 230],
  teal: [0.014, 200],
  mint: [0.014, 170],
  sage: [0.014, 145],
  olive: [0.016, 115],
};

// Status colours in OKLCH: [light text, light tint, dark text, dark tint].
const STATUS: Record<'ok' | 'warn' | 'bad', [Lch, Lch, Lch, Lch]> = {
  ok: [
    [0.5, 0.12, 152],
    [0.955, 0.03, 152],
    [0.76, 0.14, 152],
    [0.24, 0.04, 152],
  ],
  warn: [
    [0.5, 0.11, 70],
    [0.955, 0.04, 85],
    [0.82, 0.14, 80],
    [0.24, 0.04, 80],
  ],
  bad: [
    [0.52, 0.17, 27],
    [0.95, 0.03, 27],
    [0.72, 0.14, 25],
    [0.24, 0.04, 25],
  ],
};

const CORNERS: Record<Exclude<Corners, 'default'>, Record<string, string>> = {
  sharp: { 'r-ctl': '4px', 'r-btn': '6px', 'r-box': '6px', 'r-tag': '3px' },
  // Far enough from either base's default to read as a choice of its own: pill tags, soft cards.
  round: { 'r-ctl': '14px', 'r-btn': '999px', 'r-box': '22px', 'r-tag': '999px' },
};

// Shadows under cards and lists; Lifted stays below the menus' --sh-float.
const DEPTH: Record<'flat' | 'lifted', Record<Mode, string>> = {
  flat: { light: 'none', dark: 'none' },
  lifted: {
    light: '0 1px 3px rgba(20, 19, 18, 0.08), 0 8px 24px -8px rgba(20, 19, 18, 0.16)',
    dark: '0 1px 3px rgba(0, 0, 0, 0.45), 0 10px 28px -10px rgba(0, 0, 0, 0.75)',
  },
};

// Icon stroke width, far enough from Clean's own 1.75 to see at 15px.
const ICONS = { light: '1.25', bold: '2.4' };
const BOLD_BORDER = '2px';

// Menus and the sticky toolbar as the solid colour they float over, without the blur.
const SOLID = {
  'glass-menu': 'var(--surface)',
  'glass-bar': 'var(--bg)',
  mat: 'none',
  'mat-menu': 'none',
};

const SYSTEM_FONT =
  "-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Segoe UI Variable Text', 'Segoe UI', system-ui, sans-serif";
const SYSTEM_MONO = "ui-monospace, 'SF Mono', 'JetBrains Mono', Menlo, Consolas, monospace";

const FONTS: Record<
  Exclude<FontChoice, 'system'>,
  { sans: string; mono?: string; query: string }
> = {
  inter: { sans: 'Inter', query: 'family=Inter:wght@400;500;600;700' },
  geist: {
    sans: 'Geist',
    mono: 'Geist Mono',
    query: 'family=Geist:wght@400;500;600;700&family=Geist+Mono:wght@400;500',
  },
  plex: {
    sans: 'IBM Plex Sans',
    mono: 'IBM Plex Mono',
    query: 'family=IBM+Plex+Sans:wght@400;500;600;700&family=IBM+Plex+Mono:wght@400;500',
  },
  'dm-sans': { sans: 'DM Sans', query: 'family=DM+Sans:wght@400;500;600;700' },
};

// Noto Sans per interface language; the scripts share code points, so each needs its own.
const CJK_FONTS: Record<string, string> = {
  'zh-TW': 'Noto Sans TC',
  'zh-CN': 'Noto Sans SC',
  ja: 'Noto Sans JP',
  ko: 'Noto Sans KR',
};

export function isCjkLanguage(lang: string): boolean {
  return Object.hasOwn(CJK_FONTS, lang);
}

/** The stylesheet for a font, plus the Noto Sans for `cjkLang` when that is a CJK language. */
export function fontUrl(font: FontChoice, cjkLang: string | null = null): string | null {
  if (font === 'system') return null;
  const cjk =
    cjkLang && isCjkLanguage(cjkLang)
      ? `&family=${CJK_FONTS[cjkLang].replace(/ /g, '+')}:wght@400;500;600;700`
      : '';
  return `https://fonts.googleapis.com/css2?${FONTS[font].query}${cjk}&display=swap`;
}

function inGamut(lch: Lch) {
  return lchToRgb(lch).every((v) => v >= -0.0005 && v <= 1.0005);
}

/** The most chroma sRGB can show at this lightness and hue. */
function maxChroma(l: number, h: number) {
  let [lo, hi] = [0, 0.4];
  for (let i = 0; i < 24; i++) {
    const mid = (lo + hi) / 2;
    if (inGamut([l, mid, h])) lo = mid;
    else hi = mid;
  }
  return lo;
}

// Tailwind's palette, which the presets' hand-picked pairs follow: for the hue of each 600 shade,
// the lightness and hue of its 400 shade. Dark mode lifts an accent to its 400.
const DARK_STEPS: [hue600: number, l400: number, hue400: number][] = [
  [27.3, 0.704, 22.2],
  [41.1, 0.75, 55.9],
  [58.3, 0.828, 84.4],
  [75.8, 0.852, 91.9],
  [131.7, 0.841, 128.9],
  [149.2, 0.792, 151.7],
  [163.2, 0.765, 163.2],
  [184.7, 0.777, 181.9],
  [221.7, 0.789, 211.5],
  [242, 0.746, 232.7],
  [262.9, 0.707, 254.6],
  [277, 0.673, 276.9],
  [293, 0.702, 293.5],
  [302.3, 0.714, 305.5],
  [322.9, 0.74, 322.2],
  [360.6, 0.718, 349.8],
  [377.6, 0.712, 373.4],
];

/** The 400 lightness and hue for any hue, read between the two nearest steps. */
function darkStep(h: number): [l: number, h: number] {
  const steps = [-360, 0, 360].flatMap((o) =>
    DARK_STEPS.map(([a, l, b]): [number, number, number] => [a + o, l, b + o]),
  );
  const i = steps.findIndex(([a]) => a > h);
  const [[h0, l0, b0], [h1, l1, b1]] = [steps[i - 1], steps[i]];
  const k = (h - h0) / (h1 - h0);
  return [l0 + (l1 - l0) * k, b0 + (b1 - b0) * k];
}

/** The accent for dark mode: lifted to the lightness and hue of its Tailwind 400 shade, as
 *  saturated relative to what sRGB can show as the picked colour was. A near-grey accent becomes
 *  a light grey; an accent that is already light keeps its hue. */
export function deriveAccentDark(accent: string): string {
  const [l, c, h] = toOklch(accent);
  const [stepL, stepH] = darkStep(h);
  // How much the accent behaves as a colour rather than a grey.
  const hued = Math.min(1, Math.max(0, (c - 0.04) / 0.05));
  const target = Math.max(l, 0.92 + (stepL - 0.92) * hued);
  const lift = Math.min(1, Math.max(0, (stepL - l) / 0.15));
  const turn = ((((stepH - h + 540) % 360) - 180) * hued * lift + h + 360) % 360;
  const relative = c > 0 ? Math.min(1, c / maxChroma(l, h)) : 0;
  return fromOklch([target, relative * maxChroma(target, turn), turn]);
}

export interface Derived {
  light: Record<string, string>;
  dark: Record<string, string>;
  /** User-picked colours the contrast guard had to move, as `token → [picked, used]`. */
  adjusted: { mode: Mode; token: 'accent' | 'link'; from: string; to: string }[];
  /** Text pairs still under 4.5:1 after the guard (empty unless a colour has no room left). */
  failures: string[];
}

function deriveMode(theme: CustomTheme, mode: Mode, out: Derived): Record<string, string> {
  const dark = mode === 'dark';
  const pickedAccent = dark ? (theme.accentDark ?? deriveAccentDark(theme.accent)) : theme.accent;
  const pickedLink = dark
    ? (theme.linkDark ?? (theme.link ? deriveAccentDark(theme.link) : pickedAccent))
    : (theme.link ?? pickedAccent);
  const [, accentChroma, accentHue] = toOklch(theme.accent);
  const [tintC, tintH] =
    theme.background === 'accent'
      ? [Math.min(0.009, accentChroma), accentHue]
      : TINTS[theme.background];

  const v: Record<string, string> = {};
  for (const [role, [l, k]] of Object.entries(NEUTRALS[mode])) {
    v[role] = fromOklch([l, tintC * k, tintH]);
  }
  const surfaces = [v.bg, v.side, v.surface, v.sunken, v.sel];
  v['ink-2'] = guard(v['ink-2'], surfaces, dark);
  v['ink-3'] = guard(v['ink-3'], surfaces, dark);
  v['sel-ink'] = v.ink;

  // Primary: keep the picked colour and choose the label that reads on it; move the fill only
  // when neither label reaches 4.5:1.
  const darkLabel = fromOklch([0.17, tintC, tintH]);
  let pri = pickedAccent;
  const label = (p: string) =>
    contrast(p, '#ffffff') >= contrast(p, darkLabel) ? '#ffffff' : darkLabel;
  while (contrast(pri, label(pri)) < MIN_CONTRAST) {
    const [l, c, h] = toOklch(pri);
    const next = fromOklch([label(pri) === '#ffffff' ? l - 0.01 : l + 0.01, c, h]);
    if (next === pri) break;
    pri = next;
  }
  v.pri = pri;
  v['on-pri'] = label(pri);
  if (pri !== pickedAccent)
    out.adjusted.push({ mode, token: 'accent', from: pickedAccent, to: pri });

  const [, lc, lh] = toOklch(pickedLink);
  // A selected row fills a large area, so the tint stays as quiet in dark as in light (Clean dark's is about 0.03).
  v['accent-bg'] = dark
    ? fromOklch([0.25, Math.min(lc * 0.25, 0.03), lh])
    : fromOklch([0.955, Math.min(lc * 0.25, 0.03), lh]);
  v.accent = guard(pickedLink, [...surfaces, v['accent-bg']], dark);
  if (v.accent !== pickedLink)
    out.adjusted.push({ mode, token: 'link', from: pickedLink, to: v.accent });

  for (const [name, [lt, lb, dt, db]] of Object.entries(STATUS)) {
    v[`${name}-bg`] = fromOklch(dark ? db : lb);
    v[name] = guard(fromOklch(dark ? dt : lt), [v.bg, v.surface, v[`${name}-bg`]], dark);
  }

  const checks: [string, string, string[]][] = [
    ['ink', v.ink, surfaces],
    ['ink-2', v['ink-2'], surfaces],
    ['ink-3', v['ink-3'], surfaces],
    ['accent', v.accent, [...surfaces, v['accent-bg']]],
    ['on-pri', v['on-pri'], [v.pri]],
    ...(['ok', 'warn', 'bad'] as const).map(
      (s) => [s, v[s], [v.bg, v.surface, v[`${s}-bg`]]] as [string, string, string[]],
    ),
  ];
  for (const [token, fg, bgs] of checks) {
    if (bgs.some((b) => contrast(fg, b) < MIN_CONTRAST)) out.failures.push(`${mode} --${token}`);
  }

  const clean = theme.base === 'clean';
  v['line-soft'] = v.line;
  v.sep = `1px solid ${v.sep}`;
  v.frame = `${clean && theme.borders === 'bold' ? BOLD_BORDER : '1px'} solid ${v.frame}`;
  if (clean && theme.depth !== 'default') {
    v['sh-box'] = DEPTH[theme.depth][mode];
    // Glass cards list their rim before this shadow, where `none` would be invalid.
    v['gs-shadow'] = theme.depth === 'flat' ? '0 0 transparent' : v['sh-box'];
  }
  v.shell = v.side;
  v['glass-bar'] = alpha(v.bg, 0.74);
  v['glass-menu'] = dark
    ? alpha(fromOklch([0.26, tintC * 1.5, tintH]), 0.76)
    : alpha(v.surface, 0.76);
  return v;
}

export function deriveTheme(theme: CustomTheme): Derived {
  const out: Derived = { light: {}, dark: {}, adjusted: [], failures: [] };
  out.light = deriveMode(theme, 'light', out);
  out.dark = deriveMode(theme, 'dark', out);
  return out;
}

// Playful keeps its ink borders, offset shadows, dot grid and pastels; a custom theme on it
// recolours surfaces, text and actions only.
const PLAYFUL_KEEPS = new Set([
  'line',
  'line-2',
  'line-soft',
  'sep',
  'frame',
  'shell',
  'glass-bar',
  'glass-menu',
]);

function shared(theme: CustomTheme): Record<string, string> {
  const v: Record<string, string> = {};
  if (theme.corners !== 'default') Object.assign(v, CORNERS[theme.corners]);
  if (theme.font !== 'system') {
    const f = FONTS[theme.font];
    v.f = `'${f.sans}', ${SYSTEM_FONT}`;
    if (f.mono) v.fm = `'${f.mono}', ${SYSTEM_MONO}`;
  }
  if (theme.base === 'clean') {
    if (theme.headings === 'tight') {
      v.h1 = '600 32px/1.1 var(--fh)';
      v['h1-track'] = '-0.035em';
    }
    if (theme.borders === 'bold') v.bw = BOLD_BORDER;
    if (theme.icons !== 'default') v.isw = ICONS[theme.icons];
    if (theme.glass === 'solid') Object.assign(v, SOLID);
  }
  return v;
}

function block(selector: string, vars: Record<string, string>) {
  const body = Object.entries(vars)
    .map(([k, val]) => `  --${k}: ${val};`)
    .join('\n');
  return `${selector} {\n${body}\n}`;
}

/** The <html> attributes a style needs; `null` removes one. `data-theme` names the base style,
 *  `data-custom` turns on the generated stylesheet, `data-layout` opts out of Clean's inset panel,
 *  and `data-glass` frosts cards, lists and the sidebar too. */
export function rootAttributes(
  style: CustomBase | 'custom',
  custom: CustomTheme,
): Record<'data-theme' | 'data-custom' | 'data-layout' | 'data-glass', string | null> {
  const isCustom = style === 'custom';
  const base = isCustom ? custom.base : style;
  return {
    'data-theme': base === 'playful' ? 'playful' : null,
    'data-custom': isCustom ? '' : null,
    'data-layout': isCustom && base === 'clean' && custom.layout === 'full' ? 'full' : null,
    'data-glass': isCustom && base === 'clean' && custom.glass === 'surfaces' ? 'surfaces' : null,
  };
}

// Each selector outranks the base style's rule for the same mode (`:root:not([data-theme=playful])`
// is 0,2,0 and its .dark rule 0,3,0), so the result does not depend on stylesheet order.
const LIGHT = ':root[data-custom]:not(.dark)';
const DARK = ':root[data-custom][data-custom].dark';

/** The stylesheet that applies a theme, with the attributes from `rootAttributes` on <html>. The
 *  font loads separately from `fontUrl`, so an unreachable Google Fonts never holds up paint. */
/** A mode's derived colours as the theme sets them: a Playful base keeps its own borders. */
export function styleVars(theme: CustomTheme, vars: Record<string, string>) {
  return theme.base === 'playful'
    ? Object.fromEntries(Object.entries(vars).filter(([k]) => !PLAYFUL_KEEPS.has(k)))
    : vars;
}

/** The custom properties a theme sets in each mode, without the leading `--`: what `themeCss`
 *  writes, and what an embedding app paints its own window with. */
export function themeVars(theme: CustomTheme): Record<'light' | 'dark', Record<string, string>> {
  const d = deriveTheme(theme);
  const common = shared(theme);
  return {
    light: { ...styleVars(theme, d.light), ...common },
    dark: { ...styleVars(theme, d.dark), ...common },
  };
}

export function themeCss(theme: CustomTheme): string {
  const vars = themeVars(theme);
  const parts: string[] = [];
  parts.push(block(LIGHT, vars.light));
  parts.push(block(DARK, vars.dark));
  if (theme.base === 'clean') {
    // Reduce transparency: the glass turns back into the solid colour it floats over.
    parts.push(
      `@media (prefers-reduced-transparency: reduce) {\n${block(LIGHT, { 'glass-menu': 'var(--surface)', 'glass-bar': 'var(--bg)' })}\n${block(DARK, { 'glass-menu': 'var(--surface)', 'glass-bar': 'var(--bg)' })}\n}`,
    );
    if (theme.buttons === 'raised') {
      const light = `{\n  background-image: linear-gradient(rgba(255, 255, 255, 0.1), transparent);\n  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.2), 0 1px 2px color-mix(in srgb, var(--pri) 35%, transparent), 0 3px 8px -3px color-mix(in srgb, var(--pri) 40%, transparent);\n}`;
      const dark = `{\n  background-image: linear-gradient(transparent, rgba(0, 0, 0, 0.14));\n  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.3), 0 1px 4px color-mix(in srgb, var(--pri) 35%, transparent);\n}`;
      // The editor's preview can show the other mode; its own mode wins there (same specificity
      // as DARK, and later).
      parts.push(
        `${LIGHT} .ss-btn.pri ${light}`,
        `${DARK} .ss-btn.pri ${dark}`,
        `:root[data-custom] .ss-mini[data-mode='light'] .ss-btn.pri ${light}`,
        `:root[data-custom] .ss-mini[data-mode='dark'] .ss-btn.pri ${dark}`,
      );
    }
    if (theme.texture === 'dots') {
      // On the page and the preview's page, not the shell around them. The doubled attribute
      // outranks the inset panel's solid background whichever stylesheet loads first. Light dots
      // sit halfway between the two line colours, the softer alone barely shows; on a dark page
      // the softer one is enough.
      const dots = (c: string) =>
        `{\n  background-image: radial-gradient(${c} 1px, transparent 1px);\n  background-size: 24px 24px;\n}`;
      const light = dots(`color-mix(in oklab, ${vars.light.line}, ${vars.light['line-2']})`);
      const dark = dots(vars.dark.line);
      parts.push(
        `:root[data-custom][data-custom]:not(.dark) .ss-main ${light}`,
        `${DARK} .ss-main ${dark}`,
        `:root[data-custom] .ss-mini[data-mode='light'] > .pn ${light}`,
        `:root[data-custom] .ss-mini[data-mode='dark'] > .pn ${dark}`,
      );
    }
  }
  if (theme.cjk && theme.font !== 'system') {
    const sans = FONTS[theme.font].sans;
    for (const [lang, family] of Object.entries(CJK_FONTS)) {
      parts.push(
        `${LIGHT}:lang(${lang}),\n${DARK}:lang(${lang}) {\n  --f: '${sans}', '${family}', ${SYSTEM_FONT};\n}`,
      );
    }
  }
  return parts.join('\n');
}
