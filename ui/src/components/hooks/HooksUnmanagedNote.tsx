import { Download } from 'lucide-react';
import type { HookUnmanaged } from '../../api/hooks';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { hookLabel } from './hooksView';

/** Hooks already in target files that Skillshare does not manage yet, one line per target. Hidden when there are none. */
export default function HooksUnmanagedNote({ entries, onImport }: { entries: HookUnmanaged[]; onImport: () => void }) {
  const t = useT();
  if (entries.length === 0) return null;
  // One target can hold hooks in several files; count them per target.
  const counts = new Map<string, number>();
  for (const e of entries) counts.set(e.target, (counts.get(e.target) ?? 0) + e.names.length);
  return (
    <div className="ss-box flex shrink-0 flex-col gap-3">
      <h3 className="text-[15px] font-semibold">{t('hooks.unmanaged.title')}</h3>
      {[...counts].map(([target, count]) => (
        <span key={target} className="flex items-center gap-2 text-[13px] text-ink-2">
          <AgentIcon target={target} size={15} />
          <span className="min-w-0 flex-1">{t(count === 1 ? 'hooks.unmanaged.one' : 'hooks.unmanaged.other', { count: String(count), target: hookLabel(target) })}</span>
        </span>
      ))}
      <Button size="sm" variant="secondary" className="self-start" onClick={onImport}><Download size={14} />{t('hooks.unmanaged.review')}</Button>
    </div>
  );
}
