import { ShieldCheck } from 'lucide-react';
import Badge from '../Badge';
import { severityBadgeVariant, severityColor, SEVERITY_ORDER } from '../../lib/severity';
import type { CompiledRule } from '../../api/client';

interface AuditOverviewProps {
  stats: { total: number; enabled: number; disabled: number; custom: number; patterns: number };
  compiledRules?: CompiledRule[];
}

export default function AuditOverview({ stats, compiledRules }: AuditOverviewProps) {
  // Count per severity from compiledRules
  const severityCounts = compiledRules
    ? SEVERITY_ORDER.reduce<Record<string, number>>((acc, sev) => {
        acc[sev] = compiledRules.filter(r => r.severity === sev).length;
        return acc;
      }, {})
    : null;

  return (
    <div className="animate-fade-in p-3 space-y-3">
      {/* Header */}
      <div className="flex items-center gap-1.5">
        <ShieldCheck size={13} strokeWidth={2} className="text-pencil-light" />
        <span className="text-xs uppercase tracking-wider text-pencil-light font-medium">Overview</span>
      </div>

      <div className="border-t border-dashed border-pencil-light/30" />

      {/* Stats grid */}
      <div className="grid grid-cols-2 gap-x-4 gap-y-2">
        <div className="flex items-center justify-between">
          <span className="text-xs text-pencil-light">Total</span>
          <span className="text-xs font-semibold text-pencil">{stats.total}</span>
        </div>

        <div className="flex items-center justify-between">
          <span className="text-xs text-pencil-light">Patterns</span>
          <span className="text-xs text-pencil-light">{stats.patterns}</span>
        </div>

        <div className="flex items-center justify-between">
          <span className="text-xs text-pencil-light">Enabled</span>
          <Badge variant="success">{stats.enabled}</Badge>
        </div>

        <div className="flex items-center justify-between">
          <span className="text-xs text-pencil-light">Custom</span>
          <Badge variant="info">{stats.custom}</Badge>
        </div>

        <div className="flex items-center justify-between col-span-2">
          <span className="text-xs text-pencil-light">Disabled</span>
          <Badge variant="warning">{stats.disabled}</Badge>
        </div>
      </div>

      {/* Severity distribution */}
      {severityCounts && (
        <>
          <div className="border-t border-dashed border-pencil-light/30" />

          <div className="space-y-1.5">
            {SEVERITY_ORDER.filter(sev => severityCounts[sev] > 0).map(sev => (
              <div key={sev} className="flex items-center gap-2">
                <span
                  className="w-1.5 h-1.5 rounded-full flex-shrink-0"
                  style={{ backgroundColor: severityColor(sev) }}
                />
                <span className="text-xs text-pencil-light flex-1">{sev}</span>
                <Badge variant={severityBadgeVariant(sev)}>{severityCounts[sev]}</Badge>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
