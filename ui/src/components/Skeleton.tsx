import { wobbly } from '../design';

interface SkeletonProps {
  className?: string;
  variant?: 'text' | 'card' | 'circle';
}

export default function Skeleton({ className = '', variant = 'text' }: SkeletonProps) {
  const base = 'animate-pulse-gentle bg-muted';

  if (variant === 'circle') {
    return (
      <div
        className={`${base} w-12 h-12 ${className}`}
        style={{ borderRadius: '50%' }}
      />
    );
  }

  if (variant === 'card') {
    return (
      <div
        className={`${base} border-2 border-muted-dark p-4 h-32 ${className}`}
        style={{ borderRadius: wobbly.md }}
      />
    );
  }

  return (
    <div
      className={`${base} h-4 ${className}`}
      style={{ borderRadius: wobbly.sm }}
    />
  );
}

/** A full loading skeleton for a page */
export function PageSkeleton() {
  return (
    <div className="space-y-6 animate-sketch-in">
      <Skeleton className="w-48 h-8" />
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <Skeleton variant="card" />
        <Skeleton variant="card" />
        <Skeleton variant="card" />
      </div>
      <Skeleton className="w-full h-4" />
      <Skeleton className="w-3/4 h-4" />
      <Skeleton className="w-1/2 h-4" />
    </div>
  );
}
