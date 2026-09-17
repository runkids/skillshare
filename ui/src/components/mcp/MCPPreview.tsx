import type { MCPPlan } from '../../api/mcp';
import Badge from '../Badge';
import { useT } from '../../i18n';

export default function MCPPreview({ plan }: { plan: MCPPlan }) {
  const t = useT();
  return <div className="space-y-3">
    <p className="text-sm text-pencil-light break-all">{plan.sourcePath}</p>
    <ul className="space-y-2 max-h-72 overflow-auto" aria-label={t('mcp.preview')}>
      {plan.changes.map(change => <li key={`${change.path}:${change.name}`} className="border border-muted rounded-lg p-3">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={change.action === 'conflict' ? 'warning' : 'default'}>{t(`mcp.${change.action}`)}</Badge>
          <span>{change.name}</span><span className="text-pencil-light">{change.target}</span>
        </div>
        <p className="text-xs text-pencil-light break-all mt-1">{change.path}</p>
        {change.message ? <p className="text-sm text-warning mt-1">{change.message}</p> : null}
      </li>)}
    </ul>
  </div>;
}
