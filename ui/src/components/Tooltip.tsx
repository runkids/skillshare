import { useState, useRef, useCallback, useLayoutEffect, useEffect, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

interface TooltipProps {
  children: ReactNode;
  content: ReactNode;
  side?: 'top' | 'bottom';
  followCursor?: boolean;
  delay?: number;
  /** Display wrapper as block element */
  block?: boolean;
}

const OFFSET = 12;
const MARGIN = 8;

export default function Tooltip({ children, content, side = 'bottom', followCursor, delay = 200, block }: TooltipProps) {
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const visibleRef = useRef(false);
  const tooltipRef = useRef<HTMLDivElement>(null);
  const latestCursor = useRef({ x: 0, y: 0 });
  const rafRef = useRef<number | undefined>(undefined);

  // Render at (0,0) hidden → measure natural size → clamp → show
  useLayoutEffect(() => {
    const el = tooltipRef.current;
    if (!el || !pos) return;

    const w = el.offsetWidth;
    const h = el.offsetHeight;
    const vw = window.innerWidth;
    const vh = window.innerHeight;

    let x = pos.x;
    let y = pos.y;
    if (x + w > vw - MARGIN) x = vw - MARGIN - w;
    if (x < MARGIN) x = MARGIN;
    if (y + h > vh - MARGIN) y = pos.y - h - OFFSET * 2;
    if (y < MARGIN) y = MARGIN;

    el.style.left = `${x}px`;
    el.style.top = `${y}px`;
    el.style.visibility = 'visible';
  }, [pos]);

  const show = useCallback((e: React.MouseEvent) => {
    if (followCursor) {
      latestCursor.current = { x: e.clientX + OFFSET, y: e.clientY + OFFSET };
      timerRef.current = setTimeout(() => {
        visibleRef.current = true;
        setPos({ ...latestCursor.current });
      }, delay);
    } else {
      const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
      timerRef.current = setTimeout(() => {
        setPos({
          x: rect.left,
          y: side === 'top' ? rect.top - 4 : rect.bottom + 4,
        });
      }, delay);
    }
  }, [side, followCursor, delay]);

  const move = useCallback((e: React.MouseEvent) => {
    latestCursor.current = { x: e.clientX + OFFSET, y: e.clientY + OFFSET };
    if (visibleRef.current && rafRef.current === undefined) {
      rafRef.current = requestAnimationFrame(() => {
        rafRef.current = undefined;
        setPos({ ...latestCursor.current });
      });
    }
  }, []);

  const hide = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    if (rafRef.current !== undefined) cancelAnimationFrame(rafRef.current);
    rafRef.current = undefined;
    visibleRef.current = false;
    setPos(null);
  }, []);

  useEffect(() => () => {
    if (timerRef.current) clearTimeout(timerRef.current);
    if (rafRef.current !== undefined) cancelAnimationFrame(rafRef.current);
  }, []);

  return (
    <>
      <span className={block ? 'block' : undefined} onMouseEnter={show} onMouseLeave={hide} onMouseMove={followCursor ? move : undefined}>
        {children}
      </span>
      {pos && createPortal(
        <div
          ref={tooltipRef}
          className="ss-tooltip fixed z-[9999] max-w-sm whitespace-pre-line bg-pencil text-paper text-xs px-2.5 py-1.5 shadow-lg pointer-events-none animate-fade-in rounded-[var(--radius-sm)]"
          style={{ left: 0, top: 0, visibility: 'hidden' }}
        >
          {content}
        </div>,
        document.body,
      )}
    </>
  );
}
