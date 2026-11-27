import type { ComponentPropsWithoutRef } from 'react';
import { splitStack } from '../lib/splitStack';
import AgentTile, { type AgentTileSize } from './AgentTile';

interface Props extends Omit<ComponentPropsWithoutRef<'span'>, 'children'> {
  targets: readonly string[];
  /** Logos past this many are left out. */
  max?: number;
  /** Ends the stack with a `+N` for the logos left out. */
  showRest?: boolean;
  size?: AgentTileSize;
  iconSize?: number;
  tileClassName?: string;
}

/** Agents as overlapping logos. */
export default function AgentStack({
  targets,
  max,
  showRest,
  size,
  iconSize = size ? undefined : 13,
  tileClassName,
  className,
  ...props
}: Props) {
  const { shown, rest } = splitStack(targets, max);
  return (
    <span {...props} className={className ? `ss-stack ${className}` : 'ss-stack'}>
      {shown.map((target) => (
        <AgentTile
          key={target}
          target={target}
          size={size}
          iconSize={iconSize}
          className={tileClassName}
        />
      ))}
      {showRest && rest > 0 && (
        <span className="ss-at text-[11px] font-semibold text-ink-2">+{rest}</span>
      )}
    </span>
  );
}
