import { X, Keyboard } from 'lucide-react';
import { useFocusTrap } from '../hooks/useFocusTrap';
import { SHORTCUT_ENTRIES, isMacOS } from '../hooks/useGlobalShortcuts';
import { radius } from '../design';

interface KeyboardShortcutsModalProps {
  open: boolean;
  onClose: () => void;
}

export default function KeyboardShortcutsModal({ open, onClose }: KeyboardShortcutsModalProps) {
  const trapRef = useFocusTrap(open);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-pencil/30 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* Dialog */}
      <div
        ref={trapRef}
        role="dialog"
        aria-modal="true"
        aria-label="Keyboard shortcuts"
        className="relative w-full max-w-md bg-surface border-2 border-pencil p-6 animate-fade-in"
        style={{ borderRadius: radius.md, boxShadow: '4px 4px 0 rgba(0,0,0,0.15)' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between mb-5">
          <div className="flex items-center gap-2">
            <Keyboard size={20} strokeWidth={2.5} className="text-pencil" />
            <h2 className="text-lg font-bold text-pencil">
              Keyboard Shortcuts
            </h2>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 flex items-center justify-center text-pencil-light hover:text-pencil transition-colors cursor-pointer"
            aria-label="Close"
          >
            <X size={18} strokeWidth={2.5} />
          </button>
        </div>

        {/* Shortcut list */}
        <div className="space-y-1">
          {SHORTCUT_ENTRIES.map((entry) => (
            <div
              key={entry.keys}
              className="flex items-center justify-between py-2 px-2 hover:bg-paper-warm/60 transition-colors"
              style={{ borderRadius: radius.sm }}
            >
              <span className="text-sm text-pencil-light">
                {entry.label}
              </span>
              <ShortcutKeys keys={entry.keys} />
            </div>
          ))}
        </div>

        {/* Footer hint */}
        <p className="mt-4 pt-3 border-t border-dashed border-pencil-light/30 text-xs text-pencil-light">
          Shortcuts are disabled when typing in inputs.
        </p>
      </div>
    </div>
  );
}

/** Renders key combo like "g d" or "Mod+S" as individual key badges */
function ShortcutKeys({ keys }: { keys: string }) {
  const mac = isMacOS();

  // Handle modifier shortcuts like "Mod+S"
  if (keys.startsWith('Mod+')) {
    const keyPart = keys.replace('Mod+', '');
    const modSymbol = mac ? '⌘' : 'Ctrl';
    return (
      <span className="flex items-center gap-0.5">
        <KeyBadge label={modSymbol} />
        <KeyBadge label={keyPart} />
      </span>
    );
  }

  // Regular keys: "g d" → chord, "?" → single
  const parts = keys.split(' ');
  return (
    <span className="flex items-center gap-1">
      {parts.map((part, i) => (
        <span key={i}>
          <KeyBadge label={part} />
          {i < parts.length - 1 && (
            <span className="text-pencil-light/50 text-xs mx-0.5">then</span>
          )}
        </span>
      ))}
    </span>
  );
}

function KeyBadge({ label }: { label: string }) {
  return (
    <kbd
      className="inline-flex items-center justify-center min-w-[24px] h-6 px-1.5 text-xs font-mono font-medium text-pencil bg-paper-warm border border-pencil-light/40"
      style={{ borderRadius: radius.sm, boxShadow: '0 1px 0 rgba(0,0,0,0.1)' }}
    >
      {label}
    </kbd>
  );
}
