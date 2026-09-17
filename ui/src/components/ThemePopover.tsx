import { useState, useRef, useEffect, useCallback } from 'react';
import { SunMoon, Sun, Moon, Monitor, Check } from 'lucide-react';
import { useTheme, type Style, type ModePreference } from '../context/ThemeContext';
import { useT } from '../i18n';

const styles: { value: Style; label: string }[] = [
  { value: 'clean', label: 'theme.clean' },
  { value: 'playful', label: 'theme.playful' },
];

const modes: { value: ModePreference; label: string; icon: typeof Sun }[] = [
  { value: 'light', label: 'theme.light', icon: Sun },
  { value: 'dark', label: 'theme.dark', icon: Moon },
  { value: 'system', label: 'theme.system', icon: Monitor },
];

/** Miniature of each style so the choice is visible before picking it. Fixed colours on purpose. */
function StyleThumb({ value }: { value: Style }) {
  const playful = value === 'playful';
  const card = playful
    ? { borderRadius: '3px 9px 4px 8px', border: '1.5px solid #2d2a26', background: '#fff6b8' }
    : { borderRadius: 5, border: '1px solid #d9d6cf', background: '#fff' };
  return (
    <span
      className="flex flex-col gap-1 p-[7px] h-[54px] rounded-[7px] border border-line-soft"
      style={{ background: playful ? '#FFF8E8' : '#f7f6f3' }}
      aria-hidden="true"
    >
      <span className="block w-[34px] h-[5px] rounded-[3px]" style={{ background: '#2d2a26' }} />
      <span className="block h-[9px]" style={card} />
      <span className="block h-[9px]" style={card} />
    </span>
  );
}

export default function ThemePopover() {
  const t = useT();
  const { style, setStyle, modePreference, setModePreference } = useTheme();
  const [open, setOpen] = useState(false);
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
    (panelRef.current.querySelector('[role="radio"][aria-checked="true"]') as HTMLElement | null)?.focus();
  }, [open]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent, group: 'style' | 'mode') => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();
    const items = group === 'style' ? styles : modes;
    const current = group === 'style' ? style : modePreference;
    const idx = items.findIndex((i) => i.value === current);
    const next = e.key === 'ArrowRight' ? items[(idx + 1) % items.length] : items[(idx - 1 + items.length) % items.length];
    if (group === 'style') setStyle(next.value as Style);
    else setModePreference(next.value as ModePreference);
  }, [style, modePreference, setStyle, setModePreference]);

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

      {open && (
        <div
          ref={panelRef}
          role="dialog"
          aria-label={t('theme.settings')}
          className="ss-menu absolute left-0 bottom-full mb-3 z-50 !w-[264px] !p-3 gap-2 animate-dropdown-in"
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
                  className={`flex-1 flex flex-col gap-1.5 p-1.5 rounded-[10px] border-2 cursor-pointer focus-visible:outline-2 focus-visible:outline-link ${on ? 'border-link' : 'border-transparent hover:bg-sunken'}`}
                >
                  <StyleThumb value={s.value} />
                  <span className={`flex items-center justify-between px-0.5 text-[13px] ${on ? 'font-semibold text-ink' : 'text-ink-2'}`}>
                    {t(s.label)}
                    {on && <Check size={14} />}
                  </span>
                </button>
              );
            })}
          </div>
          <span className="text-xs font-semibold text-ink-3 mt-1">{t('theme.mode')}</span>
          <div role="radiogroup" aria-label={t('theme.mode')} className="ss-seg !flex">
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
