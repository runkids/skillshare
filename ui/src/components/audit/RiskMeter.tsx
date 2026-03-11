import { radius } from '../../design';
import { riskColor, riskBgColor } from './helpers';

export default function RiskMeter({ riskLabel, riskScore }: { riskLabel: string; riskScore: number }) {
  const color = riskColor(riskLabel);

  return (
    <div
      className="flex items-center gap-2 px-3 py-1.5 border"
      style={{
        borderRadius: radius.sm,
        borderColor: color,
        backgroundColor: riskBgColor(riskLabel),
      }}
    >
      <div className="flex flex-col items-start gap-0.5">
        <span className="text-[10px] text-pencil-light uppercase tracking-wide leading-none">
          Risk
        </span>
        <span
          className="text-sm font-bold leading-none"
          style={{ color }}
        >
          {riskLabel.toUpperCase()}
        </span>
      </div>
      {/* Mini bar */}
      <div className="flex flex-col items-end gap-0.5">
        <div
          className="w-12 h-1.5 bg-muted/50 overflow-hidden"
          style={{ borderRadius: '999px' }}
        >
          <div
            className="h-full"
            style={{
              width: `${riskScore}%`,
              backgroundColor: color,
              borderRadius: '999px',
            }}
          />
        </div>
        <span className="text-[10px] text-pencil-light leading-none font-mono">{riskScore}/100</span>
      </div>
    </div>
  );
}
