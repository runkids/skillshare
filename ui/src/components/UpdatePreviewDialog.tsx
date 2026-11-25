import { useQuery } from '@tanstack/react-query';
import { ExternalLink, X } from 'lucide-react';
import { api, ApiError } from '../api/client';
import type { RepoUpstreamChanges } from '../api/client';
import { queryKeys } from '../lib/queryKeys';
import { compareUrl, sourcePage, type FilterUnit } from '../lib/updateFilters';
import { formatNumber, formatRelativeTime, plural, useI18n, useT } from '../i18n';
import Button from './Button';
import DialogShell from './DialogShell';
import FileDiffList, { LocalChanges } from './FileDiffList';
import Spinner from './Spinner';

interface UpdatePreviewDialogProps {
  /** The unit to preview; null closes the dialog. */
  unit: FilterUnit | null;
  /** upstream: what updating brings in. local: what was edited since install. */
  mode: 'upstream' | 'local';
  onClose: () => void;
  onUpdate: (opts: { force?: boolean; merge?: boolean }) => void;
}

/** Shows what an update would change before it runs, or a skill's own edits before they are overwritten. */
export default function UpdatePreviewDialog({
  unit,
  mode,
  onClose,
  onUpdate,
}: UpdatePreviewDialogProps) {
  const t = useT();
  const local = mode === 'local';
  const title = local ? t('update.check.dirty') : t('update.preview.title');
  return (
    <DialogShell open={!!unit} onClose={onClose} maxWidth="3xl" padding="none" ariaLabel={title}>
      {unit && (
        <>
          <div className="dh flex items-start gap-3">
            <div className="flex flex-col gap-1 min-w-0 flex-1">
              <h2 className="ss-h2 font-mono break-all">{unit.label}</h2>
              <span className="text-[13px] text-ink-2">{title}</span>
            </div>
            <button
              type="button"
              className="ss-ib shrink-0"
              aria-label={t('common.close')}
              onClick={onClose}
            >
              <X size={16} />
            </button>
          </div>
          <div className="db text-ink-2 overflow-y-auto">
            {local ? (
              <>
                <p className="m-0">
                  {t('resourceDetail.modified.message', { name: unit.label })}{' '}
                  {t('resourceDetail.modified.mergeHint')}
                </p>
                <LocalChanges relPath={unit.name} />
              </>
            ) : unit.isRepo ? (
              <RepoChanges unit={unit} />
            ) : (
              <SkillChanges unit={unit} />
            )}
          </div>
          <div className="df flex-wrap">
            {local ? (
              <>
                <Button variant="ghost" onClick={onClose}>
                  {t('common.cancel')}
                </Button>
                <Button variant="secondary" onClick={() => onUpdate({ merge: true })}>
                  {t('update.updating.merge')}
                </Button>
                <Button variant="danger" onClick={() => onUpdate({ force: true })}>
                  {t('update.tab.force')}
                </Button>
              </>
            ) : (
              <>
                <span className="mr-auto text-[13px] text-ink-3">{t('update.tab.audited')}</span>
                <Button variant="ghost" onClick={onClose}>
                  {t('common.close')}
                </Button>
                <Button variant="primary" onClick={() => onUpdate({})}>
                  {t('update.row.update')}
                </Button>
              </>
            )}
          </div>
        </>
      )}
    </DialogShell>
  );
}

function Loading() {
  return (
    <div className="flex justify-center py-6">
      <Spinner />
    </div>
  );
}

function LoadFailed({ error }: { error: Error }) {
  const t = useT();
  // 404 and 409 are skillshare's own refusals; anything else is a git diagnostic, shown as is.
  const status = error instanceof ApiError ? error.status : 0;
  const reason =
    status === 404
      ? t('update.preview.noRecord')
      : status === 409
        ? t('update.preview.noUpstream')
        : t('update.preview.loadFailed', { message: error.message });
  return <div className="ss-note bad">{reason}</div>;
}

function SourceLink({ link }: { link: { url: string; host: string } | null }) {
  const t = useT();
  if (!link) return null;
  const href = link.url;
  return (
    <a
      className="ss-btn sm ghost self-start"
      href={href}
      target="_blank"
      rel="noreferrer"
      title={href}
    >
      <ExternalLink size={14} />
      {t('update.preview.viewOn', { host: link.host })}
    </a>
  );
}

/** A skill installed from a repo: its files at the installed commit against the latest. */
function SkillChanges({ unit }: { unit: FilterUnit }) {
  const t = useT();
  const { locale } = useI18n();
  const { data, error, isPending } = useQuery({
    queryKey: queryKeys.skills.upstreamChanges(unit.name),
    queryFn: () => api.getUpstreamChanges(unit.name),
    staleTime: 0,
  });
  if (isPending) return <Loading />;
  if (error) return <LoadFailed error={error} />;
  const page = sourcePage(unit, 'commits');
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <span className="font-mono text-[12.5px]">
          {data.from.slice(0, 7)} → {data.to.slice(0, 7)}
        </span>
        <span className="text-[13px]">
          {data.files.length > 0
            ? t(plural('update.preview.files', data.files.length), {
                count: formatNumber(data.files.length, locale),
              })
            : t('update.preview.noFileChanges')}
        </span>
        <SourceLink link={page} />
      </div>
      <FileDiffList files={data.files} />
    </div>
  );
}

/** A tracked repo: the commits a pull brings in and the files they touch. */
function RepoChanges({ unit }: { unit: FilterUnit }) {
  const { data, error, isPending } = useQuery({
    queryKey: queryKeys.repoUpstreamChanges(unit.name),
    queryFn: () => api.getRepoUpstreamChanges(unit.name),
    staleTime: 0,
  });
  if (isPending) return <Loading />;
  if (error) return <LoadFailed error={error} />;
  return <RepoChangeList data={data} compare={compareUrl(unit, data.from, data.to)} />;
}

function RepoChangeList({
  data,
  compare,
}: {
  data: RepoUpstreamChanges;
  compare: { url: string; host: string } | null;
}) {
  const t = useT();
  const { locale } = useI18n();
  const more = data.total_commits - data.commits.length;
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <span className="text-[13px]">
          {t(plural('update.preview.commits', data.total_commits), {
            count: formatNumber(data.total_commits, locale),
          })}{' '}
          ·{' '}
          {t(plural('update.preview.files', data.files.length), {
            count: formatNumber(data.files.length, locale),
          })}
        </span>
        <SourceLink link={compare} />
      </div>
      <div className="ss-list">
        {data.commits.map((c) => (
          <div key={c.hash} className="ss-r !min-h-0 items-baseline gap-3 py-2.5">
            <span className="font-mono text-xs text-ink-3 shrink-0">{c.hash}</span>
            <span className="flex-1 min-w-0 text-[13px] text-ink break-words">{c.subject}</span>
            <span className="hidden sm:inline shrink-0 text-xs text-ink-3">
              {c.author} · {formatRelativeTime(c.date, locale)}
            </span>
          </div>
        ))}
        {more > 0 && (
          <div className="ss-r !min-h-0 py-2.5 text-[13px] text-ink-3">
            {t(plural('update.preview.moreCommits', more), { count: formatNumber(more, locale) })}
          </div>
        )}
      </div>
      {data.files.length > 0 && (
        <div className="ss-list">
          {data.files.map((f) => (
            <div key={f.path} className="ss-r !min-h-0 gap-3 py-2">
              <span
                className="flex-1 min-w-0 font-mono text-[12.5px] text-ink truncate"
                title={f.path}
              >
                {f.path}
              </span>
              {f.binary ? (
                <span className="shrink-0 text-xs text-ink-3">
                  {t('update.preview.binaryShort')}
                </span>
              ) : (
                <span className="shrink-0 font-mono text-xs tabular-nums">
                  <span className="text-ok">+{f.added}</span>{' '}
                  <span className="text-bad">−{f.deleted}</span>
                </span>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
