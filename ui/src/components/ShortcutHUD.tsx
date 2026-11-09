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
    <div className="ss-hud fixed bottom-6 left-1/2 -translate-x-1/2 z-50 animate-fade-in">
      {modifierShortcuts.map((entry) => {
        // "Mod+S" → extract the key part after "Mod+"
        const keyPart = entry.keys.replace('Mod+', '');
        return (
          <div key={entry.keys} className="flex items-center gap-2">
            <span className="flex items-center gap-0.5">
              <kbd className="ss-tag min-w-6 justify-center">{modifierSymbol}</kbd>
              <kbd className="ss-tag min-w-6 justify-center">{keyPart}</kbd>
            </span>
            <span>{t(entry.labelKey)}</span>
          </div>
        );
      })}
    </div>
  );
}
