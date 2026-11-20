import { Link } from 'react-router-dom';
import { TriangleAlert } from 'lucide-react';
import type { UnmatchedInclude } from '../../api/client';
import { plural, useI18n, useT } from '../../i18n';
import { projectUrl } from '../projects/projectView';
import { joinList } from '../targets/targetView';

/** Targets sharing a filter that selects nothing, as one notice with a link to where the filter is edited. */
export default function UnmatchedNotices({ items = [] }: { items?: UnmatchedInclude[] }) {
  const t = useT();
  const { locale } = useI18n();
  const groups = new Map<string, UnmatchedInclude[]>();
  for (const u of items) {
    // Only a project's targets share one filter to edit; other targets stay apart.
    const key = JSON.stringify([u.root ?? u.target, u.patterns, u.suggestions ?? [], u.all]);
    groups.set(key, [...(groups.get(key) ?? []), u]);
  }
  if (groups.size === 0) return null;

  return (
    <section className="ss-list">
      {[...groups].map(([key, group]) => {
        const first = group[0];
        // A project target is named project@tool; the notice names the project once.
        const at = first.root ? first.target.lastIndexOf('@') : -1;
        const targets =
          at > 0
            ? t('sync.unmatched.inProject', {
                project: first.target.slice(0, at),
                tools: joinList(
                  group.map((u) => u.target.slice(u.target.lastIndexOf('@') + 1)),
                  locale,
                ),
              })
            : joinList(
                group.map((u) => u.target),
                locale,
              );
        const patterns = joinList(
          first.patterns.map((p) => `"${p}"`),
          locale,
        );
        const suggestions = first.suggestions ?? [];
        return (
          <div key={key} className="ss-r !min-h-0 bg-warn-bg text-[13px]">
            <TriangleAlert size={16} className="shrink-0 text-warn" />
            <span className="flex-1 break-words">
              {first.all
                ? t(plural('sync.unmatched.all', group.length), { targets, patterns })
                : t('sync.unmatched.some', { targets, patterns })}
              {suggestions.length > 0 && (
                <>
                  {' '}
                  {t('sync.unmatched.suggest', {
                    names: joinList(
                      suggestions.map((n) => `"${n}"`),
                      locale,
                    ),
                  })}
                </>
              )}
            </span>
            <Link
              to={
                first.root ? projectUrl(first.root) : `/targets/${encodeURIComponent(first.target)}`
              }
              className="ss-btn sm shrink-0"
            >
              {t('sync.unmatched.edit')}
            </Link>
          </div>
        );
      })}
    </section>
  );
}
