import { Download } from 'lucide-react';
import type { HookUnmanaged } from '../../api/hooks';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { hookLabel } from './hooksView';

/** Hooks already in Agent files that Skillshare does not manage yet. Hidden when there are none. */
export default function HooksUnmanagedNote({ entries, onImport }: { entries: HookUnmanaged[]; onImport: (target: string) => void }) {
  const t = useT();
  if (entries.length === 0) return null;
  const count = entries.reduce((n, e) => n + e.names.length, 0);
  // One file per Agent can hold hooks in several places; show each Agent once but count every hook.
  const targets = [...new Set(entries.map((e) => e.target))];
  return (
    <div className="ss-note inf !items-center">
      <span className="ss-stack">{targets.map((x) => <span key={x} className="ss-at"><AgentIcon target={x} size={13} /></span>)}</span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span>{t(count === 1 ? 'hooks.unmanaged.one' : 'hooks.unmanaged.other', { count, targets: targets.map(hookLabel).join(', ') })}</span>
        {entries.map((e) => <span key={`${e.target}:${e.path}`} className="truncate font-mono text-xs opacity-75" title={e.path}>{shortenHome(e.path)}</span>)}
      </span>
      <Button size="sm" variant="secondary" onClick={() => onImport(targets[0])}><Download size={14} />{t('mcp.importAction')}</Button>
    </div>
  );
}
