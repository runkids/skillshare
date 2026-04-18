import { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronDown, List } from 'lucide-react';
import { createPortal } from 'react-dom';
import { useT } from '../../i18n';

export interface HeadingItem {
  level: number;
  text: string;
  slug: string;
  line: number;
}

export function parseOutline(markdown: string): HeadingItem[] {
  const out: HeadingItem[] = [];
  let inCode = false;
  const lines = markdown.split('\n');
  lines.forEach((line, idx) => {
    if (/^\s*```/.test(line)) {
      inCode = !inCode;
      return;
    }
    if (inCode) return;
    const m = line.match(/^(#{1,6})\s+(.+?)\s*$/);
    if (!m) return;
    const level = m[1].length;
    const text = m[2].replace(/`([^`]+)`/g, '$1').replace(/\*+/g, '');
    const slug = text
      .toLowerCase()
      .replace(/[^\w\s-]/g, '')
      .replace(/\s+/g, '-');
    out.push({ level, text, slug, line: idx });
  });
  return out;
}

interface OutlineProps {
  markdown: string;
  activeSlug?: string | null;
  onJump: (heading: HeadingItem) => void;
  /** Visual variant. `inline` renders as a trigger + popover (default),
   *  `floating` keeps the legacy fixed-position FAB for read-mode overlay. */
  variant?: 'inline' | 'floating';
}

export default function Outline({ markdown, activeSlug, onJump, variant = 'inline' }: OutlineProps) {
  const t = useT();
  const items = useMemo(() => parseOutline(markdown), [markdown]);
  const [open, setOpen] = useState(false);
  const [stickyOpen, setStickyOpen] = useState(false);
  const [inlineVisible, setInlineVisible] = useState(true);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const stickyRef = useRef<HTMLDivElement | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    if (!open && !stickyOpen) return;
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node;
      const inRoot = rootRef.current?.contains(target);
      const inSticky = stickyRef.current?.contains(target);
      if (!inRoot && !inSticky) {
        setOpen(false);
        setStickyOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false);
        setStickyOpen(false);
      }
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open, stickyOpen]);

  // Observe the inline trigger: when it leaves the viewport we reveal a
  // sticky floating version so the user can keep navigating while reading.
  useEffect(() => {
    if (variant !== 'inline') return;
    const el = triggerRef.current;
    if (!el || typeof IntersectionObserver === 'undefined') return;
    const io = new IntersectionObserver(
      ([entry]) => setInlineVisible(entry.isIntersecting),
      { threshold: 0, rootMargin: '-8px 0px 0px 0px' }
    );
    io.observe(el);
    return () => io.disconnect();
  }, [variant, items.length]);

  if (items.length < 2) return null;

  const renderItems = (onSelect: () => void) => (
    <ul className="outline-list">
      {items.map((h, i) => (
        <li key={i}>
          <button
            type="button"
            className={`outline-item level-${h.level}${activeSlug === h.slug ? ' active' : ''}`}
            onClick={() => {
              onJump(h);
              onSelect();
            }}
            title={h.text}
            role="menuitem"
          >
            {h.text}
          </button>
        </li>
      ))}
    </ul>
  );

  const trigger = (
    <button
      ref={triggerRef}
      type="button"
      className="ss-skill-editor-outline-trigger"
      onClick={() => setOpen((v) => !v)}
      aria-expanded={open}
      aria-haspopup="menu"
      title={open ? t('skillEditor.outline.close') : t('skillEditor.outline.show')}
    >
      <List size={14} strokeWidth={2.2} />
      <span>{t('skillEditor.outline.label')}</span>
      <span className="count">{items.length}</span>
      <ChevronDown size={12} strokeWidth={2.4} className={open ? 'rot' : ''} />
    </button>
  );

  const panel = open && (
    <div
      className={`ss-skill-editor-outline-popover variant-${variant}`}
      role="menu"
      aria-label="Document outline"
    >
      {renderItems(() => setOpen(false))}
    </div>
  );

  // Sticky floating version: only for inline variant, only when inline
  // trigger is off-screen. Rendered via portal so transforms on ancestors
  // (e.g. the hand-drawn card rotation) don't displace it.
  const showSticky = variant === 'inline' && !inlineVisible;
  const stickyNode = showSticky
    ? createPortal(
        <div
          ref={stickyRef}
          className="ss-skill-editor-outline-root variant-sticky"
          aria-label="Document outline (sticky)"
        >
          <button
            type="button"
            className="ss-skill-editor-outline-trigger sticky"
            onClick={() => setStickyOpen((v) => !v)}
            aria-expanded={stickyOpen}
            aria-haspopup="menu"
            title={stickyOpen ? 'Close outline' : 'Jump to section'}
          >
            <List size={16} strokeWidth={2.4} />
            <span className="count">{items.length}</span>
          </button>
          {stickyOpen && (
            <div
              className="ss-skill-editor-outline-popover variant-sticky"
              role="menu"
              aria-label="Document outline"
            >
              {renderItems(() => setStickyOpen(false))}
            </div>
          )}
        </div>,
        document.body
      )
    : null;

  return (
    <>
      <div ref={rootRef} className={`ss-skill-editor-outline-root variant-${variant}`}>
        {trigger}
        {panel}
      </div>
      {stickyNode}
    </>
  );
}
