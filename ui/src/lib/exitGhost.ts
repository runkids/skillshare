type Leave = (ghost: HTMLElement, reduced: boolean) => Animation;

/**
 * Callback ref for a layer its caller removes outright (many dialogs and bars are mounted only while
 * open). When React removes the element, a frozen, inert copy stays in <body> and plays the animation
 * `leave` returns, so the layer exits instead of vanishing. Refs must be stable: create these at
 * module level.
 */
export function exitGhost(leave: Leave) {
  return (node: HTMLElement | null) => {
    if (!node) return;
    return () => {
      // Removed along with what holds it (its page on a route change): nothing to see leave.
      const parent = node.parentNode;
      // Read now: a detached element reports no scroll position.
      const scrolled: [number, number][] = [];
      [node, ...node.querySelectorAll<HTMLElement>('*')].forEach((el, i) => { if (el.scrollTop) scrolled.push([i, el.scrollTop]); });
      queueMicrotask(() => {
        if (node.isConnected) return; // StrictMode re-attaching the ref, not a removal
        if (!parent?.isConnected) return;
        if (typeof node.animate !== 'function') return; // nothing could play the exit (jsdom)
        const ghost = node.cloneNode(true) as HTMLElement;
        ghost.inert = true;
        ghost.setAttribute('aria-hidden', 'true');
        ghost.removeAttribute('aria-modal'); // so code looking for the open dialog skips the copy
        // .ss-ghost freezes the copy's own entrances, which inserting it would replay.
        ghost.classList.add('ss-ghost');
        // Its ids and label targets would otherwise also name the live layer that replaces it.
        for (const el of [ghost, ...ghost.querySelectorAll('[id], [for]')]) {
          el.removeAttribute('id');
          el.removeAttribute('for');
        }
        // First in <body>: a layer that opens as this one leaves (same z-index) paints above the copy.
        document.body.prepend(ghost);
        if (scrolled.length) {
          const copies = [ghost, ...ghost.querySelectorAll<HTMLElement>('*')];
          for (const [i, top] of scrolled) copies[i].scrollTop = top;
        }
        const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
        leave(ghost, reduced).finished.finally(() => ghost.remove());
      });
    };
  };
}

const out = (ms: number) => ({ duration: ms, easing: 'ease-in', fill: 'forwards' }) as const;

/** Dialog scrim: the backdrop lightens while the dialog shrinks back the way it came. */
export const dialogGhost = exitGhost((ghost, reduced) => {
  if (reduced) return ghost.animate([{ opacity: 1 }, { opacity: 0 }], out(150));
  ghost.querySelector('.ss-dlg')?.animate([{ transform: 'none' }, { transform: 'scale(.97) translateY(4px)' }], out(200));
  return ghost.animate([{ opacity: 1 }, { opacity: 0 }], out(200));
});

/** Floating selection bar: sinks back down the way it rose, blurring and shrinking as it goes. */
export const bulkGhost = exitGhost((ghost, reduced) =>
  reduced
    ? ghost.animate([{ opacity: 1 }, { opacity: 0 }], out(150))
    : ghost.animate(
        [
          { opacity: 1, transform: 'translateX(-50%)', filter: 'blur(0)' },
          { opacity: 0, transform: 'translateX(-50%) translateY(100%) scale(.97)', filter: 'blur(6px)' },
        ],
        out(250),
      ),
);
