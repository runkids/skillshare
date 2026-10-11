import { useRef, useState } from 'react';
import { bindingVersion, byPlugin, pluginsApi, syncAction, targetMap, type PluginInventory, type PluginOutcome, type PluginPlan, type PluginRequest, type PluginResult, type PluginTarget } from '../../api/plugins';
import type { useT } from '../../i18n';
import { keyedMessage } from './outcomeText';
import type { PluginUpdate } from './PluginList';

type T = ReturnType<typeof useT>;
type Updates = Record<string, Record<PluginTarget, PluginUpdate>>;
/** One plugin and the Agents to update it on. */
export interface UpdatePick { name: string; targets: PluginTarget[] }
/** One plugin in a batch update. */
export interface BatchItem extends UpdatePick {
  status: 'waiting' | 'running' | 'done' | 'failed' | 'skipped';
  note?: string;
  /** The Agents the last try failed on; a retry sends only these. */
  failed?: PluginTarget[];
  /** The source moved past what the check showed: nothing was applied, and only a new check can show what it would bring. */
  stale?: boolean;
}

/** What is still to update after the last check: the Agents it found behind that are still bound and can be updated from here, per plugin. */
export function updatesPending(updates: Updates, inventory?: PluginInventory): UpdatePick[] {
  const targets = targetMap(inventory?.targetDefinitions);
  return Object.entries(updates)
    .map(([name, u]) => ({ name, targets: Object.keys(u).filter((target) => inventory?.packages[name]?.bindings[target] && targets[target]?.operations.includes('update')) }))
    .filter((p) => p.targets.length > 0);
}

/** The server joins each failed target's English text; show the translated messages once each instead. */
export function failureText(t: T, response: PluginResult) {
  const failed = new Set((response.result?.results ?? []).filter((r) => r.status === 'failed').map((r) => keyedMessage(t, r)));
  return response.failure && failed.size ? [...failed].join(' ') : response.failure;
}

/** What a check found to update, per plugin and Agent (version '' when the source changed without a new one). */
export function updatesFound(plan: PluginPlan) {
  const found: Updates = {};
  for (const c of plan.changes) if (c.action === 'update-available') (found[c.name] ??= {})[c.target] = { version: c.binding?.version ?? '', commit: c.binding?.commit };
  return found;
}

/**
 * An Agent comes off once the result reports it updated, or already there (noop); one that
 * failed, was skipped, or never ran because the apply stopped first keeps its Update action.
 */
export function updatesLeft(updates: Updates, plan: PluginPlan, response: PluginResult) {
  const done = new Set((response.result?.results ?? []).filter((r) => r.status !== 'failed' && r.status !== 'skipped').map((r) => `${r.name}:${r.target}`));
  const next = { ...updates };
  for (const c of plan.changes) {
    if ((c.action !== 'update' && c.action !== 'noop') || !done.has(`${c.name}:${c.target}`) || !next[c.name]) continue;
    next[c.name] = { ...next[c.name] };
    delete next[c.name][c.target];
  }
  return next;
}

/** What Sync would do: every binding whose tick differs from what its Agent has. */
export function syncTodo(inventory?: PluginInventory) {
  return Object.entries(inventory?.packages ?? {})
    .flatMap(([name, pack]) => Object.entries(pack.bindings).map(([target, b]) => ({ name, target, word: syncAction(b!, inventory?.hosts.find((h) => h.target === target)) })))
    .filter((x) => x.word);
}

/**
 * A package Pi installs itself (no source Skillshare copies from) and bound only to Pi targets
 * is a Pi package: it gets its own list, in the same rows. A plugin Skillshare installs stays above.
 */
export function piPackageTest(inventory: PluginInventory | undefined) {
  const piTargets = new Set((inventory?.targetDefinitions ?? []).filter((d) => d.npm).map((d) => d.target));
  return (name: string) => {
    // A run can name a package config no longer has, such as one it just removed.
    const pack = inventory?.packages[name];
    if (!pack) return false;
    const bound = Object.entries(pack.bindings);
    return !pack.source && bound.length > 0 && bound.every(([target, b]) => piTargets.has(target) && !b?.source);
  };
}

export function installedVersion(inventory: PluginInventory | undefined, name: string, target: string) {
  const b = inventory?.packages[name]?.bindings[target];
  return b && inventory ? bindingVersion(inventory, target, b) : undefined;
}

/** A preview lists every binding; the ones it leaves alone (`idle`) fold away under the ones it changes (`active`). */
export function reviewChanges(plan: PluginPlan | undefined, inventory?: PluginInventory) {
  const changes = plan?.changes ?? [];
  return {
    active: changes.filter((c) => c.action !== 'noop'),
    idle: changes.filter((c) => c.action === 'noop').map((c) => ({ name: c.name, target: c.target, version: c.binding?.version ?? installedVersion(inventory, c.name, c.target) })),
  };
}

/** What the last run changed, failures first; Agents that ended the same way share a row. */
export function changedRuns(t: T, outcomes: PluginOutcome[]) {
  return byPlugin(outcomes.filter((r) => r.status !== 'unchanged').map((r) => ({ ...r, note: r.message ? keyedMessage(t, r) : '' })), (r) => `${r.status}\0${r.note}`)
    .sort((a, b) => Number(b.status === 'failed') - Number(a.status === 'failed'));
}

/**
 * The Plugins page's preview → review → apply flow and what it derives from the inventory.
 * A change is written only by `apply`, and only with the revision its preview returned.
 */
export function usePluginsFlow({ t, inventory, refresh, onPreviewed }: {
  t: T;
  inventory?: PluginInventory;
  /** Refetch what a write changed. */
  refresh: () => void;
  /** A preview is ready for review: close whatever asked for it. */
  onPreviewed: () => void;
}) {
  const [review, setReview] = useState<{ request: PluginRequest; plan: PluginPlan } | null>(null);
  const [busy, setBusy] = useState(false);
  const [working, setWorking] = useState('');
  const [failure, setFailure] = useState('');
  const [result, setResult] = useState<PluginResult | null>(null);
  // What the last check found to update. An applied update takes its Agents off.
  const [updates, setUpdates] = useState<Updates>({});
  const [checked, setChecked] = useState<PluginPlan | null>(null);
  const [checking, setChecking] = useState(false);
  // The batch update under way or just finished, one row per plugin, until the dialog closes.
  const [batch, setBatch] = useState<BatchItem[] | null>(null);
  const stopped = useRef(false);

  /** A check is read-only and opens nothing: it only marks the rows. Quiet, as on page load, it also keeps its error to itself. */
  const check = async (quiet = false) => {
    setChecking(true); if (!quiet) setFailure('');
    try { const plan = await pluginsApi.preview({ action: 'check' }); setUpdates(updatesFound(plan)); setChecked(plan); }
    catch (e) { if (!quiet) setFailure((e as Error).message); }
    finally { setChecking(false); }
  };
  /** Each plugin picked is its own preview and apply, one after another, so one failure stops nothing else. */
  const updateMany = async (picks: UpdatePick[]) => {
    stopped.current = false;
    setBusy(true); setFailure(''); setResult(null);
    // A retry puts its row back to waiting where it is, keeping every Agent it showed; a first run starts the batch.
    const names = new Set(picks.map((p) => p.name));
    setBatch((prev) => [...(prev ?? []).map((i) => (names.has(i.name) ? { ...i, status: 'waiting' as const, note: undefined } : i)), ...picks.filter((p) => !prev?.some((i) => i.name === p.name)).map((p) => ({ ...p, status: 'waiting' as const }))]);
    const set = (name: string, patch: Partial<BatchItem>) => setBatch((prev) => prev && prev.map((i) => (i.name === name ? { ...i, ...patch } : i)));
    let staleSeen = false;
    for (const p of picks) {
      if (stopped.current) { set(p.name, { status: 'skipped' }); continue; }
      set(p.name, { status: 'running' });
      try {
        const request: PluginRequest = { action: 'update', name: p.name, targets: p.targets };
        const plan = await pluginsApi.preview(request);
        if (plan.blocked) throw new Error(plan.changes.filter((c) => c.action === 'blocked').map((c) => keyedMessage(t, c)).join(' ') || t('plugins.blocked'));
        // The user approved the versions the check showed; a source that moved since gets a new check, not a silent install.
        const moved = plan.changes.some((c) => {
          const seen = updates[c.name]?.[c.target];
          return c.action === 'update' && seen && ((c.binding?.version ?? '') !== seen.version || (!!seen.commit && c.binding?.commit !== seen.commit));
        });
        if (moved) { staleSeen = true; set(p.name, { status: 'skipped', stale: true, note: t('plugins.updateDialog.stale') }); continue; }
        // What the Agent is about to do (an npm install, say) shows while it runs.
        set(p.name, { note: plan.changes.filter((c) => c.message).map((c) => keyedMessage(t, c)).join(' ') || undefined });
        const response = await pluginsApi.apply(request, plan.revision);
        setUpdates((prev) => updatesLeft(prev, plan, response));
        const results = response.result?.results ?? [];
        const failed = failureText(t, response);
        const failedTargets = results.filter((r) => r.status === 'failed').map((r) => r.target);
        const skipped = results.filter((r) => r.status === 'skipped');
        if (failed) set(p.name, { status: 'failed', note: failed, failed: failedTargets.length ? failedTargets : p.targets });
        else if (skipped.length) set(p.name, { status: 'skipped', note: skipped.filter((r) => r.message).map((r) => keyedMessage(t, r)).join(' ') || undefined, failed: skipped.map((r) => r.target) });
        else set(p.name, { status: 'done', note: undefined, failed: undefined });
      } catch (e) { set(p.name, { status: 'failed', note: (e as Error).message, failed: p.targets }); }
    }
    setBusy(false); refresh();
    // A source that moved needs a fresh check before it can be offered again.
    if (staleSeen) void check(true);
  };

  // `key` names the control that started this, so only it shows a spinner.
  const preview = async (request: PluginRequest, key = '') => {
    setBusy(true); setWorking(key); setFailure(''); setResult(null);
    try {
      const plan = await pluginsApi.preview(request);
      setReview({ request, plan }); onPreviewed();
    }
    catch (e) { setFailure((e as Error).message); throw e; }
    finally { setBusy(false); setWorking(''); }
  };
  const begin = (request: PluginRequest, key = '') => { void preview(request, key).catch(() => {}); };
  const apply = async () => {
    if (!review) return;
    setBusy(true); setFailure('');
    try {
      const response = await pluginsApi.apply(review.request, review.plan.revision);
      setResult(response); setFailure(failureText(t, response)); setReview(null); refresh();
      if (review.request.action === 'update') setUpdates((prev) => updatesLeft(prev, review.plan, response));
    }
    catch (e) { setFailure((e as Error).message); refresh(); }
    finally { setBusy(false); }
  };
  /** A tick on a row is its own preview and apply, with no review in between. */
  const selectTarget = async (name: string, target: PluginTarget, selected: boolean) => {
    setBusy(true); setWorking(`${name}:${target}`); setFailure(''); setResult(null);
    try {
      const request: PluginRequest = { action: selected ? 'enable' : 'disable', name, targets: [target] };
      const plan = await pluginsApi.preview(request);
      const response = await pluginsApi.apply(request, plan.revision);
      if (response.failure) setFailure(failureText(t, response));
      refresh();
    } catch (e) { setFailure((e as Error).message); }
    finally { setBusy(false); setWorking(''); }
  };

  const outcomes = result?.result?.results ?? [];
  return {
    busy, working, failure, review, updates, outcomes, checked, checking, batch,
    preview, begin, apply, selectTarget, check, updateMany,
    stopBatch: () => { stopped.current = true; },
    closeBatch: () => setBatch(null),
    closeReview: () => setReview(null),
    clearResult: () => setResult(null),
    todo: syncTodo(inventory),
    pendingUpdates: updatesPending(updates, inventory),
    ...reviewChanges(review?.plan, inventory),
    changed: changedRuns(t, outcomes),
    isPi: piPackageTest(inventory),
  };
}
