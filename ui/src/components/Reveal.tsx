import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';

/**
 * Grows a block in when it opens and keeps it, inert, while it shrinks away after it closes.
 * The caller renders the children only while open; during the shrink the last ones stay on screen.
 */
export default function Reveal({ open, className = '', children }: { open: boolean; className?: string; children: ReactNode }) {
  const [held, setHeld] = useState<ReactNode>(open ? children : null);
  if (open && children !== held) setHeld(children);
  useEffect(() => {
    if (open) return;
    // Outlasts the .ss-collapse.grow.shut transition in components.css (100ms delay + 250ms).
    const timer = setTimeout(() => setHeld(null), 370);
    return () => clearTimeout(timer);
  }, [open]);
  if (!open && held == null) return null;
  return (
    <div className={`ss-collapse grow ${open ? '' : 'shut'} ${className}`} inert={!open} aria-hidden={!open || undefined}>
      <div>{open ? children : held}</div>
    </div>
  );
}
