import { useEffect, useLayoutEffect, useRef } from 'react';
import type { RefObject } from 'react';
import { claimSaveShortcut } from '../../hooks/useGlobalShortcuts';

/** The dialog in front: dialogs open in portals at the end of the body, so the last is on top. */
const topDialog = () => {
  const open = document.querySelectorAll('[role="dialog"][aria-modal="true"]');
  return open.length ? open[open.length - 1] : null;
};

/**
 * ⌘S / Ctrl+S calls save instead of opening the browser's save-page dialog,
 * and instead of the dashboard's go-to-Sync shortcut, while enabled. save
 * decides whether there is anything to save.
 *
 * Only the editor in front saves: a page editor while no dialog is open, an
 * editor in a dialog (scope: an element inside it) while its dialog is on top.
 */
export function useSaveShortcut(
  save: () => void,
  enabled = true,
  scope?: RefObject<HTMLElement | null>,
) {
  const saveRef = useRef(save);
  useLayoutEffect(() => {
    saveRef.current = save;
  });
  useEffect(() => {
    if (!enabled) return;
    const release = claimSaveShortcut();
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
        e.preventDefault();
        const top = topDialog();
        const inFront = top ? Boolean(scope?.current && top.contains(scope.current)) : !scope;
        if (inFront) saveRef.current();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
      release();
    };
  }, [enabled, scope]);
}
