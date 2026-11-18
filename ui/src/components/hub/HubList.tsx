import { Star } from 'lucide-react';
import AddHubMenu from './AddHubMenu';
import { useT } from '../../i18n';

export interface HubListItem {
  key: string;
  label: string;
  sub: string;
  mine: boolean;
  isDefault: boolean;
}

interface Props {
  items: HubListItem[];
  selected: string | null;
  onPick: (key: string) => void;
  /** While a hub is being edited the list stays visible but cannot be used. */
  locked: boolean;
  onAdd: (url: string) => Promise<void>;
  onCreate: () => void;
  onImport: (file: File) => void;
}

/** Every hub in one list: the built-in one, saved subscriptions and the user's own. */
export default function HubList({
  items,
  selected,
  onPick,
  locked,
  onAdd,
  onCreate,
  onImport,
}: Props) {
  const t = useT();
  return (
    <div className="sticky top-6 z-20 flex flex-col gap-3">
      <div className="ss-sec !mb-0">
        <h2>{t('hubs.browse.sources')}</h2>
        <span className="ss-cnt">{items.length}</span>
      </div>
      <div
        className={`ss-list ${locked ? 'pointer-events-none opacity-55' : ''}`}
        aria-disabled={locked || undefined}
      >
        {items.map((hub) => (
          <button
            key={hub.key}
            type="button"
            disabled={locked}
            aria-current={hub.key === selected}
            className={`ss-r !min-h-[52px] w-full !flex-col !items-start !gap-0.5 text-start ${hub.key === selected ? 'sel' : ''}`}
            onClick={() => onPick(hub.key)}
          >
            <span className="flex items-center gap-1.5 text-[13.5px] font-semibold">
              {hub.label}
              {hub.isDefault && (
                <Star size={12} className="fill-current text-warn" aria-label={t('hubs.default')} />
              )}
              {hub.mine && <span className="ss-tag inf">{t('hubs.mine')}</span>}
            </span>
            <span className="w-full truncate font-mono text-[11.5px] text-ink-3">{hub.sub}</span>
          </button>
        ))}
      </div>
      {locked ? (
        <span className="text-xs text-ink-3">{t('hubs.edit.locked')}</span>
      ) : (
        <AddHubMenu onAdd={onAdd} onCreate={onCreate} onImport={onImport} />
      )}
    </div>
  );
}
