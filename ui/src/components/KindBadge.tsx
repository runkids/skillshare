interface KindBadgeProps {
  kind: 'skill' | 'agent';
}

export default function KindBadge({ kind }: KindBadgeProps) {
  return <span className={`ss-tag shrink-0 ${kind === 'agent' ? 'inf' : ''}`}>{kind}</span>;
}
