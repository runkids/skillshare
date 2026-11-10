import { useEffect, useState } from 'react';

/**
 * Keeps a closing layer mounted for its exit animation of `ms`. While it leaves it is inert and
 * hidden from assistive tech, so it takes no clicks or focus and reads as already closed.
 */
export function usePresence(open: boolean, ms: number) {
  const [shown, setShown] = useState(open);
  if (open && !shown) setShown(true);
  useEffect(() => {
    if (open) return;
    const id = setTimeout(() => setShown(false), ms);
    return () => clearTimeout(id);
  }, [open, ms]);
  const closing = shown && !open;
  return { shown, closing, exitProps: closing ? ({ inert: true, 'aria-hidden': true } as const) : {} };
}

/** A menu: grows in with `.animate-dropdown-in`, fades out over `.animate-dropdown-out`'s 0.12s. */
export function useMenuPresence(open: boolean) {
  const p = usePresence(open, 120);
  return { ...p, motion: p.closing ? 'animate-dropdown-out' : 'animate-dropdown-in' };
}

/** `value` while open; while closing, the value it had when last open (callers often clear it on close). */
export function useHeldWhileClosing<T>(value: T, open: boolean): T {
  const [held, setHeld] = useState(value);
  if (open && value !== held) setHeld(() => value);
  return open ? value : held;
}
