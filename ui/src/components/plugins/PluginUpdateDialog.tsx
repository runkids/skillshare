import { useState } from 'react';
import { Check, CircleAlert, CircleArrowUp, Loader2, Package, X } from 'lucide-react';
import { targetMap } from '../../api/plugins';
import type { PluginInventory, PluginPlan, PluginTarget } from '../../api/plugins';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import { plural, useT } from '../../i18n';
import { keyedMessage } from './outcomeText';
import { Toggle, VersionChange } from './PluginList';
import { installedVersion, updatedTargets, versionMoves, type BatchItem, type UpdatePick } from './pluginsFlow';
import { UnchangedRuns } from './PluginRuns';

interface Props {
  open: boolean;
  /** What the last check found. */
  plan: PluginPlan | null;
  /** What of it is still to update, after the updates applied since. */
  pending: UpdatePick[];
  inventory: PluginInventory;
  /** The batch under way or just finished; none while picking. */
  batch: BatchItem[] | null;
  busy: boolean;
  isPi: (name: string) => boolean;
  onUpdate: (picks: UpdatePick[]) => void;
  onStop: () => void;
  onClose: () => void;
}

/** What a check found, one row per plugin: tick what to update, watch it run, retry what failed. */
export default function PluginUpdateDialog({ open, plan, pending, inventory, batch, busy, isPi, onUpdate, onStop, onClose }: Props) {
  const t = useT();
  const changes = plan?.changes ?? [];
  const pluginTargets = targetMap(inventory.targetDefinitions);
  const label = (target: string) => pluginTargets[target]?.label ?? target;
  // Only what is still behind and can be updated from here gets a tick; an update applied since the check took its Agent off.
  const found = changes.filter((c) => c.action === 'update-available' && pending.some((p) => p.name === c.name && p.targets.includes(c.target)));
  const names = pending.map((p) => p.name);
  const unverified = changes.filter((c) => c.action === 'native-check');
  // A source that no longer offers a compatible plugin for an Agent: nothing can be updated there until that changes.
  const blocked = changes.filter((c) => c.action === 'blocked');
  const others = [
    { title: t('plugins.updateDialog.blockedTitle'), rows: blocked, tag: <span className="ss-tag bad">{t('plugins.blocked')}</span> },
    { title: t('plugins.updateDialog.unverifiedTitle'), rows: unverified, tag: <span className="ss-tag warn">{t('plugins.unverified')}</span> },
  ].filter((g) => g.rows.length > 0);
  const idle = changes.filter((c) => c.action === 'noop').map((c) => ({ name: c.name, target: c.target, version: c.binding?.version ?? installedVersion(inventory, c.name, c.target) }));
  return (
    <DialogShell open={open} onClose={onClose} preventClose={busy} ariaLabel={t('plugins.updateDialog.title')} maxWidth="2xl" padding="none">
      {batch
        ? <Running batch={batch} busy={busy} inventory={inventory} label={label} onRetry={(item) => onUpdate([{ name: item.name, targets: item.failed ?? item.targets }])} onStop={onStop} onClose={onClose} />
        : (
          <Picking key={plan?.revision} names={names} attention={others.length > 0} current={others.length === 0 ? new Set(idle.map((r) => r.name)).size : 0} inventory={inventory} label={label} busy={busy} onUpdate={onUpdate} onClose={onClose}
            found={found.map((c) => ({ name: c.name, target: c.target, version: c.binding?.version, commit: c.binding?.commit, note: c.message ? keyedMessage(t, c) : '' }))}>
            {others.map((g) => (
              <div key={g.title} className="flex flex-col gap-2">
                <span className="text-xs font-medium text-ink-3">{g.title} · {g.rows.length}</span>
                <div className="ss-list">
                  {g.rows.map((c) => (
                    <div key={`${c.name}:${c.target}`} className="ss-r">
                      <span className="ss-at"><AgentIcon target={c.target} size={17} /></span>
                      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                        <span className="flex items-center gap-2"><span className="font-mono font-semibold">{c.name}</span><span className="text-[13px] text-ink-2">{label(c.target)}</span><VersionChange from={installedVersion(inventory, c.name, c.target)} /></span>
                        {c.message && <span className="text-xs text-ink-3">{keyedMessage(t, c)}</span>}
                      </span>
                      {g.tag}
                    </div>
                  ))}
                </div>
              </div>
            ))}
            {/* With nothing to update or look into, what is up to date is the whole answer, so it shows open. */}
            <UnchangedRuns runs={idle} isPi={isPi} versions expanded={names.length === 0 && others.length === 0 ? { label } : undefined} />
          </Picking>
        )}
    </DialogShell>
  );
}

type Found = { name: string; target: PluginTarget; version?: string; commit?: string; note: string };

/** The dialog's header as on the Skills update page: a tile for the state, then what is happening. */
function Hero({ icon, tone = '', title, sub, busy, onClose }: { icon: React.ReactNode; tone?: string; title: React.ReactNode; sub: string; busy: boolean; onClose: () => void }) {
  const t = useT();
  return (
    <div className="dh items-center" aria-live="polite">
      <div className="flex min-w-0 items-center gap-3.5">
        <span className={`ss-update-icon ${tone}`}>{icon}</span>
        <div className="flex min-w-0 flex-col gap-0.5"><h2 className="ss-h2">{title}</h2><p className="text-[13px] text-ink-2">{sub}</p></div>
      </div>
      <IconButton icon={<X size={16} />} label={t('common.close')} disabled={busy} onClick={onClose} />
    </div>
  );
}

function Picking({ names, attention, current, found, inventory, label, busy, onUpdate, onClose, children }: {
  names: string[]; attention: boolean;
  /** How many plugins are up to date, when that is all the check found. */
  current: number; found: Found[]; inventory: PluginInventory; label: (target: string) => string; busy: boolean;
  onUpdate: (picks: UpdatePick[]) => void; onClose: () => void; children: React.ReactNode;
}) {
  const t = useT();
  const targetsOf = (name: string) => found.filter((c) => c.name === name).map((c) => c.target);
  // Everything starts ticked; a plugin on several Agents can have one left out.
  const [picked, setPicked] = useState<Record<string, PluginTarget[]>>(() => Object.fromEntries(names.map((name) => [name, targetsOf(name)])));
  const pickedOf = (name: string) => picked[name] ?? [];
  const picks: UpdatePick[] = names.map((name) => ({ name, targets: pickedOf(name) })).filter((p) => p.targets.length > 0);
  const all = picks.length === names.length;
  const agents = picks.reduce((n, p) => n + p.targets.length, 0);
  const setAll = (on: boolean) => setPicked(Object.fromEntries(names.map((name) => [name, on ? targetsOf(name) : []])));
  const toggle = (name: string, target: PluginTarget, on: boolean) => setPicked((prev) => ({ ...prev, [name]: on ? [...(prev[name] ?? []), target] : (prev[name] ?? []).filter((x) => x !== target) }));
  return (
    <>
      {names.length > 0
        ? <Hero icon={<CircleArrowUp size={20} />} title={t(plural('plugins.updateDialog.available', names.length), { count: names.length })} sub={t('plugins.updateDialog.pickHelp')} busy={busy} onClose={onClose} />
        : attention
          ? <Hero icon={<CircleArrowUp size={20} />} title={t('plugins.updateDialog.title')} sub={t('plugins.updateDialog.noneHere')} busy={busy} onClose={onClose} />
          : <Hero icon={<Check size={20} strokeWidth={2.6} className="ss-pop" />} tone="ok" title={current > 0 ? t(plural('plugins.updateDialog.allCurrent', current), { count: current }) : t('plugins.updateDialog.none')} sub={t('plugins.updateDialog.justChecked')} busy={busy} onClose={onClose} />}
      <div className="db flex flex-col gap-4 overflow-y-auto">
        {names.length > 0 && (
          <div className="ss-list">
            <div className="ss-gh gap-3">
              <Checkbox size="sm" label={t('plugins.updateDialog.selectAll')} checked={all} indeterminate={!all && picks.length > 0} onChange={setAll} />
              <span className="text-xs text-ink-3">{t('plugins.updateDialog.selected', { count: picks.length, total: names.length })} · {t(plural('plugins.updateDialog.agents', agents), { count: agents })}</span>
            </div>
            {names.map((name) => {
              const rows = found.filter((c) => c.name === name);
              const on = pickedOf(name).length > 0;
              const { moves, shared } = versionMoves(inventory, name, rows);
              const note = rows.find((c) => c.note)?.note;
              return (
                <div key={name} className="ss-r items-start py-3">
                  <Checkbox size="sm" hideLabel label={name} checked={on} onChange={(checked) => setPicked((prev) => ({ ...prev, [name]: checked ? targetsOf(name) : [] }))} className="mt-2" />
                  <span className="ss-at mt-0.5">{rows.length === 1 ? <AgentIcon target={rows[0].target} size={17} /> : <Package size={15} />}</span>
                  <span className="flex min-w-0 flex-1 flex-col gap-1.5">
                    <span className="flex items-center gap-2">
                      <span className="font-mono font-semibold">{name}</span>
                      {rows.length === 1 && <span className="text-[13px] text-ink-2">{label(rows[0].target)}</span>}
                      {shared && <VersionChange {...moves[0]} />}
                      {rows.length > 1 && <span className="text-xs text-ink-3">{t(plural('plugins.updateDialog.agents', rows.length), { count: rows.length })}</span>}
                    </span>
                    {rows.length > 1 && (
                      <span className="flex flex-wrap items-center gap-1.5">
                        {rows.map((c) => <Toggle key={c.target} target={c.target} label={label(c.target)} on={pickedOf(name).includes(c.target)} applying={false} disabled={busy} onClick={() => toggle(name, c.target, !pickedOf(name).includes(c.target))} />)}
                      </span>
                    )}
                    {!shared && moves.map((m) => <span key={m.target} className="flex items-center gap-2 text-xs text-ink-3">{label(m.target)}<VersionChange {...m} /></span>)}
                    {note && <span className="text-xs text-ink-3">{note}</span>}
                  </span>
                </div>
              );
            })}
          </div>
        )}
        {children}
      </div>
      <div className="df">
        {names.length > 0
          ? <><Button variant="ghost" disabled={busy} onClick={onClose}>{t('common.cancel')}</Button><Button disabled={busy || picks.length === 0} onClick={() => onUpdate(picks)}>{t(plural('plugins.updateDialog.updateSelected', picks.length), { count: picks.length })}</Button></>
          : <Button onClick={onClose}>{t('plugins.updateDialog.finish')}</Button>}
      </div>
    </>
  );
}

function Running({ batch, busy, inventory, label, onRetry, onStop, onClose }: {
  batch: BatchItem[]; busy: boolean; inventory: PluginInventory; label: (target: string) => string;
  onRetry: (item: BatchItem) => void; onStop: () => void; onClose: () => void;
}) {
  const t = useT();
  // A plugin counts as updated when any of its Agents was; one where another Agent was skipped says so on its row.
  const done = batch.filter((i) => i.status !== 'failed' && updatedTargets(i).length > 0).length;
  const failed = batch.filter((i) => i.status === 'failed').length;
  const settled = batch.filter((i) => i.status !== 'waiting' && i.status !== 'running').length;
  const current = batch.find((i) => i.status === 'running');
  const status = (item: BatchItem) => {
    if (item.status === 'running') return <span className="inline-flex items-center gap-2 text-[13px] font-semibold text-ink"><Loader2 size={13} className="animate-spin" />{t('plugins.updateDialog.updating')}</span>;
    if (item.status === 'waiting') return <span className="text-[13px] text-ink-3">{t('plugins.updateDialog.waiting')}</span>;
    if (item.status === 'done') return <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-ok"><Check size={15} strokeWidth={2.6} className="ss-pop" />{t('plugins.updateDialog.done')}</span>;
    if (item.status === 'skipped') return <span className="ss-st warn">{t('plugins.outcome.skipped')}</span>;
    return <span className="ss-st bad">{t('plugins.outcome.failed')}</span>;
  };
  return (
    <>
      {busy
        ? <Hero icon={<Loader2 size={20} className="animate-spin" />} busy={busy} onClose={onClose} sub={t('plugins.updateDialog.runningHelp')}
            title={<>{t('plugins.updateDialog.running', { done: Math.min(settled + 1, batch.length), total: batch.length })}{current && <span className="ml-3 font-mono text-[14px] font-medium text-ink-2">{current.name}</span>}</>} />
        : failed > 0
          ? <Hero icon={<CircleAlert size={20} />} tone="warn" busy={busy} onClose={onClose} title={t('plugins.updateDialog.doneFailed', { updated: done, failed })} sub={t('plugins.updateDialog.doneHelp')} />
          : <Hero icon={<Check size={20} strokeWidth={2.6} className="ss-pop" />} tone="ok" busy={busy} onClose={onClose} title={t(plural('plugins.updateDialog.updated', done), { count: done })} sub={t('plugins.updateDialog.doneHelp')} />}
      <div className="db flex flex-col gap-4 overflow-y-auto">
        <div className="ss-bar" role="progressbar" aria-valuemin={0} aria-valuemax={batch.length} aria-valuenow={settled} aria-label={t('plugins.updateDialog.title')}>
          <span style={{ width: `${(settled / batch.length) * 100}%` }} />
        </div>
        <div className="ss-list">
          {batch.map((item) => {
            // A row no check showed keeps the version it has.
            const { moves, shared } = item.versions ?? { moves: [{ target: item.targets[0], from: installedVersion(inventory, item.name, item.targets[0]) }], shared: true };
            return (
              <div key={item.name} className={`ss-r ${{ failed: 'bg-bad-bg', running: 'updating', waiting: 'dim', done: 'flash', skipped: '' }[item.status]}`}>
                <span className="ss-at">{item.targets.length === 1 ? <AgentIcon target={item.targets[0]} size={17} /> : <Package size={15} />}</span>
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="flex items-center gap-2">
                    <span className="nm font-mono">{item.name}</span>
                    <span className="text-[13px] text-ink-2">{item.targets.map(label).join(' · ')}</span>
                    {shared && <VersionChange {...moves[0]} />}
                  </span>
                  {!shared && moves.map((m) => <span key={m.target} className="flex items-center gap-2 text-xs text-ink-3">{label(m.target)}<VersionChange {...m} /></span>)}
                  {item.failed && updatedTargets(item).length > 0 && <span className="text-xs text-ink-3">{t(item.status === 'failed' ? 'plugins.updateDialog.partFailed' : 'plugins.updateDialog.partSkipped', { done: updatedTargets(item).map(label).join(' · '), failed: item.failed.map(label).join(' · ') })}</span>}
                  {item.note && <span className={item.status === 'failed' ? 'font-mono text-xs text-bad' : 'text-xs text-ink-3'}>{item.note}</span>}
                </span>
                {status(item)}
                {(item.status === 'failed' || item.status === 'skipped') && !item.stale && !busy && <Button size="sm" variant="secondary" onClick={() => onRetry(item)}>{t('plugins.updateDialog.retry')}</Button>}
                {item.status === 'running' && <span className="ss-rowbar" aria-hidden />}
              </div>
            );
          })}
        </div>
        {!busy && <p className="text-xs text-ink-3">{t('plugins.nativeHelp')}</p>}
      </div>
      <div className="df">
        {/* The server clears an earlier stop as the batch starts, so Stop waits until a plugin is under way. */}
        {busy
          ? <><span className="mr-auto text-xs text-ink-3">{t('plugins.updateDialog.stopHelp')}</span><Button variant="secondary" onClick={onStop} disabled={!current}>{t('plugins.updateDialog.stop')}</Button></>
          : <Button onClick={onClose}>{t('plugins.updateDialog.finish')}</Button>}
      </div>
    </>
  );
}
