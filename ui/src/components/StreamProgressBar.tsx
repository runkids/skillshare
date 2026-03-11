import type { LucideIcon } from 'lucide-react';
import Card from './Card';
import { radius, palette } from '../design';

interface StreamProgressBarProps {
  count: number;
  total: number;
  startTime: number;
  icon: LucideIcon;
  iconClassName?: string;
  labelDiscovering: string;
  labelRunning: string;
  units: string;
}

export default function StreamProgressBar({
  count, total, startTime,
  icon: Icon, iconClassName = 'animate-spin',
  labelDiscovering, labelRunning, units,
}: StreamProgressBarProps) {
  const pct = total > 0 ? Math.min((count / total) * 100, 100) : 0;
  const elapsed = (Date.now() - startTime) / 1000;
  const eta = count > 0 && pct < 100
    ? Math.round((elapsed / count) * (total - count))
    : null;

  return (
    <Card variant="outlined">
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Icon size={18} strokeWidth={2.5} className={`text-pencil-light ${iconClassName}`} />
            <span className="font-medium text-pencil">
              {total > 0 ? labelRunning : labelDiscovering}
            </span>
          </div>
          {total > 0 && (
            <span className="text-sm text-pencil-light font-mono">
              {Math.round(pct)}%
            </span>
          )}
        </div>
        {total > 0 && (
          <>
            <div
              className="h-4 border-2 border-pencil-light/50 bg-paper-warm overflow-hidden"
              style={{ borderRadius: radius.sm }}
            >
              <div
                className="h-full transition-all duration-200 ease-out"
                style={{
                  width: `${pct}%`,
                  backgroundColor: palette.info,
                  borderRadius: radius.sm,
                }}
              />
            </div>
            <div className="flex items-center justify-between text-sm text-pencil-light">
              <span>{count} / {total} {units}</span>
              {eta !== null && <span>~{eta}s remaining</span>}
            </div>
          </>
        )}
      </div>
    </Card>
  );
}
