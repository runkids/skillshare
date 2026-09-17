import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

const DELAY = 300;
const GAP = 6;
const MARGIN = 8;

/**
 * Shows the full text of a cut-off `.truncate` element on hover, app-wide, so call sites don't opt in.
 * Elements with a native `title` are left to it.
 */
export default function TruncateTip() {
  const [tip, setTip] = useState<{ text: string; rect: DOMRect } | null>(null);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let current: Element | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const hide = () => {
      clearTimeout(timer);
      current = null;
      setTip(null);
    };
    const over = (e: MouseEvent) => {
      const el = e.target instanceof Element ? e.target.closest('.truncate') : null;
      if (el === current) return;
      hide();
      if (!el || el.hasAttribute('title') || el.scrollWidth <= el.clientWidth) return;
      current = el;
      timer = setTimeout(() => setTip({ text: el.textContent ?? '', rect: el.getBoundingClientRect() }), DELAY);
    };
    const out = (e: MouseEvent) => {
      if (!e.relatedTarget) hide();
    };
    document.addEventListener('mouseover', over);
    document.addEventListener('mouseout', out);
    document.addEventListener('mousedown', hide);
    window.addEventListener('scroll', hide, true);
    return () => {
      clearTimeout(timer);
      document.removeEventListener('mouseover', over);
      document.removeEventListener('mouseout', out);
      document.removeEventListener('mousedown', hide);
      window.removeEventListener('scroll', hide, true);
    };
  }, []);

  // Render hidden, measure, then place below the text (above when it would leave the viewport).
  useLayoutEffect(() => {
    const node = ref.current;
    if (!node || !tip) return;
    const { rect } = tip;
    const below = rect.bottom + GAP;
    node.style.left = `${Math.max(MARGIN, Math.min(rect.left, window.innerWidth - node.offsetWidth - MARGIN))}px`;
    node.style.top = `${below + node.offsetHeight > window.innerHeight - MARGIN ? rect.top - GAP - node.offsetHeight : below}px`;
    node.style.visibility = 'visible';
  }, [tip]);

  if (!tip) return null;
  return createPortal(
    <div ref={ref} role="tooltip" className="ss-tip pointer-events-none fixed z-[9999] max-w-sm break-words animate-fade-in" style={{ left: 0, top: 0, visibility: 'hidden' }}>
      {tip.text}
    </div>,
    document.body,
  );
}
