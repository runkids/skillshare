import { useEffect, useState } from 'react';
import { ArrowUp } from 'lucide-react';
import { useT } from '../i18n';

/** Floating button for long pages; appears once the page is scrolled past `threshold` pixels. */
export default function ScrollToTop({ threshold = 400 }: { threshold?: number }) {
  const t = useT();
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const handler = () => setVisible(window.scrollY > threshold);
    window.addEventListener('scroll', handler, { passive: true });
    handler();
    return () => window.removeEventListener('scroll', handler);
  }, [threshold]);

  if (!visible) return null;
  const label = t('common.scrollToTop');
  return (
    <button type="button" className="ss-top animate-fade-in" aria-label={label} title={label} onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}>
      <ArrowUp size={18} />
    </button>
  );
}
