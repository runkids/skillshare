import { useEffect, useEffectEvent } from 'react';
import type { RefObject } from 'react';

/**
 * While `open`, calls `close` on a mousedown outside `ref` or on Escape. `ref` has to contain the
 * trigger too, or a click on it would close and reopen. `close` gets the event, so a caller can
 * treat Escape differently.
 */
export function useDismiss(
  ref: RefObject<HTMLElement | null>,
  open: boolean,
  close: (e: MouseEvent | KeyboardEvent) => void,
) {
  const dismiss = useEffectEvent(close);
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) dismiss(e);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') dismiss(e);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [ref, open]);
}
