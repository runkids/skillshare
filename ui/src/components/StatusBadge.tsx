import Badge from './Badge';

const statusVariant: Record<string, 'success' | 'warning' | 'danger' | 'info' | 'default'> = {
  merged: 'default',
  copied: 'default',
  linked: 'default',
  'not exist': 'warning',
  'has files': 'info',
  conflict: 'danger',
  broken: 'danger',
  unknown: 'default',
};

export default function StatusBadge({ status }: { status: string }) {
  return <Badge variant={statusVariant[status] ?? 'default'}>{status}</Badge>;
}
