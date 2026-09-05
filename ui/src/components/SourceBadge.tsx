import Badge from './Badge';

type SourceType = 'tracked' | 'github' | 'remote' | 'local';

interface SourceBadgeProps {
  type?: string;
  isInRepo?: boolean;
  size?: 'sm' | 'md';
}

// Metadata `type` values: github, github-subdir, git-https, git-ssh
// (optionally -subdir), local, or empty for skills with no install source.
function resolveSource(type?: string, isInRepo?: boolean): SourceType {
  if (isInRepo) return 'tracked';
  if (type?.startsWith('github')) return 'github';
  if (type && !type.startsWith('local')) return 'remote';
  return 'local';
}

const config: Record<SourceType, { label: string; variant: 'default' | 'info' }> = {
  tracked: { label: 'Tracked', variant: 'default' },
  github: { label: 'GitHub', variant: 'info' },
  remote: { label: 'Remote', variant: 'info' },
  local: { label: 'Local', variant: 'default' },
};

export default function SourceBadge({ type, isInRepo, size = 'sm' }: SourceBadgeProps) {
  const source = resolveSource(type, isInRepo);
  const { label, variant } = config[source];
  return <Badge variant={variant} size={size}>{label}</Badge>;
}
