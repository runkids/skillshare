import { FlaskConical, FileEdit } from 'lucide-react';
import type { CompiledRule } from '../../api/client';
import { severityBadgeVariant } from '../../lib/severity';
import Badge from '../Badge';
import Button from '../Button';
import CopyButton from '../CopyButton';

interface RuleDetailCardProps {
  rule: CompiledRule;
  onTestRegex: () => void;
  onEditInYaml: () => void;
}

function sourceBadgeVariant(source: string): 'default' | 'accent' | 'info' {
  switch (source) {
    case 'global': return 'accent';
    case 'project': return 'info';
    default: return 'default';
  }
}

export default function RuleDetailCard({ rule, onTestRegex, onEditInYaml }: RuleDetailCardProps) {
  return (
    <div className="animate-fade-in p-3 space-y-3">
      {/* Header */}
      <div>
        <Badge variant="info">
          <span className="font-mono truncate">{rule.id}</span>
        </Badge>
      </div>

      {/* Source row */}
      <div className="flex items-center gap-2">
        <span className="text-xs text-pencil-light">Source</span>
        <Badge variant={sourceBadgeVariant(rule.source)}>{rule.source}</Badge>
        {rule.source !== 'builtin' && (
          <Badge variant="warning">Overridden by {rule.source}</Badge>
        )}
      </div>

      {/* Disabled badge */}
      {!rule.enabled && (
        <div>
          <Badge variant="danger">Disabled</Badge>
        </div>
      )}

      {/* Separator */}
      <hr className="border-dashed border-pencil-light/30" />

      {/* Message */}
      <p className="text-sm text-pencil leading-relaxed">{rule.message}</p>

      {/* Severity */}
      <div>
        <Badge variant={severityBadgeVariant(rule.severity)}>{rule.severity}</Badge>
      </div>

      {/* Separator */}
      <hr className="border-dashed border-pencil-light/30" />

      {/* Regex */}
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <span className="text-xs uppercase tracking-wider text-pencil-light">Regex</span>
          <CopyButton value={rule.regex} />
        </div>
        <div className="bg-paper border border-dashed border-pencil-light/30 rounded-lg p-2 font-mono text-xs break-all">
          {rule.regex}
        </div>
      </div>

      {/* Exclude */}
      {rule.exclude && (
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="text-xs uppercase tracking-wider text-pencil-light">Exclude</span>
            <CopyButton value={rule.exclude} />
          </div>
          <div className="bg-paper border border-dashed border-pencil-light/30 rounded-lg p-2 font-mono text-xs break-all">
            {rule.exclude}
          </div>
        </div>
      )}

      {/* Separator */}
      <hr className="border-dashed border-pencil-light/30" />

      {/* Quick actions */}
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="sm" onClick={onTestRegex}>
          <FlaskConical size={14} />
          Test Regex
        </Button>
        <Button variant="ghost" size="sm" onClick={onEditInYaml}>
          <FileEdit size={14} />
          Edit in YAML
        </Button>
      </div>
    </div>
  );
}
