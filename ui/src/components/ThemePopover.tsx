import { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { SunMoon, Sun, Moon, Monitor, Check } from 'lucide-react';
import { useTheme, type Style, type ModePreference } from '../context/ThemeContext';
import { useT } from '../i18n';
import { deriveTheme } from '../lib/customTheme';
import { useThemeLabel } from '../hooks/useThemeLabel';
import { segSlide } from '../lib/segSlide';
import { useMenuPresence } from '../hooks/usePresence';

const styles: { value: Style; label: string }[] = [
  { value: 'clean', label: 'theme.clean' },
  { value: 'playful', label: 'theme.playful' },
  { value: 'custom', label: 'theme.custom' },
];

const modes: { value: ModePreference; label: string; icon: typeof Sun }[] = [
  { value: 'light', label: 'theme.light', icon: Sun },
  { value: 'dark', label: 'theme.dark', icon: Moon },
  { value: 'system', label: 'theme.system', icon: Monitor },
];

type ThumbColors = { bg: string; ink: string; surface: string; line: string; accent: string };

/** Miniature of each style so the choice is visible before picking it. Fixed colours on purpose,
 *  except Custom, which shows the custom theme's own. */
function StyleThumb({ value, custom }: { value: Style; custom: ThumbColors }) {
  const playful = value === 'playful';
  const card = playful
    ? { borderRadius: '3px 9px 4px 8px', border: '1.5px solid #3a342d', background: '#f2dc9c' }
    : { borderRadius: 5, border: '1px solid #d9d6cf', background: '#fff' };
  const last =
    value === 'custom'
      ? { borderRadius: 99, background: custom.accent, width: '50%' }
      : playful
        ? { ...card, background: '#fffdf9' }
        : card;
  return (
    <span
      className="flex flex-col gap-1 p-[7px] h-[54px] rounded-[7px] border border-line-soft"
      style={{
        background: value === 'custom' ? custom.bg : playful ? '#f8f5ef' : '#f7f6f3',
      }}
      aria-hidden="true"
    >
      <span
        className="block w-[34px] h-[5px] rounded-[3px]"
        style={{ background: value === 'custom' ? custom.ink : '#2d2a26' }}
      />
      <span
        className="block h-[9px]"
        style={
          value === 'custom'
            ? { borderRadius: 5, border: `1px solid ${custom.line}`, background: custom.surface }
            : card
        }
      />
      <span className="block h-[9px]" style={last} />
    </span>
  );
}

export default function ThemePopover() {
  const t = useT();
  const label = useThemeLabel();
  const { style, setStyle, modePreference, setModePreference, customTheme, resolvedMode } =
    useTheme();
  const navigate = useNavigate();
  const custom = useMemo(() => deriveTheme(customTheme)[resolvedMode], [customTheme, resolvedMode]);
  const thumb = {
    bg: custom.bg,
    ink: custom.ink,
    surface: custom.surface,
    line: custom.line,
    accent: custom.pri,
  };
  const [open, setOpen] = useState(false);
  const menu = useMenuPresence(open);
  const containerRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  // Return focus to trigger on close
  const prevOpen = useRef(open);
  useEffect(() => {
    if (prevOpen.current && !open) triggerRef.current?.focus();
    prevOpen.current = open;
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  // Focus the selected style on open
  useEffect(() => {
    if (!open || !panelRef.current) return;
    (
      panelRef.current.querySelector('[role="radio"][aria-checked="true"]') as HTMLElement | null
    )?.focus();
  }, [open]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent, group: 'style' | 'mode') => {
      if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
      e.preventDefault();
      const items = group === 'style' ? styles : modes;
      const current = group === 'style' ? style : modePreference;
      const idx = items.findIndex((i) => i.value === current);
      const next =
        e.key === 'ArrowRight'
          ? items[(idx + 1) % items.length]
          : items[(idx - 1 + items.length) % items.length];
      if (group === 'style') setStyle(next.value as Style);
      else setModePreference(next.value as ModePreference);
    },
    [style, modePreference, setStyle, setModePreference],
  );

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={triggerRef}
        type="button"
        onClick={() => setOpen(!open)}
        className={`ss-ib ${open ? 'bg-sel text-sel-ink' : ''}`}
        aria-label={t('theme.settings')}
        title={t('theme.settings')}
        aria-expanded={open}
      >
        <SunMoon size={16} />
      </button>

      {menu.shown && (
        <div
          ref={panelRef}
          role="dialog"
          aria-label={t('theme.settings')}
          {...menu.exitProps}
          className={`ss-menu absolute left-0 bottom-full mb-3 z-50 origin-bottom-left !w-[312px] !p-3 gap-2 ${menu.motion}`}
        >
          <span className="text-xs font-semibold text-ink-3">{t('theme.style')}</span>
          <div role="radiogroup" aria-label={t('theme.style')} className="flex gap-1.5">
            {styles.map((s) => {
              const on = style === s.value;
              return (
                <button
                  key={s.value}
                  type="button"
                  role="radio"
                  aria-checked={on}
                  tabIndex={on ? 0 : -1}
                  onClick={() => setStyle(s.value)}
                  onKeyDown={(e) => handleKeyDown(e, 'style')}
                  className={`flex-1 flex flex-col gap-1.5 p-1.5 rounded-[10px] border cursor-pointer focus-visible:outline-2 focus-visible:outline-link ${on ? 'border-line-2 bg-sel' : 'border-transparent hover:bg-sunken'}`}
                >
                  <StyleThumb value={s.value} custom={thumb} />
                  <span
                    className={`flex items-center justify-between px-0.5 text-[13px] ${on ? 'font-semibold text-ink' : 'text-ink-2'}`}
                  >
                    {t(s.label)}
                    {on && <Check size={14} />}
                  </span>
                </button>
              );
            })}
          </div>
          <div className="flex items-center justify-between gap-2 rounded-[var(--r-ctl)] bg-sunken px-2.5 py-2 text-[13px]">
            <span className="flex min-w-0 items-center gap-1.5 text-ink-2">
              {[thumb.accent, thumb.bg, thumb.ink].map((c, i) => (
                <span
                  key={i}
                  className="size-3 shrink-0 rounded-full shadow-[0_0_0_1px_var(--line-2)]"
                  style={{ background: c }}
                />
              ))}
              <span className="ml-0.5 truncate">{label(customTheme)}</span>
            </span>
            <button
              type="button"
              className="ss-more shrink-0"
              onClick={() => {
                setStyle('custom');
                setOpen(false);
                void navigate('/appearance#custom-theme');
              }}
            >
              {t('theme.editColors')}
            </button>
          </div>
          <span className="text-xs font-semibold text-ink-3 mt-1">{t('theme.mode')}</span>
          <div
            ref={segSlide}
            role="radiogroup"
            aria-label={t('theme.mode')}
            className="ss-seg !flex"
          >
            {modes.map((m) => {
              const on = modePreference === m.value;
              const Icon = m.icon;
              return (
                <button
                  key={m.value}
                  type="button"
                  role="radio"
                  aria-checked={on}
                  tabIndex={on ? 0 : -1}
                  onClick={() => setModePreference(m.value)}
                  onKeyDown={(e) => handleKeyDown(e, 'mode')}
                  className={`flex-1 justify-center !px-0 ${on ? 'on' : ''}`}
                >
                  <Icon size={14} />
                  {t(m.label)}
                </button>
              );
            })}
          </div>
          <span className="text-xs text-ink-3">{t('theme.hint')}</span>
        </div>
      )}
    </div>
  );
}
