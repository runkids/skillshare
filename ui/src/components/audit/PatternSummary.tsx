import { Eye, EyeOff } from 'lucide-react';
import type { CompiledRule } from '../../api/client';
import { severityBadgeVariant, SEVERITY_ORDER } from '../../lib/severity';
import Badge from '../Badge';
import Button from '../Button';

interface PatternSummaryProps {
  pattern: string;
  rules: CompiledRule[];
  onTogglePattern: (pattern: string, enabled: boolean) => void;
  isToggling: boolean;
}

function getMaxSeverity(rules: CompiledRule[]): string {
  for (const sev of SEVERITY_ORDER) {
    if (rules.some(r => r.severity === sev)) return sev;
  }
  return 'INFO';
}

export default function PatternSummary({
  pattern,
  rules,
  onTogglePattern,
  isToggling,
}: PatternSummaryProps) {
  const enabledCount = rules.filter(r => r.enabled).length;
  const disabledCount = rules.length - enabledCount;
  const maxSeverity = getMaxSeverity(rules);

  const allEnabled = disabledCount === 0;
  const allDisabled = enabledCount === 0;

  return (
    <div className="animate-fade-in p-3 space-y-3">
      {/* Header */}
      <div className="flex items-center justify-between gap-2">
        <span className="text-base font-semibold text-pencil truncate">{pattern}</span>
        <Badge variant={severityBadgeVariant(maxSeverity)} size="sm">
          {maxSeverity}
        </Badge>
      </div>

      {/* Stats row */}
      <div className="flex items-center gap-2 text-xs">
        <span className="text-pencil-light">{rules.length} rules</span>
        <span className="text-pencil-light/40">·</span>
        <span className="text-success">{enabledCount} enabled</span>
        {disabledCount > 0 && (
          <>
            <span className="text-pencil-light/40">·</span>
            <span className="text-warning">{disabledCount} disabled</span>
          </>
        )}
      </div>

      {/* Separator */}
      <div className="border-t border-dashed border-pencil-light/30" />

      {/* Rule list */}
      <div className="space-y-0">
        {rules.map(rule => (
          <div key={rule.id} className="flex items-center gap-2 py-0.5">
            <span
              className={`w-2 h-2 rounded-full flex-shrink-0 ${
                rule.enabled ? 'bg-success' : 'bg-muted-dark'
              }`}
            />
            <span className="font-mono text-xs text-pencil truncate flex-1">{rule.id}</span>
            <Badge variant={severityBadgeVariant(rule.severity)} size="sm">
              {rule.severity}
            </Badge>
          </div>
        ))}
      </div>

      {/* Separator */}
      <div className="border-t border-dashed border-pencil-light/30" />

      {/* Bulk actions */}
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onTogglePattern(pattern, true)}
          disabled={allEnabled || isToggling}
        >
          <Eye size={14} strokeWidth={2} />
          Enable All
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onTogglePattern(pattern, false)}
          disabled={allDisabled || isToggling}
        >
          <EyeOff size={14} strokeWidth={2} />
          Disable All
        </Button>
      </div>
    </div>
  );
}
