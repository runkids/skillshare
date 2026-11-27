import { useCallback, useEffect, useEffectEvent, useMemo, useRef, useState } from 'react';
import { useLocation, useSearchParams } from 'react-router-dom';
import { ArrowRight, Check, LayoutDashboard, Palette, Puzzle, Target } from 'lucide-react';
import { useTheme, type ResolvedMode } from '../context/ThemeContext';
import { useI18n, useT } from '../i18n';
import { useToast } from './Toast';
import { segSlide } from '../lib/segSlide';
import { BASE_PATH } from '../lib/basePath';
import { downloadJson } from '../lib/downloadJson';
import { useThemeLabel } from '../hooks/useThemeLabel';
import {
  ACCENT_SWATCHES,
  BACKGROUNDS,
  PRESETS,
  decodeShare,
  deriveAccentDark,
  deriveTheme,
  encodeShare,
  parseCustomTheme,
  parseHex,
  editedFrom,
  isCjkLanguage,
  presetOf,
  serializeCustomTheme,
  styleVars,
  type Background,
  type CustomTheme,
  type PresetId,
} from '../lib/customTheme';
import Switch from './Switch';

function Choice<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T | null;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
}) {
  return (
    <div
      ref={segSlide}
      className="ss-seg max-w-full shrink-0 overflow-x-auto"
      role="radiogroup"
      aria-label={label}
    >
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          className={value === o.value ? 'on' : ''}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

/** A settings row; `stacked` puts the controls on their own line under the label, for long sets. */
function Row({
  label,
  hint,
  stacked,
  children,
}: {
  label: string;
  hint?: string;
  stacked?: boolean;
  children: React.ReactNode;
}) {
  return (
    // On a phone the controls go under the label instead of squeezing it.
    <div className={stacked ? 'ss-setrow !block' : 'ss-setrow max-sm:flex-wrap max-sm:gap-y-3'}>
      <div className={stacked ? 'l mb-3' : 'l max-sm:basis-full'}>
        <b>{label}</b>
        {hint && <span>{hint}</span>}
      </div>
      {children}
    </div>
  );
}

/** A hex field that applies a full six-digit colour as it is typed, and a short `#abc` on blur,
 *  so typing `#4f46e5` is not cut short at `#4f4`. */
function HexField({
  value,
  label,
  onChange,
}: {
  value: string;
  label: string;
  onChange: (hex: string) => void;
}) {
  const [text, setText] = useState(value);
  const [shown, setShown] = useState(value);
  if (shown !== value) {
    setShown(value);
    setText(value);
  }
  return (
    <span className="ss-inp sm !h-[30px] w-[112px] shrink-0 font-mono">
      <input
        type="color"
        aria-label={`${label} (picker)`}
        value={value || '#000000'}
        className="ss-swatch"
        onChange={(e) => onChange(e.target.value)}
      />
      <input
        aria-label={label}
        value={text}
        maxLength={7}
        spellCheck={false}
        className="min-w-0 flex-1 bg-transparent text-xs outline-none"
        onChange={(e) => {
          setText(e.target.value);
          const typed = e.target.value.trim();
          if (/^#?[0-9a-f]{6}$/i.test(typed)) onChange(parseHex(typed)!);
        }}
        onBlur={() => {
          const hex = parseHex(text.trim());
          if (hex && hex !== value) onChange(hex);
          else setText(value);
        }}
      />
    </span>
  );
}

/** Every preset as a small page in the current mode: sidebar, title, a line of text, a button. */
function PresetGrid({
  value,
  modified,
  mode,
  onPick,
}: {
  value: PresetId | null;
  modified: boolean;
  mode: ResolvedMode;
  onPick: (id: PresetId) => void;
}) {
  const t = useT();
  const thumbs = useMemo(
    () =>
      (Object.keys(PRESETS) as PresetId[]).map((id) => ({
        id,
        c: deriveTheme(PRESETS[id])[mode],
      })),
    [mode],
  );
  return (
    <div className="ss-presets" role="radiogroup" aria-label={t('settings.custom.preset')}>
      {thumbs.map(({ id, c }) => {
        const p: CustomTheme = PRESETS[id];
        const on = value === id;
        return (
          <button
            key={id}
            type="button"
            role="radio"
            aria-checked={on}
            className={on ? 'ss-preset on' : 'ss-preset'}
            onClick={() => onPick(id)}
          >
            <span className="th" style={{ background: c.bg }} aria-hidden>
              <span className="sd" style={{ background: c.side, borderColor: c.line }} />
              <span className="pg">
                <span className="ln w-[60%]" style={{ background: c.ink }} />
                <span className="ln w-[85%]" style={{ background: c['ink-3'] }} />
                <span className="bt" style={{ background: c.pri }} />
              </span>
            </span>
            <span className="nm">
              {p.name}
              {on && modified ? (
                <span className="ss-tag">{t('settings.custom.modified')}</span>
              ) : (
                <small>{t(`settings.appearance.style.${p.base}`)}</small>
              )}
            </span>
          </button>
        );
      })}
    </div>
  );
}

/** The parts of the preview an option changes; each carries the names in its `data-part`. */
type Part =
  | 'all'
  | 'accent'
  | 'pri'
  | 'link'
  | 'bg'
  | 'shape'
  | 'font'
  | 'text'
  | 'heading'
  | 'layout'
  | 'button'
  | 'icon';
type Hint = { part: Part; n: number };
const EASE_OUT = 'cubic-bezier(0.23, 1, 0.32, 1)';

/** A small copy of the dashboard drawn with the live classes, so every option shows as it changes.
 *  It can show the other mode: the derived colours for that mode are set on it as variables. */
function MiniPreview({
  mode,
  colors,
  tone,
  hint,
  onMode,
}: {
  mode: ResolvedMode;
  colors?: Record<string, string>;
  /** The accent of the mode shown, for the hint. */
  tone: string;
  hint: Hint | null;
  onMode: (mode: ResolvedMode) => void;
}) {
  const t = useT();
  const root = useRef<HTMLDivElement>(null);
  const running = useRef<Animation[]>([]);
  // Marks the parts an option just changed, once: a wash of the accent behind changed text, and a
  // soft glow spreading from anything with a shape (the whole preview, the page panel, buttons,
  // tags). The glow is a shadow added to the element's own, so it follows its corners as they
  // change. The page panel's glow sits inside it, where the preview's edge cannot clip it; flush,
  // the panel has no corners to show.
  const show = useEffectEvent((part: Part) => {
    const box = root.current;
    if (!box) return;
    for (const a of running.current) a.cancel();
    const still = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
    const glow = (spread: number, alpha: number, inset: boolean) => ({
      boxShadow: `${inset ? 'inset ' : ''}0 0 0 ${still ? 3 : spread}px color-mix(in srgb, ${tone} ${alpha}%, transparent)`,
    });
    running.current = [box, ...box.querySelectorAll<HTMLElement>('[data-part]')]
      .filter((el) => el.dataset.part?.split(' ').includes(part))
      .filter(
        (el) => !el.matches('.pn') || parseFloat(getComputedStyle(el).borderTopLeftRadius) > 0,
      )
      .map((el) => {
        const wash = `color-mix(in srgb, ${tone} 22%, ${getComputedStyle(el).backgroundColor})`;
        if (el.matches('.ss-h1, p, .ss-more')) {
          return el.animate?.(
            [
              { backgroundColor: wash },
              { backgroundColor: wash, offset: 0.3, easing: EASE_OUT },
              {},
            ],
            { duration: 1000 },
          );
        }
        const inset = el.matches('.pn');
        return el.animate?.([{ ...glow(1, 30, inset), easing: EASE_OUT }, glow(6, 0, inset)], {
          duration: 900,
          composite: 'add',
        });
      })
      .filter((a): a is Animation => !!a);
  });
  useEffect(() => {
    if (hint) show(hint.part);
  }, [hint]);
  const style = colors
    ? ({
        ...Object.fromEntries(Object.entries(colors).map(([k, v]) => [`--${k}`, v])),
        colorScheme: mode,
      } as React.CSSProperties)
    : undefined;
  return (
    <div
      ref={root}
      className="ss-mini relative"
      data-mode={mode}
      style={style}
      data-part="all bg layout shape"
    >
      <div className="mode">
        <Choice
          label={t('settings.custom.previewMode')}
          value={mode}
          options={[
            { value: 'light', label: t('theme.light') },
            { value: 'dark', label: t('theme.dark') },
          ]}
          onChange={onMode}
        />
      </div>
      <div inert className="sd">
        <span className="ss-nv" data-part="icon">
          <LayoutDashboard size={15} />
          {t('layout.nav.dashboard')}
        </span>
        <span className="ss-nv on" data-part="icon">
          <Puzzle size={15} />
          {t('layout.nav.skills')}
        </span>
        <span className="ss-nv" data-part="icon">
          <Target size={15} />
          {t('layout.nav.targets')}
        </span>
      </div>
      <div inert className="pn" data-part="shape">
        <div className="ss-h1 max-w-[calc(100%-9rem)] self-start truncate" data-part="font heading">
          {t('layout.nav.skills')}
        </div>
        <p className="self-start text-[13px] text-ink-2" data-part="font text">
          {t('settings.custom.previewHint')}
        </p>
        <div className="flex flex-wrap items-center gap-3">
          <span className="ss-st ok">{t('targets.state.synced')}</span>
          <span className="ss-tag" data-part="shape">
            merge
          </span>
          <a className="ss-more" href="#preview" data-part="accent link">
            {t('settings.custom.sampleLink')}
          </a>
          <span className="ml-auto flex gap-2">
            <button type="button" className="ss-btn sm" data-part="shape">
              {t('settings.custom.sampleButton')}
            </button>
            <button type="button" className="ss-btn pri sm" data-part="accent pri shape button">
              {t('settings.custom.samplePrimary')}
            </button>
          </span>
        </div>
      </div>
    </div>
  );
}

function themeLink(theme: CustomTheme) {
  return `${window.location.origin}${BASE_PATH}/appearance?custom=${encodeShare(theme)}`;
}

function download(theme: CustomTheme, label: string) {
  const name = label.toLowerCase().replace(/[^a-z0-9]+/g, '-');
  downloadJson(
    `skillshare-theme-${name.replace(/^-|-$/g, '') || 'custom'}.json`,
    JSON.parse(serializeCustomTheme(theme)),
  );
}

/** A theme someone shared as a `?custom=` link; applied only when the user accepts it. */
export function SharedThemeNote() {
  const t = useT();
  const { setStyle, setCustomTheme } = useTheme();
  const [params, setParams] = useSearchParams();
  const label = useThemeLabel();
  const shared = useMemo(() => decodeShare(params.get('custom') ?? ''), [params]);
  if (!params.get('custom')) return null;
  const clear = () =>
    setParams(
      (p) => {
        p.delete('custom');
        return p;
      },
      { replace: true },
    );
  if (!shared) {
    return (
      <div className="ss-note bad mb-3">
        <span className="flex-1">{t('settings.custom.sharedInvalid')}</span>
        <button type="button" className="ss-btn sm" onClick={clear}>
          {t('common.close')}
        </button>
      </div>
    );
  }
  const name = label(shared);
  return (
    <div className="ss-note inf mb-3">
      <Palette size={16} />
      <span className="flex-1">{t('settings.custom.shared', { name })}</span>
      <button type="button" className="ss-btn sm" onClick={clear}>
        {t('settings.custom.dismiss')}
      </button>
      <button
        type="button"
        className="ss-btn pri sm"
        onClick={() => {
          setCustomTheme(shared);
          setStyle('custom');
          clear();
        }}
      >
        {t('settings.custom.useShared')}
      </button>
    </div>
  );
}

export default function CustomThemeSettings() {
  const t = useT();
  const { locale } = useI18n();
  const { toast } = useToast();
  const { customTheme: theme, setCustomTheme, resolvedMode } = useTheme();
  // The preview follows the page's mode until the user picks the other one to look at.
  const [previewMode, setPreviewMode] = useState<ResolvedMode>(resolvedMode);
  const [pageMode, setPageMode] = useState(resolvedMode);
  if (pageMode !== resolvedMode) {
    setPageMode(resolvedMode);
    setPreviewMode(resolvedMode);
  }
  const fileInput = useRef<HTMLInputElement>(null);
  // The preview is marked stuck once it pins to the top, which is when its sentinel has left
  // through the top.
  const [stuck, setStuck] = useState(false);
  const sentinel = useCallback((el: HTMLDivElement | null) => {
    if (!el || typeof IntersectionObserver === 'undefined') return;
    const io = new IntersectionObserver(([e]) => setStuck(!e.isIntersecting));
    io.observe(el);
    return () => io.disconnect();
  }, []);
  const section = useRef<HTMLDivElement>(null);
  const { hash, key } = useLocation();
  // The theme menu's "Edit colors" opens this section, also when the URL already has the hash.
  useEffect(() => {
    if (hash === '#custom-theme') section.current?.scrollIntoView({ block: 'start' });
  }, [hash, key]);
  const derived = useMemo(() => deriveTheme(theme), [theme]);
  // What each background would look like with the rest of this theme, for the chips.
  const tints = useMemo(
    () =>
      Object.fromEntries(
        BACKGROUNDS.map((b) => [b, deriveTheme({ ...theme, background: b })[resolvedMode]]),
      ) as Record<Background, Record<string, string>>,
    [theme, resolvedMode],
  );
  const preset = presetOf(theme);
  const label = useThemeLabel();
  const name = label(theme);
  const from = editedFrom(theme);
  const [hint, setHint] = useState<Hint | null>(null);
  const snap = (part: Part) => setHint((h) => ({ part, n: (h?.n ?? 0) + 1 }));
  /** Applies an option and marks the part of the preview it changed. */
  const change = (patch: Partial<CustomTheme>, part: Part) => {
    setCustomTheme({ ...theme, ...patch });
    snap(part);
  };
  // A preset's dark colours were chosen for its own accent and link, so a new pick drops them;
  // a dark accent the user set stays.
  const presetDark = (key: 'accentDark' | 'linkDark') => {
    const light = key === 'accentDark' ? 'accent' : 'link';
    const pinned = Object.values<CustomTheme>(PRESETS).some(
      (p) => p[key] && p[key] === theme[key] && p[light] === theme[light],
    );
    return pinned ? { [key]: undefined } : {};
  };
  const adjusted = (token: 'accent' | 'link') =>
    derived.adjusted.find((a) => a.token === token && a.mode === resolvedMode);
  const accentAdjust = adjusted('accent');
  const linkAdjust = adjusted('link');
  const clean = theme.base === 'clean';
  const derive = !theme.accentDark;
  // Playful keeps Kalam for headings.
  const fontPart: Part = clean ? 'font' : 'text';
  // With a link colour of its own, the accent no longer reaches the link.
  const accentPart: Part = theme.link ? 'pri' : 'accent';

  const importFile = async (file: File) => {
    let next: CustomTheme | null = null;
    try {
      next = parseCustomTheme(JSON.parse(await file.text()));
    } catch {
      next = null;
    }
    if (!next) {
      toast(t('settings.custom.importFailed'), 'error');
      return;
    }
    setCustomTheme(next);
    snap('all');
    toast(t('settings.custom.imported', { name: label(next) }), 'success');
  };

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(themeLink(theme));
      toast(t('settings.custom.linkCopied'), 'success');
    } catch {
      toast(t('settings.custom.copyFailed'), 'error');
    }
  };

  return (
    // One block, so the preview stays in view through Advanced as well.
    <div id="custom-theme" ref={section} className="scroll-mt-6">
      <section>
        <div className="ss-sec">
          <h2 className="ss-h2">{t('settings.custom.title')}</h2>
          <span className="ss-cnt">{name}</span>
          {derived.failures.length > 0 && (
            <span className="ss-st warn ml-auto">{t('settings.custom.fails')}</span>
          )}
        </div>
        {/* Stuck to the very top, with page colour above and to the sides (Playful's offset shadows
            reach past the box), so rows never show through. */}
        {/* The sentinel reaches a screen up from the preview, so it still counts as in view while
            the preview is below the fold, and a jump straight to the pinned position crosses it. It
            ends level with the sticky box's top (its -mt-3), so it leaves just as the box pins. */}
        <div className="relative" aria-hidden>
          <div
            ref={sentinel}
            className="pointer-events-none absolute bottom-0 h-screen w-px md:bottom-3"
          />
        </div>
        <div
          className={`ss-ministick z-20 mb-3 md:sticky md:top-0 md:-mt-3 md:pt-3 ${stuck ? 'stuck' : ''}`}
        >
          <MiniPreview
            mode={previewMode}
            colors={
              previewMode === resolvedMode ? undefined : styleVars(theme, derived[previewMode])
            }
            tone={derived[previewMode].accent}
            hint={hint}
            onMode={setPreviewMode}
          />
        </div>
        <div className="ss-list">
          <Row stacked label={t('settings.custom.preset')} hint={t('settings.custom.presetHint')}>
            <PresetGrid
              value={preset ?? from}
              modified={!!from}
              mode={resolvedMode}
              onPick={(id) => {
                setCustomTheme({ ...PRESETS[id], from: id });
                snap('all');
              }}
            />
          </Row>
          <Row label={t('settings.custom.base')} hint={t('settings.custom.baseHint')}>
            <Choice
              label={t('settings.custom.base')}
              value={theme.base}
              options={[
                { value: 'clean', label: t('settings.appearance.style.clean') },
                { value: 'playful', label: t('settings.appearance.style.playful') },
              ]}
              onChange={(base) => change({ base }, 'all')}
            />
          </Row>
          <Row
            stacked
            label={t('settings.custom.accent')}
            hint={
              accentAdjust
                ? t('settings.custom.adjusted', { color: accentAdjust.to })
                : t('settings.custom.accentHint')
            }
          >
            <div className="flex flex-wrap items-center gap-2">
              {ACCENT_SWATCHES.map((c) => (
                <button
                  key={c}
                  type="button"
                  aria-label={t('settings.custom.swatch', { color: c })}
                  aria-pressed={theme.accent === c}
                  // Every swatch is dark enough for a white check.
                  className="grid size-6 place-items-center rounded-full text-white shadow-[0_0_0_1px_var(--line-2)] aria-pressed:outline-1 aria-pressed:outline-offset-2 aria-pressed:outline-ink-3"
                  style={{ background: c }}
                  onClick={() => change({ accent: c, ...presetDark('accentDark') }, accentPart)}
                >
                  {theme.accent === c && <Check size={14} aria-hidden />}
                </button>
              ))}
              <HexField
                value={theme.accent}
                label={t('settings.custom.accentHex')}
                onChange={(accent) => change({ accent, ...presetDark('accentDark') }, accentPart)}
              />
            </div>
          </Row>
          <Row
            stacked
            label={t('settings.custom.background')}
            hint={t('settings.custom.backgroundHint')}
          >
            <div
              className="flex flex-wrap gap-2"
              role="radiogroup"
              aria-label={t('settings.custom.background')}
            >
              {BACKGROUNDS.map((b) => (
                <button
                  key={b}
                  type="button"
                  role="radio"
                  aria-checked={theme.background === b}
                  className={theme.background === b ? 'ss-tint on' : 'ss-tint'}
                  onClick={() => change({ background: b }, 'bg')}
                >
                  <span
                    className="ss-tint-sw"
                    style={{
                      background: `linear-gradient(135deg, ${tints[b].side} 50%, ${tints[b].sel} 50%)`,
                    }}
                    aria-hidden
                  />
                  {t(`settings.custom.background.${b}`)}
                </button>
              ))}
            </div>
          </Row>
          {clean && (
            <Row label={t('settings.custom.texture')} hint={t('settings.custom.textureHint')}>
              <Choice
                label={t('settings.custom.texture')}
                value={theme.texture}
                options={(['none', 'dots'] as const).map((v) => ({
                  value: v,
                  label: t(`settings.custom.texture.${v}`),
                }))}
                onChange={(texture) => change({ texture }, 'bg')}
              />
            </Row>
          )}
          <Row label={t('settings.custom.corners')}>
            <Choice
              label={t('settings.custom.corners')}
              value={theme.corners}
              options={(['sharp', 'default', 'round'] as const).map((v) => ({
                value: v,
                label: t(`settings.custom.corners.${v}`),
              }))}
              onChange={(corners) => change({ corners }, 'shape')}
            />
          </Row>
          {clean && (
            <Row label={t('settings.custom.depth')} hint={t('settings.custom.depthHint')}>
              <Choice
                label={t('settings.custom.depth')}
                value={theme.depth}
                options={(['flat', 'default', 'lifted'] as const).map((v) => ({
                  value: v,
                  label: t(`settings.custom.depth.${v}`),
                }))}
                onChange={(depth) => change({ depth }, 'shape')}
              />
            </Row>
          )}
          <Row label={t('settings.custom.font')} hint={t('settings.custom.fontHint')}>
            <Choice
              label={t('settings.custom.font')}
              value={theme.font}
              options={[
                { value: 'system', label: t('settings.custom.font.system') },
                { value: 'inter', label: 'Inter' },
                { value: 'geist', label: 'Geist' },
                { value: 'plex', label: 'IBM Plex' },
                { value: 'dm-sans', label: 'DM Sans' },
              ]}
              onChange={(font) => change({ font }, fontPart)}
            />
          </Row>
          {theme.font !== 'system' && isCjkLanguage(locale) && (
            <Row label={t('settings.custom.cjk')} hint={t('settings.custom.cjkHint')}>
              <Switch
                checked={!!theme.cjk}
                aria-label={t('settings.custom.cjk')}
                onChange={() => change({ cjk: theme.cjk ? undefined : true }, fontPart)}
              />
            </Row>
          )}
          <Row
            label={t('settings.custom.deriveDark')}
            hint={
              derive
                ? t('settings.custom.deriveDarkHint', { color: derived.dark.pri })
                : t('settings.custom.darkAccentHint')
            }
          >
            <div className="flex items-center gap-3 shrink-0">
              {derive && (
                // The pair the switch derives: this accent, and what dark mode uses for it.
                <span className="flex items-center gap-1.5 text-ink-3" aria-hidden>
                  <i
                    className="size-5 rounded-full shadow-[0_0_0_1px_var(--line-2)]"
                    style={{ background: theme.accent }}
                  />
                  <ArrowRight size={14} />
                  <i
                    className="size-5 rounded-full shadow-[0_0_0_1px_var(--line-2)]"
                    style={{ background: derived.dark.pri }}
                  />
                </span>
              )}
              {!derive && (
                <HexField
                  value={theme.accentDark ?? ''}
                  label={t('settings.custom.darkAccent')}
                  onChange={(accentDark) => {
                    setPreviewMode('dark');
                    change({ accentDark }, accentPart);
                  }}
                />
              )}
              <Switch
                checked={derive}
                aria-label={t('settings.custom.deriveDark')}
                onChange={() => {
                  // Dark colours are what this changes, so the preview shows them.
                  setPreviewMode('dark');
                  change(
                    derive
                      ? { accentDark: deriveAccentDark(theme.accent) }
                      : { accentDark: undefined, linkDark: undefined },
                    accentPart,
                  );
                }}
              />
            </div>
          </Row>
          <Row label={t('settings.custom.share')} hint={t('settings.custom.shareHint')}>
            <div className="flex flex-wrap items-center justify-end gap-2 shrink-0">
              <input
                ref={fileInput}
                type="file"
                accept=".json,application/json"
                className="hidden"
                aria-label={t('settings.custom.import')}
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  e.target.value = '';
                  if (file) void importFile(file);
                }}
              />
              <button
                type="button"
                className="ss-btn sm"
                onClick={() => fileInput.current?.click()}
              >
                {t('settings.custom.import')}
              </button>
              <button type="button" className="ss-btn sm" onClick={() => download(theme, name)}>
                {t('settings.custom.export')}
              </button>
              <button type="button" className="ss-btn sm" onClick={() => void copyLink()}>
                {t('settings.custom.copyLink')}
              </button>
            </div>
          </Row>
        </div>
      </section>

      <section className="mt-7">
        <div className="ss-sec">
          <h2 className="ss-h2">{t('settings.custom.advanced')}</h2>
        </div>
        <div className="ss-list">
          {clean && (
            <Row label={t('settings.custom.layout')} hint={t('settings.custom.layoutHint')}>
              <Choice
                label={t('settings.custom.layout')}
                value={theme.layout}
                options={[
                  { value: 'inset', label: t('settings.custom.layout.inset') },
                  { value: 'full', label: t('settings.custom.layout.full') },
                ]}
                onChange={(layout) => change({ layout }, 'layout')}
              />
            </Row>
          )}
          <Row
            label={t('settings.custom.links')}
            hint={
              linkAdjust && theme.link
                ? t('settings.custom.adjusted', { color: linkAdjust.to })
                : t('settings.custom.linksHint')
            }
          >
            <div className="flex items-center gap-2 shrink-0">
              <Choice
                label={t('settings.custom.links')}
                value={theme.link ? 'custom' : 'same'}
                options={[
                  { value: 'same', label: t('settings.custom.links.same') },
                  { value: 'custom', label: t('settings.custom.links.custom') },
                ]}
                onChange={(v) =>
                  change(
                    v === 'same'
                      ? { link: undefined, linkDark: undefined }
                      : { link: derived.light.accent, linkDark: undefined },
                    'link',
                  )
                }
              />
              {theme.link && (
                <HexField
                  value={theme.link}
                  label={t('settings.custom.linkHex')}
                  onChange={(link) => change({ link, ...presetDark('linkDark') }, 'link')}
                />
              )}
            </div>
          </Row>
          {clean && (
            <Row label={t('settings.custom.headings')} hint={t('settings.custom.headingsHint')}>
              <Choice
                label={t('settings.custom.headings')}
                value={theme.headings}
                options={[
                  { value: 'default', label: t('settings.custom.headings.default') },
                  { value: 'tight', label: t('settings.custom.headings.tight') },
                ]}
                onChange={(headings) => change({ headings }, 'heading')}
              />
            </Row>
          )}
          {clean && (
            <Row label={t('settings.custom.buttons')} hint={t('settings.custom.buttonsHint')}>
              <Choice
                label={t('settings.custom.buttons')}
                value={theme.buttons}
                options={[
                  { value: 'flat', label: t('settings.custom.buttons.flat') },
                  { value: 'raised', label: t('settings.custom.buttons.raised') },
                ]}
                onChange={(buttons) => change({ buttons }, 'button')}
              />
            </Row>
          )}
          {clean && (
            <Row label={t('settings.custom.borders')} hint={t('settings.custom.bordersHint')}>
              <Choice
                label={t('settings.custom.borders')}
                value={theme.borders}
                options={(['default', 'bold'] as const).map((v) => ({
                  value: v,
                  label: t(`settings.custom.borders.${v}`),
                }))}
                onChange={(borders) => change({ borders }, 'shape')}
              />
            </Row>
          )}
          {clean && (
            <Row label={t('settings.custom.icons')} hint={t('settings.custom.iconsHint')}>
              <Choice
                label={t('settings.custom.icons')}
                value={theme.icons}
                options={(['light', 'default', 'bold'] as const).map((v) => ({
                  value: v,
                  label: t(`settings.custom.icons.${v}`),
                }))}
                onChange={(icons) => change({ icons }, 'icon')}
              />
            </Row>
          )}
          {clean && (
            <Row label={t('settings.custom.glass')} hint={t('settings.custom.glassHint')}>
              {/* The preview has no menu or toolbar to show this on, so nothing is marked. */}
              <Choice
                label={t('settings.custom.glass')}
                value={theme.glass}
                options={(['frosted', 'surfaces', 'solid'] as const).map((v) => ({
                  value: v,
                  label: t(`settings.custom.glass.${v}`),
                }))}
                onChange={(glass) => setCustomTheme({ ...theme, glass })}
              />
            </Row>
          )}
        </div>
      </section>
    </div>
  );
}
