import { useState, useRef, useCallback, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

interface TooltipProps {
  children: ReactNode;
  content: string;
  side?: 'top' | 'bottom';
}

export default function Tooltip({ children, content, side = 'bottom' }: TooltipProps) {
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const show = useCallback((e: React.MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    timerRef.current = setTimeout(() => {
      setPos({
        x: rect.left,
        y: side === 'top' ? rect.top - 4 : rect.bottom + 4,
      });
    }, 200);
  }, [side]);

  const hide = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    setPos(null);
  }, []);

  return (
    <>
      <span onMouseEnter={show} onMouseLeave={hide}>
        {children}
      </span>
      {pos && createPortal(
        <div
          className="ss-tooltip fixed z-[9999] max-w-sm break-all whitespace-pre-line bg-pencil text-paper text-xs px-2.5 py-1.5 shadow-lg pointer-events-none animate-fade-in rounded-[var(--radius-sm)]"
          style={{
            left: pos.x,
            top: pos.y,
            ...(side === 'top' ? { transform: 'translateY(-100%)' } : {}),
          }}
        >
          {content}
        </div>,
        document.body,
      )}
    </>
  );
}
