import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, ApiError, type ActiveProfileRequest, type ProfileApplyResult, type ProfileEntryChange, type ProfilesResponse } from '../../api/client';
import { useAppContext } from '../../context/AppContext';
import { useOverviewQuery, useSkillsQuery } from '../../hooks/useSharedQueries';
import { useT } from '../../i18n';
import { invalidate } from '../../lib/queryEvents';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import CopyButton from '../CopyButton';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import { blockedEntries, profileError, profileGroups, profileReason } from './profileView';

interface Props { candidate: string | null; profiles: ProfilesResponse; onClose: () => void }

// Each candidate owns its own consent and asynchronous preview lifetime.
export default function ProfileSwitchDialog(props: Props) {
  return <SwitchAttempt key={props.candidate ?? '__all'} {...props} />;
}

function SwitchAttempt({ candidate, profiles, onClose }: Props) {
  const t = useT();
  const client = useQueryClient();
  const { isProjectMode } = useAppContext();
  const overview = useOverviewQuery();
  const skills = useSkillsQuery();
  const [preview, setPreview] = useState<ProfileApplyResult | null>(null);
  const [result, setResult] = useState<ProfileApplyResult | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [blocked, setBlocked] = useState<ProfileEntryChange[]>([]);
  const [needsEmpty, setNeedsEmpty] = useState(false);
  const [allowEmpty, setAllowEmpty] = useState(false);
  const [discardEdits, setDiscardEdits] = useState(false);
  const [loading, setLoading] = useState(true);
  const [applying, setApplying] = useState(false);
  const [revision, setRevision] = useState(0);
  const body: ActiveProfileRequest = { name: candidate, ...(discardEdits && { discardEdits: true }), ...(allowEmpty && { allowEmpty: true }) };

  useEffect(() => {
    let current = true;
    setLoading(true);
    setError(null);
    setPreview(null);
    api.profiles.setActive({ name: candidate, dryRun: true, ...(discardEdits && { discardEdits: true }), ...(allowEmpty && { allowEmpty: true }) }).then((plan) => {
      if (current) { setPreview(plan); setBlocked(plan.blocked ?? []); }
    }).catch((err: unknown) => {
      if (!current) return;
      setError(err);
      setBlocked(blockedEntries(err));
      if (err instanceof ApiError && err.code === 'profile_empty') setNeedsEmpty(true);
    }).finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [candidate, discardEdits, allowEmpty, revision]);

  const apply = async () => {
    setApplying(true);
    setError(null);
    try {
      const applied = await api.profiles.setActive(body);
      setResult(applied);
      void invalidate(client, 'profileSwitched');
    } catch (err: unknown) {
      setPreview(null);
      setError(err);
      setBlocked(blockedEntries(err));
      setDiscardEdits(false);
      if (err instanceof ApiError && err.code === 'profile_empty') { setNeedsEmpty(true); setAllowEmpty(false); }
    } finally { setApplying(false); }
  };

  const chosen = profiles.profiles.find((p) => p.name === candidate);
  const total = skills.data?.resources.filter((s) => s.kind !== 'agent' && !s.disabled).length;
  const shown = result ?? preview;
  const groups = profileGroups(shown?.targets ?? null);
  const discardable = blocked.length > 0 && blocked.every((e) => e.class === 'modified_copy');
  const title = t('profiles.title', { name: candidate ?? t('profiles.all') });
  const canApply = !!preview && !loading && !applying && !result && (!needsEmpty || allowEmpty) && (blocked.length === 0 || discardable && discardEdits);
  return <DialogShell open onClose={onClose} maxWidth="lg" padding="none" preventClose={applying} ariaLabel={title}>
    <div className="dh"><div className="flex flex-col gap-1"><h2 className="ss-h2">{title}</h2><span className="text-sm text-ink-3">{t('profiles.selected', { selected: chosen?.selected ?? total ?? '…', total: total ?? '…' })}</span></div></div>
    <div className="db flex flex-col gap-4">
      {loading && <Spinner />}
      {error != null && <div className="ss-note bad">{profileError(error, t)}</div>}
      {result && <div className={`ss-note ${result.complete ? 'inf' : 'warn'}`}>{t(result.complete ? 'profiles.complete' : 'profiles.incomplete')}</div>}
      {groups.length > 0 && <div className="ss-list">{groups.map((group, index) => {
        const target = shown!.targets![index];
        return <div key={group.key}>
          <div className="ss-r flex-wrap"><AgentIcon target={group.name} /><span className="nm m flex-1">{group.name}</span>
            <span className="ss-cnt">{t('profiles.counts', { added: target.added?.length ?? 0, removed: target.removed?.length ?? 0, refreshed: target.refreshed?.length ?? 0 })}</span>
            {result && <span className={`ss-st ${target.state === 'failed' ? 'bad' : target.state === 'incomplete' ? 'warn' : 'ok'}`}>{target.state}</span>}
          </div>
          {group.rows.map((row) => <div className="ss-r text-sm text-ink-2" key={row.key}><span className="font-mono">{row.icon === 'add' ? '+' : row.icon === 'remove' ? '−' : row.icon === 'kept' ? '=' : '↻'} {row.name}</span>{row.detail && <span>{profileReason(row.detail, t)}</span>}</div>)}
          {target.error && <div className="ss-note bad">{profileReason(target.error, t)}</div>}
          {target.prune_error && <div className="ss-note bad">{profileReason(target.prune_error, t)}</div>}
          {target.failed_removals?.map((name) => <div className="ss-r text-bad font-mono" key={name}>− {name}</div>)}
        </div>;
      })}</div>}
      {!!shown?.symlink_targets?.length && <div className="ss-note inf">{t('profiles.symlink', { targets: shown.symlink_targets.join(', ') })}</div>}
      {blocked.length > 0 && !result && <div className="ss-note warn flex-col items-start">
        {blocked.map((entry, i) => <div key={`${entry.target}/${entry.entry}/${i}`}><code>{entry.target}/{entry.entry}</code> · {profileReason(entry.reason, t)}</div>)}
        {discardable && <Checkbox checked={discardEdits} onChange={setDiscardEdits} disabled={loading || applying || isProjectMode && !overview.data?.configDir} label={t(isProjectMode ? 'profiles.discardProject' : 'profiles.discard', { folder: overview.data?.configDir ? `${overview.data.configDir}/backups` : '…' })} />}
      </div>}
      {needsEmpty && !result && <div className="ss-note warn flex-col items-start">
        <span>{t('profiles.empty')}</span>
        <Checkbox checked={allowEmpty} onChange={setAllowEmpty} disabled={loading || applying} label={t('profiles.allowEmpty')} />
      </div>}
      {!!chosen?.unmatched.length && !result && <div className="ss-note warn">{t('profiles.unmatched', { patterns: chosen.unmatched.join(', ') })}</div>}
      {result?.snapshot?.map((snapshot) => <div className="ss-box p-3 flex flex-col gap-2" key={snapshot.path}>
        <span>{t('profiles.snapshot')} · {snapshot.target}</span>
        <div className="flex items-center gap-2 min-w-0"><code className="break-all">{snapshot.path}</code><CopyButton value={snapshot.path} title={t('common.copyPath')} errorMessage={t('memory.copyFailed')} copiedLabel="" /></div>
        <div className="flex items-center gap-2 min-w-0"><code className="break-all">{snapshot.restore}</code><CopyButton value={snapshot.restore} title={t('common.copy')} errorMessage={t('memory.copyFailed')} copiedLabel="" /></div>
        <span className="text-sm text-ink-3">{snapshot.retention === 'Existing backup retention applies; restore before cleanup removes this snapshot.' ? t('profiles.retention') : snapshot.retention}</span>
      </div>)}
    </div>
    <div className="df">
      <Button variant="ghost" disabled={applying} onClick={onClose}>{t(result ? 'common.close' : 'common.cancel')}</Button>
      {!result && <>{error != null && <Button variant="ghost" disabled={loading || applying} onClick={() => setRevision((v) => v + 1)}>{t('syncPreview.refreshPreview')}</Button>}<Button disabled={!canApply} loading={applying} onClick={apply}>{t('profiles.switch')}</Button></>}
    </div>
  </DialogShell>;
}
