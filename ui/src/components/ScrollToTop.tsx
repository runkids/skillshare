import { useState, useEffect } from 'react';
import { ArrowUp } from 'lucide-react';
import { radius } from '../design';

interface ScrollToTopProps {
  /** Scroll threshold in pixels before the button appears (default: 400) */
  threshold?: number;
}

export default function ScrollToTop({ threshold = 400 }: ScrollToTopProps) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const handler = () => setVisible(window.scrollY > threshold);
    window.addEventListener('scroll', handler, { passive: true });
    handler();
    return () => window.removeEventListener('scroll', handler);
  }, [threshold]);

  if (!visible) return null;

  return (
    <button
      onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
      className="fixed bottom-6 right-6 z-40 w-10 h-10 flex items-center justify-center bg-surface border-2 border-pencil text-pencil hover:bg-paper-warm transition-all duration-150 cursor-pointer animate-fade-in"
      style={{
        borderRadius: radius.sm,
        boxShadow: '3px 3px 0 rgba(0,0,0,0.15)',
      }}
      aria-label="Scroll to top"
    >
      <ArrowUp size={18} strokeWidth={2.5} />
    </button>
  );
}
