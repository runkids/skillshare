import { Download } from 'lucide-react';
import type { MCPUnmanaged } from '../../api/mcp';
import AgentStack from '../AgentStack';
import Button from '../Button';
import { useT, plural } from '../../i18n';
import { targetLabel } from './mcpView';

interface Props {
  entries: MCPUnmanaged[];
  /** Opens the import with this Agent's file selected: the first one that has entries. */
  onImport: (target: string) => void;
}

/** Servers already in Agent files that skillshare does not manage yet. Hidden when there are none. */
export default function MCPUnmanagedNote({ entries, onImport }: Props) {
  const t = useT();
  if (entries.length === 0) return null;
  const count = entries.reduce((n, e) => n + e.names.length, 0);
  const targets = entries.map((e) => e.target);
  return (
    <div className="ss-note inf !items-center">
      <AgentStack targets={targets} />
      <span className="flex-1">
        {t(plural('mcp.unmanaged', count), { count, targets: targets.map(targetLabel).join(', ') })}
      </span>
      <Button size="sm" variant="secondary" onClick={() => onImport(targets[0])}>
        <Download size={14} />
        {t('mcp.importAction')}
      </Button>
    </div>
  );
}
