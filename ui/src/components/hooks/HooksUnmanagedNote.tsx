import type { HookUnmanaged } from '../../api/hooks';
import { useT, plural } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { hookLabel } from './hooksView';

/** Hooks already in target files that Skillshare does not manage yet, one line per target. Hidden when there are none. */
export default function HooksUnmanagedNote({
  entries,
  onImport,
}: {
  entries: HookUnmanaged[];
  onImport: () => void;
}) {
  const t = useT();
  if (entries.length === 0) return null;
  // One target can hold hooks in several files; count the importable ones per target.
  const counts = new Map<string, number>();
  for (const e of entries) {
    if (e.importable) counts.set(e.target, (counts.get(e.target) ?? 0) + e.names.length);
  }
  // Sources Skillshare only lists stay visible, but Review cannot take them over.
  const listed = entries.filter((e) => !e.importable);
  return (
    <div className="ss-box flex shrink-0 flex-col gap-3">
      <h3 className="text-[15px] font-semibold">
        {t(counts.size > 0 ? 'hooks.unmanaged.title' : 'hooks.listed.title')}
      </h3>
      {[...counts].map(([target, count]) => (
        <span key={target} className="flex items-center gap-2 text-[13px] text-ink-2">
          <AgentIcon target={target} size={15} />
          <span className="min-w-0 flex-1 [text-wrap:balance]">
            {t(plural('hooks.unmanaged', count), {
              count: String(count),
              target: hookLabel(target),
            })}
          </span>
        </span>
      ))}
      {listed.map((e) => (
        <span
          key={`${e.target}:${e.path}`}
          className="flex items-center gap-2 text-[13px] text-ink-3"
        >
          <AgentIcon target={e.target} size={15} />
          <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">
            {t(plural('hooks.listed', e.names.length), {
              count: String(e.names.length),
              target: hookLabel(e.target),
              file: shortenHome(e.path),
            })}
          </span>
        </span>
      ))}
      {counts.size > 0 && (
        <Button size="sm" variant="secondary" className="self-start" onClick={onImport}>
          {t('hooks.unmanaged.review')}
        </Button>
      )}
    </div>
  );
}
