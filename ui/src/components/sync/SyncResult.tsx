import { Link } from 'react-router-dom';
import { AlertCircle, CircleCheck, TriangleAlert } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { useT, plural } from '../../i18n';
import { failureExplanation, type SyncFailure } from './syncView';
import SyncError from './SyncError';
import { profileReason } from '../profiles/profileView';

const PART_TAG: Record<SyncFailure['part'], string> = { skill: 'Skills', agent: 'Agents', extra: 'Extras', config: 'Config' };

interface Props {
  failures: SyncFailure[];
  /** Warnings not already shown as a failure */
  warnings: string[];
  /** Targets that synced */
  synced: number;
  force?: boolean;
  /** Turns on Force; left out where there is no Force switch, so a symlink conflict offers Open target */
  onForce?: () => void;
}

/** The outcome of a sync that had failures or warnings: failed targets first, then warnings, then what synced. */
export default function SyncResult({ failures, warnings, synced, force, onForce }: Props) {
  const t = useT();
  if (failures.length === 0 && warnings.length === 0) return null;
  const failedTargets = new Set(failures.map((f) => f.target)).size;
  return (
    <section className="ss-list" aria-labelledby={failures.length > 0 ? 'sync-result-title' : undefined}>
      {failures.length > 0 && (
        <>
          <div className="flex items-center gap-2.5 bg-bad-bg px-4 py-3">
            <AlertCircle size={18} className="text-bad" />
            <h2 id="sync-result-title" className="flex-1 text-[15px] font-semibold">{t(plural('sync.result.failed', failedTargets), { count: failedTargets })}</h2>
          </div>
          {failures.map((f) => (
            <div key={`${f.part}/${f.extra ?? ''}/${f.target}`} className="ss-r">
              <span className="ss-at"><AgentIcon target={f.target} size={17} /></span>
              <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                <div className="flex items-center gap-2">
                  <span className="font-semibold">{f.target}</span>
                  <span className="ss-tag">{f.extra ? `${PART_TAG[f.part]} · ${f.extra}` : PART_TAG[f.part]}</span>
                </div>
                <Explanation failure={f} />
                <SyncError error={f.error} />
              </div>
              {f.conflict && !force && onForce ? (
                <Button size="sm" variant="secondary" onClick={onForce} title={t('sync.forceHint')}>{t('sync.result.useForce')}</Button>
              ) : (
                <Link to={`/targets/${encodeURIComponent(f.target)}`} className="ss-btn sm shrink-0">{t('sync.result.openTarget')}</Link>
              )}
            </div>
          ))}
        </>
      )}
      {warnings.map((w) => (
        <div key={w} className="ss-r !min-h-0 bg-warn-bg text-[13px]">
          <TriangleAlert size={16} className="text-warn" />
          <span className="flex-1 break-words">{profileReason(w, t)}</span>
        </div>
      ))}
      {failures.length > 0 && synced > 0 && (
        <div className="ss-r !min-h-0 text-[13px]">
          <CircleCheck size={16} className="text-ok" />
          <span className="flex-1">{t(plural('sync.result.synced', synced), { count: synced })}</span>
        </div>
      )}
    </section>
  );
}

/** What went wrong, in plain words, when the error is a known case. */
function Explanation({ failure }: { failure: SyncFailure }) {
  const t = useT();
  const key = failureExplanation(failure);
  return key ? <span className="text-[13px]">{t(key)}</span> : null;
}
