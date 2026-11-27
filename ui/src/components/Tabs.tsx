import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from 'react';
import { Link } from 'react-router-dom';
import type { LinkProps } from 'react-router-dom';
import { segSlide } from '../lib/segSlide';

interface TabsProps extends HTMLAttributes<HTMLElement> {
  /** `TabLink`s, or `TabButton`s in a row with `role="tablist"`. */
  children: ReactNode;
  /** Rows that a route change replaces (each page renders its own) share a key, so the underline
   *  slides over from where the old row had it. */
  slideKey?: string;
}

/** A row of tabs whose underline slides between them in the Clean style. A `role="tablist"` row
 *  switches content in place and renders a div; any other row links between pages and renders a nav. */
export default function Tabs({ slideKey, className = '', children, ...props }: TabsProps) {
  const Tag = props.role === 'tablist' ? 'div' : 'nav';
  return (
    <Tag ref={segSlide} data-slide-key={slideKey} className={`ss-tabs ${className}`} {...props}>
      {children}
    </Tag>
  );
}

interface TabProps {
  /** The selected tab. */
  on: boolean;
  /** A count after the label; leave it out to show none. */
  count?: number;
  children: ReactNode;
}

/** A tab that goes to another page or `?tab=`. */
export function TabLink({
  on,
  count,
  children,
  ...props
}: TabProps & Omit<LinkProps, 'className'>) {
  return (
    <Link className={on ? 'on' : ''} aria-current={on ? 'page' : undefined} {...props}>
      {children}
      {count !== undefined && <span className="ss-cnt">{count}</span>}
    </Link>
  );
}

/** A tab that switches content in place, inside a `<Tabs role="tablist">`. */
export function TabButton({
  on,
  count,
  children,
  ...props
}: TabProps & Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'className'>) {
  return (
    <button type="button" role="tab" aria-selected={on} className={on ? 'on' : ''} {...props}>
      {children}
      {count !== undefined && <span className="ss-cnt">{count}</span>}
    </button>
  );
}
