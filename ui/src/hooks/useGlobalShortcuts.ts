import { useEffect, useRef, useCallback, useState } from 'react';
import { useNavigate } from 'react-router-dom';

export interface ShortcutEntry {
  /** Display label for the shortcuts modal */
  label: string;
  /** Key(s) to display, e.g. "?" or "g d" */
  keys: string;
  /** Whether this is a modifier shortcut (Cmd/Ctrl+key) */
  modifier?: boolean;
}

/** Detect macOS for modifier key display */
export function isMacOS(): boolean {
  // navigator.userAgentData is modern but not universal
  if ('userAgentData' in navigator) {
    return (navigator as { userAgentData?: { platform?: string } }).userAgentData?.platform === 'macOS';
  }
  return /Mac|iPhone|iPad|iPod/.test(navigator.platform ?? '');
}

/** All registered global shortcuts for display in the help modal */
export const SHORTCUT_ENTRIES: ShortcutEntry[] = [
  { keys: '?', label: 'Open keyboard shortcuts' },
  { keys: '/', label: 'Focus search input' },
  { keys: 'r', label: 'Refresh current page' },
  { keys: 'g d', label: 'Go to Dashboard' },
  { keys: 'g s', label: 'Go to Skills' },
  { keys: 'g t', label: 'Go to Targets' },
  { keys: 'g l', label: 'Go to Log' },
  { keys: 'g a', label: 'Go to Audit' },
  { keys: 'g u', label: 'Go to Update' },
  { keys: 'Mod+S', label: 'Go to Sync', modifier: true },
];

const NAV_MAP: Record<string, string> = {
  d: '/',
  s: '/skills',
  t: '/targets',
  l: '/log',
  a: '/audit',
  u: '/update',
};

const CHORD_TIMEOUT = 500;

interface UseGlobalShortcutsOptions {
  onToggleHelp: () => void;
  onRefresh?: () => void;
  onSync?: () => void;
}

export function useGlobalShortcuts({ onToggleHelp, onRefresh, onSync }: UseGlobalShortcutsOptions) {
  const navigate = useNavigate();
  const pendingChordRef = useRef<string | null>(null);
  const chordTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [modifierHeld, setModifierHeld] = useState(false);

  const clearChord = useCallback(() => {
    pendingChordRef.current = null;
    if (chordTimerRef.current) {
      clearTimeout(chordTimerRef.current);
      chordTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      // --- Modifier shortcuts (Cmd/Ctrl+key) — check FIRST ---
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey) {
        const key = e.key.toLowerCase();
        if (key === 's') {
          e.preventDefault();
          if (onSync) {
            onSync();
          } else {
            navigate('/sync');
          }
          return;
        }
        // All other modifier combos: let browser handle natively
        return;
      }

      // Skip when focus is inside an input, textarea, or contenteditable
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if ((e.target as HTMLElement)?.isContentEditable) return;

      // Skip remaining modifier combos (Alt, Shift, etc.)
      if (e.metaKey || e.ctrlKey || e.altKey) return;

      const key = e.key;

      // Handle pending chord (g + second key)
      if (pendingChordRef.current === 'g') {
        clearChord();
        const path = NAV_MAP[key];
        if (path) {
          e.preventDefault();
          navigate(path);
        }
        return;
      }

      switch (key) {
        case '?':
          e.preventDefault();
          onToggleHelp();
          break;
        case '/': {
          // Focus the first visible search input on the page
          const input = document.querySelector<HTMLInputElement>(
            'input[type="text"]:not([hidden])',
          );
          if (input) {
            e.preventDefault();
            input.focus();
          }
          break;
        }
        case 'r':
          if (onRefresh) {
            e.preventDefault();
            onRefresh();
          }
          break;
        case 'g':
          e.preventDefault();
          pendingChordRef.current = 'g';
          chordTimerRef.current = setTimeout(clearChord, CHORD_TIMEOUT);
          break;
      }
    };

    // Modifier held detection (for HUD overlay)
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Meta' || e.key === 'Control') {
        setModifierHeld(true);
      }
    };

    const handleKeyUp = (e: KeyboardEvent) => {
      if (e.key === 'Meta' || e.key === 'Control') {
        setModifierHeld(false);
      }
    };

    const handleBlur = () => {
      setModifierHeld(false);
    };

    window.addEventListener('keydown', handler);
    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('keyup', handleKeyUp);
    window.addEventListener('blur', handleBlur);

    return () => {
      window.removeEventListener('keydown', handler);
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('keyup', handleKeyUp);
      window.removeEventListener('blur', handleBlur);
      clearChord();
    };
  }, [navigate, onToggleHelp, onRefresh, onSync, clearChord]);

  return { modifierHeld };
}
