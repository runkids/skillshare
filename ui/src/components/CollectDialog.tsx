import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDownToLine, Bot, CircleCheck, CircleMinus, CircleX, Puzzle, RefreshCw, X } from 'lucide-react';
import { api, type LocalSkillInfo } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { formatSize } from '../lib/format';
import { formatAgentDisplayName } from '../lib/resourceNames';
import { useT } from '../i18n';
import AgentIcon from './AgentIcon';
import Button from './Button';
import DialogShell from './DialogShell';
import { Checkbox } from './Input';
import SegmentedControl from './SegmentedControl';
import Spinner from './Spinner';

type Kind = 'skill' | 'agent';
type Outcome = { item: LocalSkillInfo; outcome: 'pulled' | 'skipped' | 'failed'; error?: string };

const keyOf = (i: LocalSkillInfo) => `${i.targetName}/${i.kind ?? 'skill'}/${i.name}`;
const displayName = (i: LocalSkillInfo) => (i.kind === 'agent' ? formatAgentDisplayName(i.name) : i.name);
const KindMark = ({ kind }: { kind?: string }) => (
  <span className={`ss-cat sm ${kind === 'agent' ? 'agent' : 'skill'}`}>{kind === 'agent' ? <Bot size={14} /> : <Puzzle size={14} />}</span>
);

/** Moves skills and agents created inside targets into the source. Without `target` it scans every target. */
export default function CollectDialog({ target, kind, onClose }: { target?: string; kind?: Kind; onClose: () => void }) {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [scope, setScope] = useState<Kind | 'both'>(kind ?? 'both');
  const [picked, setPicked] = useState<Set<string> | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [results, setResults] = useState<Outcome[] | null>(null);

  const scan = useQuery({
    queryKey: [...queryKeys.collectScan(target), scope],
    queryFn: () => api.collectScan(target, scope === 'both' ? undefined : scope),
    staleTime: 0,
  });
  const source = useQuery({ queryKey: queryKeys.skills.all, queryFn: () => api.listSkills(), staleTime: staleTimes.skills });
  const taken = useMemo(() => new Set((source.data?.resources ?? []).map((r) => `${r.kind}/${r.relPath}`)), [source.data]);
  const conflict = (i: LocalSkillInfo) => taken.has(`${i.kind ?? 'skill'}/${i.name}`);

  const groups = (scan.data?.targets ?? []).filter((g) => g.skills.length > 0);
  const items = groups.flatMap((g) => g.skills);
  // Nothing picked yet: everything except names the source already has.
  const chosen = picked ?? new Set(items.filter((i) => !conflict(i)).map(keyOf));
  const chosenItems = items.filter((i) => chosen.has(keyOf(i)));
  const toggle = (key: string) => {
    const next = new Set(chosen);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    setPicked(next);
  };

  const noun = kind ?? 'item';
  const run = async () => {
    setBusy(true);
    setError('');
    const outcomes: Outcome[] = [];
    try {
      // A name the source already has is only collected with force, which replaces the source copy.
      for (const [group, force] of [[chosenItems.filter((i) => !conflict(i)), false], [chosenItems.filter(conflict), true]] as const) {
        if (!group.length) continue;
        const res = await api.collect({ skills: group.map((i) => ({ name: i.name, targetName: i.targetName, kind: i.kind })), force });
        for (const item of group) {
          const failure = res.failed?.[item.name];
          outcomes.push(failure ? { item, outcome: 'failed', error: failure } : { item, outcome: res.pulled?.includes(item.name) ? 'pulled' : 'skipped' });
        }
      }
      setResults(outcomes);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
      for (const key of [queryKeys.skills.all, queryKeys.overview, queryKeys.targets.all, queryKeys.diff(), queryKeys.syncMatrix(), queryKeys.collectScan(target)]) {
        void queryClient.invalidateQueries({ queryKey: key });
      }
    }
  };

  if (results) {
    const count = (o: Outcome['outcome']) => results.filter((r) => r.outcome === o).length;
    const pulled = count('pulled');
    return (
      <DialogShell open onClose={onClose} padding="none" ariaLabel={t('collectDialog.results')} className="!max-w-[560px]">
        <div className="dh">
          <div className="flex flex-col gap-1">
            <h2 className="ss-h2">{t('collectDialog.results')}</h2>
            <p className="text-[13px] text-ink-2">{t('collectDialog.resultsSummary', { pulled, skipped: count('skipped'), failed: count('failed') })}</p>
          </div>
          <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose}><X size={16} /></button>
        </div>
        <div className="db">
          <div className="ss-list !shadow-none">
            {results.map(({ item, outcome, error: why }) => (
              <div key={keyOf(item)} className="ss-r !min-h-11">
                {outcome === 'pulled' ? <CircleCheck size={16} className="shrink-0 text-ok" /> : outcome === 'skipped' ? <CircleMinus size={16} className="shrink-0 text-ink-3" /> : <CircleX size={16} className="shrink-0 text-bad" />}
                <KindMark kind={item.kind} />
                <span className="min-w-0 flex-1 truncate font-mono text-[13px] font-semibold">{displayName(item)}</span>
                <span className={`min-w-0 truncate text-[13px] ${outcome === 'failed' ? 'text-bad' : 'text-ink-2'}`} title={why}>
                  {outcome === 'pulled' ? t('collectDialog.pulled') : outcome === 'skipped' ? t('collectDialog.skipped') : t('collectDialog.failed', { error: why ?? '' })}
                </span>
              </div>
            ))}
          </div>
          {pulled > 0 && <p className="text-[13px] text-ink-2">{t('collectDialog.pulledHint')}</p>}
        </div>
        <div className="df">
          <Button variant="ghost" onClick={onClose}>{t('common.close')}</Button>
          {pulled > 0 && <Button variant="primary" onClick={() => { onClose(); navigate('/sync'); }}><RefreshCw size={15} />{t('collectDialog.goToSync')}</Button>}
        </div>
      </DialogShell>
    );
  }

  const title = target ? t('collectDialog.titleFrom', { name: target }) : t('collectDialog.titleAll');
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[620px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t(`collectDialog.subtitle.${kind ?? 'all'}`)}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <div className="db">
        {!kind && (
          <div className="flex items-center gap-2.5">
            <span className="text-[13px] text-ink-2">{t('collectDialog.lookFor')}</span>
            <SegmentedControl
              value={scope}
              onChange={(v) => { setScope(v); setPicked(null); }}
              options={[{ value: 'skill', label: 'Skills' }, { value: 'agent', label: 'Agents' }, { value: 'both', label: t('collectDialog.both') }]}
            />
            <span className="flex-1" />
            <Button variant="secondary" size="sm" onClick={() => void scan.refetch()} loading={scan.isFetching} disabled={busy}>
              {!scan.isFetching && <RefreshCw size={14} />}
              {t('collectDialog.scanAgain')}
            </Button>
          </div>
        )}
        {scan.isPending ? (
          <div className="flex items-center gap-2 text-[13px] text-ink-2"><Spinner size="sm" />{t('collectDialog.scanning')}</div>
        ) : scan.error ? (
          <div className="ss-note bad"><span className="flex-1">{scan.error.message}</span></div>
        ) : items.length === 0 ? (
          <div className="ss-note inf"><span className="flex-1">{t(target ? 'collectDialog.emptyTarget' : 'collectDialog.emptyAll')}</span></div>
        ) : (
          <div className="ss-list max-h-[360px] overflow-y-auto !shadow-none">
            <div className="ss-lh !px-4">
              <Checkbox
                label={t(items.length === 1 ? 'collectDialog.found.one' : 'collectDialog.found.other', { count: items.length })}
                checked={chosenItems.length === items.length}
                indeterminate={chosenItems.length > 0 && chosenItems.length < items.length}
                onChange={(on) => setPicked(new Set(on ? items.map(keyOf) : []))}
                disabled={busy}
              />
              <span className="flex-1" />
              <span>{t('collectDialog.selected', { selected: chosenItems.length, total: items.length })}</span>
            </div>
            {groups.map((g) => (
              <div key={g.targetName} className="contents">
                {!target && (
                  <div className="ss-gh">
                    <AgentIcon target={g.targetName} size={18} />
                    <span className="font-semibold">{g.targetName}</span>
                    <span className="text-ink-3">{t('collectDialog.groupCount', { count: g.skills.length })}</span>
                  </div>
                )}
                {g.skills.map((item) => {
                  const key = keyOf(item);
                  const on = chosen.has(key);
                  const clash = conflict(item);
                  return (
                    <div key={key} className={`ss-r !min-h-11 ${on ? 'sel' : ''}`}>
                      <Checkbox label={displayName(item)} hideLabel checked={on} onChange={() => toggle(key)} disabled={busy} />
                      <KindMark kind={item.kind} />
                      <span className="w-[180px] shrink-0 truncate font-mono text-[13px] font-semibold" title={displayName(item)}>{displayName(item)}</span>
                      <span className={`min-w-0 flex-1 truncate text-[13px] ${clash && on ? 'text-warn' : 'text-ink-2'}`}>
                        {clash ? t(on ? 'collectDialog.replaces' : 'collectDialog.exists') : formatSize(item.size)}
                      </span>
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
        )}
        {error && <div className="ss-note bad"><span className="flex-1">{error}</span></div>}
        {items.length > 0 && <p className="text-[13px] leading-relaxed text-ink-2">{t('collectDialog.hint')}</p>}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
        <Button variant="primary" onClick={run} loading={busy} disabled={chosenItems.length === 0}>
          {!busy && <ArrowDownToLine size={15} />}
          {t(`collectDialog.run.${noun}.${chosenItems.length === 1 ? 'one' : 'other'}`, { count: chosenItems.length })}
        </Button>
      </div>
    </DialogShell>
  );
}
