import { TriangleAlert } from 'lucide-react';
import type { SharedName } from '../../api/client';
import { useI18n, useT } from '../../i18n';
import { joinList } from '../targets/targetView';

/** Skills that share a name on a target, so all but a top-level one sync under their full path names. */
export default function SharedNameNotices({ items }: { items?: SharedName[] | null }) {
  const t = useT();
  const { locale } = useI18n();
  if (!items?.length) return null;

  return (
    <section className="ss-list">
      {items.map((s) => {
        const params = { target: s.target, name: s.name, paths: joinList(s.paths, locale) };
        return (
          <div key={s.target + s.name} className="ss-r !min-h-0 bg-warn-bg text-[13px]">
            <TriangleAlert size={16} className="shrink-0 text-warn" />
            <span className="flex-1 break-words">
              {t(s.same_name ? 'sync.sharedName.same' : 'sync.sharedName.differ', params)}
            </span>
          </div>
        );
      })}
    </section>
  );
}
