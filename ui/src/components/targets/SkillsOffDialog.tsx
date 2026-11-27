import { useState } from 'react';
import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Check, Copy, Folder, TriangleAlert, Unlink } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { api } from '../../api/client';
import type { Target } from '../../api/client';
import { queryKeys } from '../../lib/queryKeys';
import { shortenHome } from '../../lib/paths';
import { useI18n, plural } from '../../i18n';
import Button from '../Button';
import DialogShell from '../DialogShell';
import AgentStack from '../AgentStack';
import Spinner from '../Spinner';
import { joinList } from './targetView';

const KEEP_NAMES = 3;
const READER_ICONS = 5;

/** Turning skills off removes only the links skillshare made; the preview lists them before anything is written. */
export interface SkillsReaders {
  /** Targets with skills off that read only this folder: they lose these skills */
  off: string[];
  /** Tools on this machine, not configured, that read this folder directly: they lose them too */
  local: string[];
  /** Targets with skills on that also read this folder: their own folder keeps a copy */
  on: string[];
}

export default function SkillsOffDialog({
  target,
  readFrom,
  readers,
  managed,
  onClose,
  onStopped,
}: {
  target: Target;
  /** Who else reads this target's skills folder */
  readers?: SkillsReaders;
  /** An enabled target whose skills folder this tool also reads */
  readFrom?: Target;
  /** The other content this target keeps getting, as its tabs name it */
  managed: string[];
  onClose: () => void;
  onStopped: (removed: number) => void;
}) {
  const { t, locale } = useI18n();
  const preview = useQuery({
    queryKey: queryKeys.targets.skillsOffPreview(target.name),
    queryFn: () => api.skillsOffPreview(target.name),
    staleTime: 0,
  });
  const [showRemoved, setShowRemoved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const title = t('targetDetail.skillsOff.confirmTitle', { name: target.name });
  const path = shortenHome(target.path);

  const stop = async () => {
    setBusy(true);
    setError('');
    try {
      const result = await api.updateTarget(target.name, { skills_enabled: false });
      onStopped(result.detach?.removed.length ?? 0);
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  };

  const remove = preview.data?.remove ?? [];
  const affected = Boolean(readers && (readers.off.length > 0 || readers.local.length > 0));
  const keep = preview.data?.keep ?? [];
  const copies = preview.data?.copies ?? [];
  const names = (list: string[]) =>
    list.slice(0, KEEP_NAMES).join(', ') + (list.length > KEEP_NAMES ? ', …' : '');
  // A few overlapping logos per line with every name on hover, the rest counted, so a long list stays short.
  const readerLine = (text: string, names: string[], muted = false) => (
    <span className={`flex flex-wrap items-center gap-2 ${muted ? 'text-ink-2' : ''}`}>
      {text}
      <span className="inline-flex items-center gap-2 whitespace-nowrap">
        <AgentStack
          targets={names}
          max={READER_ICONS}
          size="sm"
          role="img"
          aria-label={names.join(', ')}
          title={names.join(', ')}
        />
        {names.length > READER_ICONS && (
          <span className="text-[12.5px] text-ink-3">
            {t('targetDetail.skillsOff.readers.more', { count: names.length - READER_ICONS })}
          </span>
        )}
      </span>
    </span>
  );
  const row = (Icon: LucideIcon, content: ReactNode) => {
    return (
      <li className="flex gap-2.5">
        <Icon size={16} className="mt-[3px] shrink-0 text-ink-3" />
        <span className="min-w-0 flex-1">{content}</span>
      </li>
    );
  };
  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      preventClose={busy}
      ariaLabel={title}
      className="!max-w-[480px]"
    >
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
      </div>
      <div className="db !gap-3 text-[13.5px] leading-relaxed">
        {affected && readers && (
          <div className="ss-note warn">
            <TriangleAlert size={16} className="mt-[2px] shrink-0" />
            <span className="flex flex-1 flex-col gap-1">
              <span className="font-semibold">
                {t('targetDetail.skillsOff.readers.title', { path })}
              </span>
              {readers.off.length > 0 &&
                readerLine(t('targetDetail.skillsOff.readers.off'), readers.off)}
              {readers.local.length > 0 &&
                readerLine(t('targetDetail.skillsOff.readers.local'), readers.local)}
              {readers.on.length > 0 &&
                readerLine(t('targetDetail.skillsOff.readers.on'), readers.on, true)}
            </span>
          </div>
        )}
        {preview.isPending ? (
          <div className="flex items-center gap-2 text-[13px] text-ink-2">
            <Spinner size="sm" />
            {t('targetDetail.loadingPreview')}
          </div>
        ) : preview.error ? (
          <div className="ss-note bad">
            <span className="flex-1">{preview.error.message}</span>
          </div>
        ) : (
          <ul className="flex flex-col gap-2.5">
            {row(
              Unlink,
              preview.data.sharedWith ? (
                t('targetDetail.skillsOff.shared', { path, target: preview.data.sharedWith })
              ) : remove.length === 0 ? (
                t('targetDetail.skillsOff.removeNone', { path })
              ) : (
                <>
                  {t(plural('targetDetail.skillsOff.remove', remove.length), {
                    count: remove.length,
                    path,
                  })}{' '}
                  <Button
                    variant="link"
                    className="!inline"
                    onClick={() => setShowRemoved(!showRemoved)}
                    aria-expanded={showRemoved}
                  >
                    {t(showRemoved ? 'targetDetail.skillsOff.hide' : 'targetDetail.skillsOff.show')}
                  </Button>
                  {showRemoved && (
                    <span className="mt-1.5 flex max-h-40 flex-col overflow-y-auto font-mono text-[12px] text-ink-2">
                      {remove.map((name) => (
                        <span key={name} className="truncate">
                          {name}
                        </span>
                      ))}
                    </span>
                  )}
                </>
              ),
            )}
            {keep.length > 0 &&
              row(
                Folder,
                t(plural('targetDetail.skillsOff.keep', keep.length), {
                  count: keep.length,
                  names: names(keep),
                }),
              )}
            {copies.length > 0 &&
              row(
                Copy,
                t(plural('targetDetail.skillsOff.copies', copies.length), {
                  count: copies.length,
                  names: names(copies),
                  path,
                }),
              )}
            {managed.length > 0 &&
              row(Check, t('targetDetail.skillsOff.managed', { items: joinList(managed, locale) }))}
          </ul>
        )}
        {readFrom && (
          <div className="ss-note inf">
            <span className="flex-1">
              {t(plural('targetDetail.skillsOff.stillSees', readFrom.linkedCount), {
                name: target.name,
                from: readFrom.name,
                path: shortenHome(readFrom.path),
                count: readFrom.linkedCount,
              })}
            </span>
          </div>
        )}
        {error && (
          <div className="ss-note bad">
            <span className="flex-1">{error}</span>
          </div>
        )}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          {t('common.cancel')}
        </Button>
        <Button
          variant={affected ? 'danger' : 'primary'}
          onClick={stop}
          loading={busy}
          disabled={preview.isPending}
        >
          {t('targetDetail.skillsOff.confirm')}
        </Button>
      </div>
    </DialogShell>
  );
}
