import { AlertTriangle, Globe, KeyRound, Terminal, Trash2 } from 'lucide-react';
import { mcpTargets } from '../../api/mcp';
import Badge from '../Badge';
import Button from '../Button';
import Card from '../Card';
import IconButton from '../IconButton';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import type { MCPResolve } from './MCPPreview';
import { describeCredentials, describeEndpoint, describeMessage, isResolvable, statusVariant, type MatrixRow } from './mcpView';

interface Props {
  rows: MatrixRow[];
  busy: boolean;
  onEdit: (name: string) => void;
  onRemove: (name: string) => void;
  onResolve: MCPResolve;
}

export default function MCPMatrix({ rows, busy, onEdit, onRemove, onResolve }: Props) {
  const t = useT();
  return <Card padding="none">
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b-2 border-dashed border-muted-dark text-pencil-light">
            <th scope="col" className="px-4 py-3 font-medium">{t('mcp.server')}</th>
            {mcpTargets.map(target => <th key={target} scope="col" className="w-24 px-1 py-3 font-medium text-center">
              <span className="inline-flex flex-col items-center gap-1"><AgentIcon target={target} size={18} />{target}</span>
            </th>)}
            <th scope="col" className="w-32 px-4 py-3"><span className="sr-only">{t('mcp.actions')}</span></th>
          </tr>
        </thead>
        {rows.map(row => {
          const conflicts = Object.values(row.cells).filter(change => change.action === 'conflict');
          const endpoint = row.server ? describeEndpoint(row.server) : '';
          return <tbody key={row.name} className="border-b border-dashed border-pencil-light/30 last:border-0">
            <tr className={row.server ? '' : 'bg-paper/60'}>
              <th scope="row" className="px-4 py-3 font-normal align-middle">
                <div className="space-y-1.5 min-w-56">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={`text-base font-semibold ${row.server ? 'text-pencil' : 'text-pencil-light line-through'}`}>{row.name}</span>
                    {row.server ? null : <Badge variant="danger">{t('mcp.removedFromSource')}</Badge>}
                  </div>
                  {row.server ? <>
                    <div className="flex items-center gap-2 min-w-0">
                      <Badge>{row.server.url ? <Globe size={10} aria-hidden="true" /> : <Terminal size={10} aria-hidden="true" />}<span className="whitespace-nowrap">{t(row.server.url ? 'mcp.remote' : 'mcp.local')}</span></Badge>
                      <span className="font-mono text-xs text-pencil-light truncate max-w-xs" title={endpoint}>{endpoint}</span>
                    </div>
                    {describeCredentials(row.server).map(credential => <span key={credential} className="inline-flex items-center gap-1 me-1.5 px-1.5 font-mono text-[11px] text-pencil-light whitespace-nowrap border border-muted rounded-[var(--radius-sm)]">
                      <KeyRound size={10} aria-hidden="true" />{credential}
                    </span>)}
                  </> : <p className="text-xs text-pencil-light">{t('mcp.removedHint')}</p>}
                </div>
              </th>
              {mcpTargets.map(target => {
                const change = row.cells[target];
                return <td key={target} className="px-1 py-3 text-center">
                  {change
                    ? <Badge size="md" variant={statusVariant[change.action]}>{t(`mcp.status.${change.action}`)}</Badge>
                    : <><span aria-hidden="true" className="text-pencil-light">—</span><span className="sr-only">{t('mcp.notTargeted', { target })}</span></>}
                </td>;
              })}
              <td className="px-4 py-3">
                {row.server ? <div className="flex items-center justify-end gap-1">
                  <Button size="sm" variant="secondary" onClick={() => onEdit(row.name)}>{t('mcp.edit')}</Button>
                  <IconButton icon={<Trash2 size={16} />} label={`${t('mcp.remove')} ${row.name}`} variant="danger-outline" onClick={() => onRemove(row.name)} />
                </div> : null}
              </td>
            </tr>
            {conflicts.map(change => <tr key={change.target}>
              <td colSpan={mcpTargets.length + 2} className="px-4 pb-3">
                <div className="flex flex-wrap items-center justify-between gap-3 px-3 py-2 bg-warning-light rounded-[var(--radius-md)]">
                  <p className="flex flex-wrap items-center gap-2 text-sm text-pencil">
                    <AlertTriangle size={16} className="text-warning" aria-hidden="true" />
                    <span className="font-semibold">{change.target}</span>
                    <span>{describeMessage(t, change.message)}</span>
                    <span className="font-mono text-xs text-pencil-light">{change.path}</span>
                  </p>
                  {isResolvable(change) ? <div className="flex gap-2">
                    <Button size="sm" variant="secondary" disabled={busy} onClick={() => onResolve(change.target, change.name, 'import')}>{t('mcp.importFromAgent', { target: change.target })}</Button>
                    <Button size="sm" variant="secondary" loading={busy} onClick={() => onResolve(change.target, change.name, 'replace')}>{t('mcp.replace')}</Button>
                  </div> : null}
                </div>
              </td>
            </tr>)}
          </tbody>;
        })}
      </table>
    </div>
    <div className="flex flex-wrap items-center gap-3 px-4 py-3 text-xs text-pencil-light border-t border-dashed border-pencil-light/30">
      <Badge variant="success">{t('mcp.status.unchanged')}</Badge>
      <Badge variant="info">{t('mcp.status.add')} / {t('mcp.status.update')}</Badge>
      <Badge variant="warning">{t('mcp.status.conflict')}</Badge>
      <Badge variant="danger">{t('mcp.status.remove')}</Badge>
      <span>— {t('mcp.legendNone')}</span>
    </div>
  </Card>;
}
