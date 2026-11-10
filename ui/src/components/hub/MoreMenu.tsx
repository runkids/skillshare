import { useEffect, useRef, useState } from 'react';
import { MoreHorizontal } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useMenuPresence } from '../../hooks/usePresence';

interface Item {
  label: string;
  icon: LucideIcon;
  onClick: () => void;
  danger?: boolean;
}

/** The rarely used actions of a hub, behind one icon button. */
export default function MoreMenu({ label, items }: { label: string; items: Item[] }) {
  const [open, setOpen] = useState(false);
  const menu = useMenuPresence(open);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative shrink-0">
      <button
        type="button"
        className="ss-btn !w-[34px] !px-0"
        aria-label={label}
        title={label}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <MoreHorizontal size={16} />
      </button>
      {menu.shown && (
        <div role="menu" aria-label={label} {...menu.exitProps} className={`ss-menu absolute right-0 top-full z-50 mt-1 !w-44 origin-top-right ${menu.motion}`}>
          {items.map(({ label: text, icon: Icon, onClick, danger }) => (
            <button key={text} type="button" role="menuitem" className={danger ? 'dng' : ''} onClick={() => { setOpen(false); onClick(); }}>
              <Icon size={15} />
              {text}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
