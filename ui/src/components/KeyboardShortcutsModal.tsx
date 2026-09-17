import { X } from 'lucide-react';
import { SHORTCUT_ENTRIES, isMacOS } from '../hooks/useGlobalShortcuts';
import DialogShell from './DialogShell';
import Button from './Button';
import { useT } from '../i18n';

interface KeyboardShortcutsModalProps {
  open: boolean;
  onClose: () => void;
}

export default function KeyboardShortcutsModal({ open, onClose }: KeyboardShortcutsModalProps) {
  const t = useT();
  const groups = [
    { labelKey: 'shortcuts.general', entries: SHORTCUT_ENTRIES.filter((e) => !e.keys.startsWith('g ')) },
    { labelKey: 'shortcuts.goTo', entries: SHORTCUT_ENTRIES.filter((e) => e.keys.startsWith('g ')) },
  ];
  return (
    <DialogShell open={open} onClose={onClose} maxWidth="3xl" padding="none" ariaLabel={t('shortcuts.title')}>
      <div className="dh">
        <h2 className="ss-h2">{t('shortcuts.title')}</h2>
        <button type="button" className="ss-ib" onClick={onClose} aria-label={t('common.close')}>
          <X size={16} />
        </button>
      </div>
      <div className="db">
        <div className="grid grid-cols-2 gap-4 items-start">
          {groups.map((g) => (
            <div key={g.labelKey} className="ss-list !shadow-none">
              <div className="ss-lh">{t(g.labelKey)}</div>
              {g.entries.map((entry) => (
                <div key={entry.keys} className="ss-r !min-h-10">
                  <span className="flex-1 text-[13px]">{t(entry.labelKey)}</span>
                  <ShortcutKeys keys={entry.keys} />
                </div>
              ))}
            </div>
          ))}
        </div>
        <p className="text-[13px] text-ink-2">{t('shortcuts.disabledInInputs')}</p>
      </div>
      <div className="df">
        <Button onClick={onClose}>{t('common.close')}</Button>
      </div>
    </DialogShell>
  );
}

/** Renders a key combo like "g d" or "Mod+S" as key tags */
function ShortcutKeys({ keys }: { keys: string }) {
  const parts = keys.startsWith('Mod+') ? [isMacOS() ? '⌘' : 'Ctrl', keys.slice(4)] : keys.split(' ');
  return (
    <span className="flex items-center gap-1">
      {parts.map((part, i) => (
        <kbd key={i} className="ss-tag min-w-6 justify-center">{part}</kbd>
      ))}
    </span>
  );
}
