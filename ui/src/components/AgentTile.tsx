import type { ComponentPropsWithoutRef, ReactNode } from 'react';
import AgentIcon from './AgentIcon';

/** Frame and logo sizes that go together. `lg` is the plain `.ss-at`: 30px, or 24px in a stack. */
const SIZES = {
  sm: { frame: '!h-5 !w-5', icon: 11 },
  md: { frame: '!h-6 !w-6', icon: 14 },
  lg: { frame: '', icon: 17 },
};

export type AgentTileSize = keyof typeof SIZES;

interface Props extends ComponentPropsWithoutRef<'span'> {
  target?: string;
  size?: AgentTileSize;
  /** For a spot whose logo differs from the size's pair. */
  iconSize?: number;
  /** Drawn in place of the logo when there is no `target`. */
  fallback?: ReactNode;
}

/** An Agent's logo in its `ss-at` frame. */
export default function AgentTile({
  target,
  size = 'lg',
  iconSize = SIZES[size].icon,
  fallback,
  className,
  children,
  ...props
}: Props) {
  return (
    <span {...props} className={['ss-at', SIZES[size].frame, className].filter(Boolean).join(' ')}>
      {target ? <AgentIcon target={target} size={iconSize} /> : fallback}
      {children}
    </span>
  );
}
