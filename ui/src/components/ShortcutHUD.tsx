import { radius } from '../design';
import { isMacOS, SHORTCUT_ENTRIES } from '../hooks/useGlobalShortcuts';
import { useT } from '../i18n';

interface ShortcutHUDProps {
  visible: boolean;
}

export default function ShortcutHUD({ visible }: ShortcutHUDProps) {
  const t = useT();
  if (!visible) return null;

  const mac = isMacOS();
  const modifierSymbol = mac ? '⌘' : 'Ctrl';
  const modifierShortcuts = SHORTCUT_ENTRIES.filter((e) => e.modifier);

  if (modifierShortcuts.length === 0) return null;

  return (
    <div
      className="ss-shortcut-hud fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex items-center gap-4 px-4 py-2.5 bg-surface/95 backdrop-blur-sm border-2 border-pencil animate-fade-in"
      style={{ borderRadius: radius.md, boxShadow: '0 2px 8px rgba(0,0,0,0.12)' }}
    >
      {modifierShortcuts.map((entry) => {
        // "Mod+S" → extract the key part after "Mod+"
        const keyPart = entry.keys.replace('Mod+', '');
        return (
          <div key={entry.keys} className="flex items-center gap-2">
            <span className="flex items-center gap-0.5">
              <kbd
                className="inline-flex items-center justify-center min-w-[24px] h-6 px-1.5 text-xs font-mono font-medium text-pencil bg-paper-warm border border-pencil-light/40"
                style={{ borderRadius: radius.sm, boxShadow: '0 1px 0 rgba(0,0,0,0.1)' }}
              >
                {modifierSymbol}
              </kbd>
              <kbd
                className="inline-flex items-center justify-center min-w-[24px] h-6 px-1.5 text-xs font-mono font-medium text-pencil bg-paper-warm border border-pencil-light/40"
                style={{ borderRadius: radius.sm, boxShadow: '0 1px 0 rgba(0,0,0,0.1)' }}
              >
                {keyPart}
              </kbd>
            </span>
            <span className="text-sm text-pencil-light">{t(entry.labelKey)}</span>
          </div>
        );
      })}
    </div>
  );
}
