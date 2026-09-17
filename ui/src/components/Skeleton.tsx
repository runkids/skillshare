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
        className={`${base} h-32 rounded-[var(--r-box)] ${className}`}
        style={style}
      />
    );
  }

  return (
    <div
      className={`${base} h-4 rounded-[5px] ${className}`}
      style={style}
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

/** Skill detail: crumb, title, tabs and document on the left, metadata and targets on the right */
export function SkillDetailSkeleton() {
  return (
    <div className="animate-fade-in">
      <div className="mb-7 flex flex-col gap-3">
        <Skeleton className="w-28 h-3" />
        <Skeleton className="w-44 h-8" />
        <Skeleton className="w-80 h-4" />
      </div>
      <div className="grid grid-cols-[minmax(0,1fr)_330px] items-start gap-8">
        <div className="flex flex-col gap-[18px]">
          <div className="flex gap-6">
            <Skeleton className="w-20 h-5" />
            <Skeleton className="w-14 h-5" />
            <Skeleton className="w-14 h-5" />
          </div>
          <div className="ss-box space-y-3 !px-[30px] !py-[26px]">
            <Skeleton className="w-1/4 h-5" />
            <Skeleton className="w-full h-4" />
            <Skeleton className="w-5/6 h-4" style={{ animationDelay: '50ms' }} />
            <Skeleton className="w-1/5 h-5" style={{ animationDelay: '100ms' }} />
            <Skeleton className="w-full h-4" style={{ animationDelay: '150ms' }} />
            <Skeleton className="w-2/3 h-4" style={{ animationDelay: '200ms' }} />
          </div>
        </div>
        <div className="flex flex-col gap-7">
          <div className="ss-box space-y-3">
            {Array.from({ length: 7 }, (_, i) => <Skeleton key={i} className="w-full h-4" style={{ animationDelay: `${i * 50}ms` }} />)}
          </div>
          <div className="ss-box space-y-3">
            {Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="w-full h-8" style={{ animationDelay: `${i * 50}ms` }} />)}
          </div>
        </div>
      </div>
    </div>
  );
}
