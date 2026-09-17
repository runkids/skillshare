import { useState } from 'react';
import { AlertTriangle } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import type { MCPPlan } from '../../api/mcp';
import Badge from '../Badge';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import { useT } from '../../i18n';
import { countActions, describeMessage, groupByFile, isResolvable, statusVariant } from './mcpView';

export type MCPResolve = (target: string, name: string, action: 'import' | 'replace') => void;

interface Props {
  plan: MCPPlan;
  onResolve?: MCPResolve;
  busy?: boolean;
}

export default function MCPPreview({ plan, onResolve, busy = false }: Props) {
  const t = useT();
  const [hideSynced, setHideSynced] = useState(true);
  const counts = countActions(plan.changes);
  const canHide = Boolean(counts.unchanged) && counts.unchanged < plan.changes.length;
  const files = groupByFile(plan.changes.filter(change => !(canHide && hideSynced && change.action === 'unchanged')));

  return <div className="space-y-3">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="flex flex-wrap gap-2">
        {Object.entries(counts).map(([action, count]) => <Badge key={action} size="md" variant={statusVariant[action]}>{t(`mcp.status.${action}`)} {count}</Badge>)}
      </div>
      {canHide ? <Checkbox size="sm" label={t('mcp.hideSynced')} checked={hideSynced} onChange={setHideSynced} /> : null}
    </div>
    {plan.blocked ? <p role="alert" className="flex items-start gap-2 p-3 text-sm text-pencil bg-warning-light rounded-[var(--radius-md)]">
      <AlertTriangle size={16} className="text-warning shrink-0 mt-0.5" aria-hidden="true" />{t('mcp.conflictHint')}
    </p> : null}
    <div className="space-y-3 max-h-[50vh] overflow-auto">
      {files.map(file => <section key={file.path} aria-label={file.target} className="border border-muted rounded-[var(--radius-md)] overflow-hidden">
        <header className="flex flex-wrap items-center gap-2 px-3 py-2 bg-paper text-sm">
          <AgentIcon target={file.target} />
          <span className="font-semibold">{file.target}</span>
          <span className="font-mono text-xs text-pencil-light break-all">{file.path}</span>
        </header>
        <ul className="divide-y divide-dashed divide-pencil-light/30">
          {file.changes.map(change => <li key={change.name} className="px-3 py-2.5 space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <Badge size="md" variant={statusVariant[change.action]}>{t(`mcp.status.${change.action}`)}</Badge>
              <span className="font-medium">{change.name}</span>
              {change.message ? <span className="text-sm text-pencil-light">{describeMessage(t, change.message)}</span> : null}
            </div>
            {onResolve && isResolvable(change) ? <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" variant="secondary" disabled={busy} onClick={() => onResolve(change.target, change.name, 'import')}>{t('mcp.importFromAgent', { target: change.target })}</Button>
              <Button size="sm" variant="secondary" loading={busy} onClick={() => onResolve(change.target, change.name, 'replace')}>{t('mcp.replace')}</Button>
              <span className="text-xs text-pencil-light">{t('mcp.resolveHint', { target: change.target })}</span>
            </div> : null}
          </li>)}
        </ul>
      </section>)}
    </div>
  </div>;
}
