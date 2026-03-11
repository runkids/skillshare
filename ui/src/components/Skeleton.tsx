import { radius } from '../design';

interface SkeletonProps {
  className?: string;
  variant?: 'text' | 'card' | 'circle';
  style?: React.CSSProperties;
}

export default function Skeleton({ className = '', variant = 'text', style }: SkeletonProps) {
  const base = 'animate-skeleton';

  if (variant === 'circle') {
    return (
      <div
        className={`${base} w-12 h-12 ${className}`}
        style={{ borderRadius: '50%', ...style }}
      />
    );
  }

  if (variant === 'card') {
    return (
      <div
        className={`${base} border border-muted p-4 h-32 ${className}`}
        style={{ borderRadius: radius.md, ...style }}
      />
    );
  }

  return (
    <div
      className={`${base} h-4 ${className}`}
      style={{ borderRadius: radius.sm, ...style }}
    />
  );
}

/** A full loading skeleton for a page */
export function PageSkeleton() {
  return (
    <div className="space-y-6 animate-fade-in">
      <Skeleton className="w-48 h-8" />
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {[0, 1, 2].map((i) => (
          <Skeleton
            key={i}
            variant="card"
            className="animate-skeleton"
            style={{ animationDelay: `${i * 50}ms` } as React.CSSProperties}
          />
        ))}
      </div>
      {[0, 1, 2].map((i) => (
        <Skeleton
          key={i}
          className={i === 0 ? 'w-full h-4' : i === 1 ? 'w-3/4 h-4' : 'w-1/2 h-4'}
          style={{ animationDelay: `${(i + 3) * 50}ms` } as React.CSSProperties}
        />
      ))}
    </div>
  );
}
