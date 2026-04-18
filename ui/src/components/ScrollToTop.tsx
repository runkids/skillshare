import { useState, useEffect } from 'react';
import { ArrowUp } from 'lucide-react';
import { radius } from '../design';

interface ScrollToTopProps {
  /** Scroll threshold in pixels before the button appears (default: 400) */
  threshold?: number;
  /** 'floating' pins the button to bottom-right (legacy);
   *  'inline' renders a compact button that fits inside a sticky header/toolbar. */
  variant?: 'floating' | 'inline';
}

export default function ScrollToTop({ threshold = 400, variant = 'floating' }: ScrollToTopProps) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const handler = () => setVisible(window.scrollY > threshold);
    window.addEventListener('scroll', handler, { passive: true });
    handler();
    return () => window.removeEventListener('scroll', handler);
  }, [threshold]);

  if (!visible) return null;

  if (variant === 'inline') {
    return (
      <button
        onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
        className="inline-flex items-center justify-center w-8 h-8 bg-surface border border-muted-dark text-pencil-light hover:text-accent hover:border-accent hover:bg-accent-light transition-all duration-150 cursor-pointer animate-fade-in"
        style={{ borderRadius: radius.sm }}
        aria-label="Scroll to top"
        title="Scroll to top"
      >
        <ArrowUp size={14} strokeWidth={2.5} />
      </button>
    );
  }

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
