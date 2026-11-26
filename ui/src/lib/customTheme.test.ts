import { describe, expect, it } from 'vitest';
import {
  BACKGROUNDS,
  PRESETS,
  contrast,
  decodeShare,
  deriveAccentDark,
  deriveTheme,
  editedFrom,
  encodeShare,
  fontUrl,
  parseCustomTheme,
  presetOf,
  rootAttributes,
  serializeCustomTheme,
  styleVars,
  themeCss,
  themeName,
  toOklch,
  type CustomTheme,
} from './customTheme';

const pine: CustomTheme = PRESETS.pine;

describe('parseCustomTheme', () => {
  it('accepts an exported theme unchanged', () => {
    expect(parseCustomTheme(JSON.parse(JSON.stringify(PRESETS.graphite)))).toEqual(
      PRESETS.graphite,
    );
  });

  it('fills missing options with defaults and drops unknown keys', () => {
    expect(parseCustomTheme({ v: 1, accent: '#0F766E', extra: 'x' })).toEqual({
      v: 1,
      name: '',
      base: 'clean',
      accent: '#0f766e',
      background: 'neutral',
      corners: 'default',
      font: 'system',
      layout: 'inset',
      headings: 'default',
      buttons: 'flat',
    });
  });

  it.each([
    ['another version', { v: 2, accent: '#0f766e' }],
    ['a colour that is not hex', { v: 1, accent: 'red; } body { display: none' }],
    ['an unknown option value', { v: 1, accent: '#0f766e', font: "x'); @import url(evil" }],
    ['a bad optional colour', { v: 1, accent: '#0f766e', link: 'url(x)' }],
    ['an array', [1]],
    ['a string', '#0f766e'],
  ])('rejects %s', (_, input) => {
    expect(parseCustomTheme(input)).toBeNull();
  });

  it('keeps the preset a theme started from, and drops one this version does not know', () => {
    expect([
      parseCustomTheme({ v: 1, accent: '#000000', from: 'pine' })?.from,
      parseCustomTheme({ v: 1, accent: '#000000', from: 'fjord' })?.from,
    ]).toEqual(['pine', undefined]);
  });

  it('strips control characters from the name and caps its length', () => {
    expect(
      parseCustomTheme({ v: 1, accent: '#000', name: `a\u0000b${'x'.repeat(60)}` })?.name,
    ).toBe(`ab${'x'.repeat(38)}`);
  });
});

describe('share link', () => {
  it('round-trips a theme, including a non-Latin name', () => {
    const theme = { ...PRESETS.ember, name: '琥珀 Ember' };
    expect(decodeShare(encodeShare(theme))).toEqual(theme);
  });

  it('rejects a value that is not a theme', () => {
    expect(decodeShare('not-base64!')).toBeNull();
  });
});

describe('deriveTheme', () => {
  const cases = Object.values(PRESETS).flatMap((p) =>
    (['clean', 'playful'] as const).map((base) => ({ ...p, base })),
  );

  it.each(cases.map((t) => [`${t.name} on ${t.base}`, t]))(
    'keeps every text colour at 4.5:1 for %s',
    (_, theme) => {
      expect(deriveTheme(theme as CustomTheme).failures).toEqual([]);
    },
  );

  const tinted = BACKGROUNDS.flatMap((background) =>
    (['clean', 'playful'] as const).map((base): CustomTheme => ({ ...pine, background, base })),
  );

  it.each(tinted.map((t) => [`${t.background} on ${t.base}`, t]))(
    'keeps every text colour at 4.5:1 on the %s background',
    (_, theme) => {
      expect(deriveTheme(theme as CustomTheme).failures).toEqual([]);
    },
  );

  it('keeps every swatch-free accent readable: a pale yellow link is darkened, and reported', () => {
    const d = deriveTheme({ ...pine, accent: '#facc15', accentDark: undefined });
    for (const bg of ['bg', 'side', 'surface', 'sunken', 'sel', 'accent-bg']) {
      expect(contrast(d.light.accent, d.light[bg])).toBeGreaterThanOrEqual(4.5);
    }
    expect(d.adjusted).toContainEqual({
      mode: 'light',
      token: 'link',
      from: '#facc15',
      to: d.light.accent,
    });
  });

  it('puts a dark label on a light primary instead of moving the fill', () => {
    const d = deriveTheme({ ...pine, accent: '#facc15' });
    expect(d.light.pri).toBe('#facc15');
    expect(contrast(d.light['on-pri'], d.light.pri)).toBeGreaterThanOrEqual(4.5);
  });

  it('lightens a black dark accent until the dark link reads', () => {
    const d = deriveTheme({ ...pine, accentDark: '#000000' });
    expect(contrast(d.dark.accent, d.dark.bg)).toBeGreaterThanOrEqual(4.5);
  });

  it('uses the pinned dark accent of a preset', () => {
    expect(deriveTheme(PRESETS.indigo).dark.pri).toBe('#9b98ff');
  });

  it('keeps links separate from the primary when a link colour is set', () => {
    const d = deriveTheme(PRESETS.graphite);
    expect([d.light.pri, d.light.accent]).toEqual(['#1e293b', '#3d5a80']);
  });
});

describe('deriveAccentDark', () => {
  // Distance in OKLab; about 0.02 is a just-visible difference.
  const distance = (a: string, b: string) => {
    const ab = (x: string) => {
      const [l, c, h] = toOklch(x);
      const r = (h * Math.PI) / 180;
      return [l, c * Math.cos(r), c * Math.sin(r)];
    };
    const [p, q] = [ab(a), ab(b)];
    return Math.hypot(p[0] - q[0], p[1] - q[1], p[2] - q[2]);
  };

  it('lands near the dark accents the presets were given by hand', () => {
    const pinned = Object.values<CustomTheme>(PRESETS).filter((p) => p.accentDark);
    const gaps = pinned.map((p) => distance(deriveAccentDark(p.accent), p.accentDark!));
    expect(gaps.reduce((a, b) => a + b) / gaps.length).toBeLessThan(0.04);
  });

  it('keeps an accent that is already light enough for dark mode', () => {
    expect(deriveAccentDark('#facc15')).toBe('#facc15');
  });

  it('turns a near-black accent into a light grey', () => {
    const [l, c] = toOklch(deriveAccentDark('#141312'));
    expect([l > 0.88, c < 0.01]).toEqual([true, true]);
  });
});

describe('themeCss', () => {
  it('never writes the theme name into CSS', () => {
    expect(themeCss({ ...pine, name: '} body { color: red }' })).not.toContain('body {');
  });

  it('loads the chosen font from Google Fonts outside the stylesheet, so an offline load does not block paint', () => {
    expect(fontUrl('plex')).toMatch(
      /^https:\/\/fonts\.googleapis\.com\/css2\?family=IBM\+Plex\+Sans/,
    );
    expect(themeCss(PRESETS.graphite)).not.toContain('@import');
  });

  it('adds the Noto Sans for a CJK interface language only when the theme asks for it', () => {
    const plex = { ...PRESETS.graphite, cjk: true };
    expect([
      fontUrl('plex', 'zh-TW')?.includes('Noto+Sans+TC'),
      fontUrl('plex', 'en')?.includes('Noto'),
      themeCss(plex).includes(":lang(ja) {\n  --f: 'IBM Plex Sans', 'Noto Sans JP'"),
      themeCss(PRESETS.graphite).includes('Noto'),
    ]).toEqual([true, false, true, false]);
  });

  it('keeps Playful borders and leaves the inset shell to Clean', () => {
    const css = themeCss({ ...pine, base: 'playful' });
    expect(css).not.toMatch(/--(frame|line|shell):/);
    expect(themeCss(pine)).toMatch(/--shell:/);
  });

  it('raises the primary button only on a Clean base', () => {
    expect(themeCss(PRESETS.indigo)).toContain('.ss-btn.pri');
    expect(themeCss({ ...PRESETS.indigo, base: 'playful' })).not.toContain('.ss-btn.pri');
  });

  it("gives the editor preview the raised button of the mode it shows, not the page's", () => {
    const css = themeCss(PRESETS.indigo);
    const rule = (mode: string) =>
      css.indexOf(`.ss-mini[data-mode='${mode}'] .ss-btn.pri`) > css.indexOf('.dark .ss-btn.pri');
    expect([rule('light'), rule('dark')]).toEqual([true, true]);
  });
});

describe('styleVars', () => {
  it("leaves a Playful base's own borders out of the colours set for a mode", () => {
    const theme: CustomTheme = { ...pine, base: 'playful' };
    expect(Object.keys(styleVars(theme, deriveTheme(theme).dark))).not.toContain('frame');
  });
});

describe('rootAttributes', () => {
  it('marks only the base style for a built-in style', () => {
    expect(rootAttributes('playful', { ...pine, layout: 'full' })).toEqual({
      'data-theme': 'playful',
      'data-custom': null,
      'data-layout': null,
    });
  });

  it('opts a Clean-based custom theme out of the inset panel', () => {
    expect(rootAttributes('custom', { ...pine, layout: 'full' })).toEqual({
      'data-theme': null,
      'data-custom': '',
      'data-layout': 'full',
    });
  });

  it('ignores the layout on a Playful base, which has no inset panel', () => {
    expect(rootAttributes('custom', { ...pine, base: 'playful', layout: 'full' })).toEqual({
      'data-theme': 'playful',
      'data-custom': '',
      'data-layout': null,
    });
  });
});

describe('presets', () => {
  it('recognizes a preset by its options, not its name', () => {
    expect(presetOf({ ...PRESETS.ember, name: 'Mine' })).toBe('ember');
  });

  it('recognizes a preset with the CJK font turned on', () => {
    expect(presetOf(parseCustomTheme({ ...PRESETS.ember, cjk: true })!)).toBe('ember');
  });

  it('recognizes a preset whichever preset it says it started from', () => {
    expect(presetOf({ ...PRESETS.ember, from: 'pine' })).toBe('ember');
  });

  it('recognizes a saved preset whatever order its keys come back in', () => {
    const saved = JSON.parse(serializeCustomTheme(PRESETS.pine));
    expect(presetOf(parseCustomTheme(saved)!)).toBe('pine');
  });

  it('names the preset an edited theme started from, and none while it still matches', () => {
    expect([
      editedFrom({ ...PRESETS.graphite, from: 'graphite', corners: 'round' }),
      editedFrom({ ...PRESETS.graphite, from: 'graphite' }),
    ]).toEqual(['graphite', null]);
  });

  it('names an edited preset generically and keeps an imported name', () => {
    expect(themeName({ ...pine, corners: 'round' })).toBeNull();
    expect(themeName({ ...pine, corners: 'round', name: 'Forest' })).toBe('Forest');
  });
});
