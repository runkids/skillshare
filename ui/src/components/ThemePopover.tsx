import { useState, useRef, useEffect, useLayoutEffect, useCallback } from 'react';
import { Palette } from 'lucide-react';
import { useTheme, type Style, type ModePreference } from '../context/ThemeContext';
import { shadows } from '../design';
import { useT } from '../i18n';

const styles: { value: Style; label: string }[] = [
  { value: 'clean', label: 'theme.clean' },
  { value: 'playful', label: 'theme.playful' },
];

const modes: { value: ModePreference; label: string }[] = [
  { value: 'light', label: 'theme.light' },
  { value: 'dark', label: 'theme.dark' },
  { value: 'system', label: 'theme.system' },
];

export default function ThemePopover() {
  const t = useT();
  const { style, setStyle, modePreference, setModePreference } = useTheme();
  const [open, setOpen] = useState(false);
  const [dropUp, setDropUp] = useState(true);
  const containerRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  // Return focus to trigger on close
  const prevOpen = useRef(open);
  useEffect(() => {
    if (prevOpen.current && !open) {
      triggerRef.current?.focus();
    }
    prevOpen.current = open;
  }, [open]);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [open]);

  // Collision detection
  useLayoutEffect(() => {
    if (!open || !containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    const panelHeight = 180;
    setDropUp(rect.top > panelHeight);
  }, [open]);

  // Focus first radio on open
  useEffect(() => {
    if (!open || !panelRef.current) return;
    const firstRadio = panelRef.current.querySelector('[role="radio"]') as HTMLElement;
    firstRadio?.focus();
  }, [open]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent, group: 'style' | 'mode') => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();

    const items = group === 'style' ? styles : modes;
    const current = group === 'style' ? style : modePreference;
    const idx = items.findIndex((i) => i.value === current);
    const next = e.key === 'ArrowRight'
      ? items[(idx + 1) % items.length]
      : items[(idx - 1 + items.length) % items.length];
    if (group === 'style') {
      setStyle(next.value as Style);
    } else {
      setModePreference(next.value as ModePreference);
    }
  }, [style, modePreference, setStyle, setModePreference]);

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={triggerRef}
        onClick={() => setOpen(!open)}
        className="flex items-center gap-3 px-3 py-1.5 text-sm text-pencil-light hover:text-pencil hover:bg-muted/20 transition-colors cursor-pointer w-full"
        aria-label={t('theme.settings')}
        aria-expanded={open}
      >
        <Palette size={16} strokeWidth={2.5} />
        {t('theme.settings')}
      </button>

      {open && (
        <div
          ref={panelRef}
          role="dialog"
          aria-label={t('theme.settings')}
          className={`
            absolute left-0 z-50 w-56 bg-surface border border-muted p-3 rounded-[var(--radius-md)] animate-dropdown-in
            ${dropUp ? 'bottom-full mb-2' : 'top-full mt-2'}
          `}
          style={{ boxShadow: shadows.lg }}
        >
          {/* Style group */}
          <div role="radiogroup" aria-label={t('theme.style')} className="mb-3">
            <div className="text-xs font-medium text-muted-dark uppercase tracking-wider mb-2">{t('theme.style')}</div>
            <div className="flex gap-2">
              {styles.map((s) => (
                <button
                  key={s.value}
                  role="radio"
                  aria-checked={style === s.value}
                  onClick={() => setStyle(s.value)}
                  onKeyDown={(e) => handleKeyDown(e, 'style')}
                  className={`
                    flex-1 px-3 py-1.5 text-sm rounded-lg transition-colors cursor-pointer
                    focus-visible:ring-2 focus-visible:ring-pencil/20 focus-visible:outline-none
                    ${style === s.value
                      ? 'bg-pencil text-paper font-medium'
                      : 'bg-muted/30 text-pencil-light hover:bg-muted/50'}
                  `}
                  tabIndex={style === s.value ? 0 : -1}
                >
                  {t(s.label)}
                </button>
              ))}
            </div>
          </div>

          {/* Mode group */}
          <div role="radiogroup" aria-label={t('theme.mode')}>
            <div className="text-xs font-medium text-muted-dark uppercase tracking-wider mb-2">{t('theme.mode')}</div>
            <div className="flex gap-1.5">
              {modes.map((m) => (
                <button
                  key={m.value}
                  role="radio"
                  aria-checked={modePreference === m.value}
                  onClick={() => setModePreference(m.value)}
                  onKeyDown={(e) => handleKeyDown(e, 'mode')}
                  className={`
                    flex-1 px-2 py-1.5 text-xs rounded-lg transition-colors cursor-pointer
                    focus-visible:ring-2 focus-visible:ring-pencil/20 focus-visible:outline-none
                    ${modePreference === m.value
                      ? 'bg-pencil text-paper font-medium'
                      : 'bg-muted/30 text-pencil-light hover:bg-muted/50'}
                  `}
                  tabIndex={modePreference === m.value ? 0 : -1}
                >
                  {t(m.label)}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
