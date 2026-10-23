import { TriangleAlert } from 'lucide-react';
import { Link } from 'react-router-dom';
import type { FolderConflict, Target } from '../../api/client';
import { useI18n, useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import Button from '../Button';
import { projectUrl } from '../projects/projectView';
import { NAMES, TargetLinks } from '../targets/FilterSection';
import { joinList } from '../targets/targetView';

/** Include and exclude as a short phrase; sorted, because the server compares them as sets. */
function filterSummary(x: Target, t: ReturnType<typeof useT>, locale: string) {
  const parts = [
    x.include.length > 0 && t('sync.folderConflict.filters.include', { names: joinList([...x.include].sort(), locale) }),
    x.exclude.length > 0 && t('sync.folderConflict.filters.exclude', { names: joinList([...x.exclude].sort(), locale) }),
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(' · ') : t('sync.folderConflict.filters.all');
}

/** A folder that targets sync into with different settings: what to stop, and which settings differ. */
export default function FolderConflictNotice({ conflict, targets, onStop }: { conflict: FolderConflict; targets: Target[]; onStop: (name: string) => void }) {
  const t = useT();
  const { locale } = useI18n();
  // A project target's settings belong to its project; the target API refuses them.
  const projectPage = (name: string) => {
    const project = targets.find((x) => x.name === name)?.project;
    return project ? projectUrl(project) : undefined;
  };
  // The target kept first, as its column.
  const columns = [conflict.keep, ...conflict.stop].map((name) => targets.find((x) => x.name === name)).filter((x): x is Target => !!x);
  // Symlink links the whole folder, so naming and filters say nothing there (null).
  const rows = [
    { key: 'mode', label: 'targetDetail.syncMode', tag: true, value: (x: Target) => x.mode },
    { key: 'naming', label: 'targetDetail.naming', tag: true, value: (x: Target) => (x.mode === 'symlink' ? null : x.targetNaming) },
    { key: 'filters', label: 'sync.folderConflict.filters', tag: false, value: (x: Target) => (x.mode === 'symlink' ? null : filterSummary(x, t, locale)) },
  ].filter((r) => new Set(columns.map(r.value).filter((v) => v !== null)).size > 1);

  return (
    <div className="ss-note warn flex-col !items-stretch">
      <div className="flex items-start gap-2.5">
        <TriangleAlert size={16} className="mt-0.5 shrink-0 text-warn" />
        <span className="min-w-0 flex-1">{t('sync.folderConflict.text', { names: joinList(conflict.targets, locale), path: shortenHome(conflict.path) })}</span>
      </div>
      <div className="flex flex-wrap gap-2 pl-[26px]">
        {conflict.stop.map((name) => {
          const page = projectPage(name);
          return page ? (
            <Link key={name} to={page} className="ss-btn sm">{t('sync.folderConflict.editProject', { name })}</Link>
          ) : (
            <Button key={name} variant="secondary" size="sm" onClick={() => onStop(name)}>
              {t('sync.folderConflict.stop', { name })}
            </Button>
          );
        })}
      </div>
      {rows.length > 0 && (
        <div className="flex flex-col gap-2 pl-[26px]">
          <table className="w-full table-fixed border-collapse text-left [border-top:var(--sep)]">
            <colgroup><col className="w-24" />{columns.map((x) => <col key={x.name} />)}</colgroup>
            <thead>
              <tr>
                <td aria-hidden="true" className="pb-1 pr-4 pt-2" />
                {columns.map((x) => (
                  <th key={x.name} scope="col" className="break-words pb-1 pr-4 pt-2 text-[12px] font-medium text-ink-3">
                    {x.name === conflict.keep ? t('sync.folderConflict.keeps', { name: x.name }) : x.name}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.key}>
                  <th scope="row" className="py-1.5 pr-4 text-left align-top font-semibold">{t(r.label)}</th>
                  {columns.map((x) => {
                    const v = r.value(x);
                    return (
                      <td key={x.name} className="break-words py-1.5 pr-4 align-top text-ink-2">
                        {v === null ? t('sync.folderConflict.na') : r.tag ? <span className="ss-tag">{v}</span> : v}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
          <div className="text-ink-2"><TargetLinks sentence={t('sync.folderConflict.hint', { names: NAMES })} names={conflict.stop} href={projectPage} /></div>
        </div>
      )}
    </div>
  );
}
