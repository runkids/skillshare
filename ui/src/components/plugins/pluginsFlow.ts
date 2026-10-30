import { useState } from 'react';
import { bindingVersion, byPlugin, pluginsApi, syncAction, targetMap } from '../../api/plugins';
import type { PluginInventory, PluginOutcome, PluginPlan, PluginRequest, PluginResult, PluginTarget, PluginUpdate, PluginUpdateEvent } from '../../api/plugins';
import type { useT } from '../../i18n';
import { keyedMessage } from './outcomeText';
import { pluginErrorMessage } from './pluginError';

type T = ReturnType<typeof useT>;
type Updates = Record<string, Record<PluginTarget, PluginUpdate>>;
/** One plugin and the Agents to update it on. */
export interface UpdatePick { name: string; targets: PluginTarget[] }
/** One plugin in a batch update. */
export interface BatchItem extends UpdatePick {
  status: 'waiting' | 'running' | 'done' | 'failed' | 'skipped';
  /** Each Agent's old → new as the check showed it, kept once the refreshed page already lists the new version. */
  versions?: ReturnType<typeof versionMoves>;
  note?: string;
  /** The Agents the last try failed on; a retry sends only these. */
  failed?: PluginTarget[];
  /** The source moved past what the check showed: nothing was applied, and only a new check can show what it would bring. */
  stale?: boolean;
}

/** Existing declarations awaiting a local binding in this target. */
export function importPackageNames(inventory: PluginInventory | undefined, target: string) {
  return Object.entries(inventory?.packages ?? {}).filter(([, p]) => {
    const b = p.bindings[target];
    return b && !b.id && !b.cleanup;
  }).map(([name]) => name).sort();
}

export function importRequest(from: string, plugin: string, selected?: string): PluginRequest {
  const name = selected ?? '';
  return { action: 'import', from, plugin, ...(name ? { name } : {}) };
}

/** What is still to update after the last check: the Agents it found behind that are still bound, still ticked, and can be updated from here, per plugin. */
export function updatesPending(updates: Updates, inventory?: PluginInventory): UpdatePick[] {
  const targets = targetMap(inventory?.targetDefinitions);
  return Object.entries(updates)
    .map(([name, u]) => ({ name, targets: Object.keys(u).filter((target) => { const b = inventory?.packages[name]?.bindings[target]; return b && b.sync !== false && targets[target]?.operations.includes('update'); }) }))
    .filter((p) => p.targets.length > 0);
}

/** The server joins each failed target's English text; show the translated messages once each instead. */
export function failureText(t: T, response: PluginResult) {
  const failed = new Set((response.result?.results ?? []).filter((r) => r.status === 'failed').map((r) => keyedMessage(t, r)));
  return response.failure && failed.size ? [...failed].join(' ') : response.failure;
}

/** Why a plan is blocked, in the user's language. */
const blockedText = (t: T, plan: PluginPlan) => plan.changes.filter((c) => c.action === 'blocked').map((c) => keyedMessage(t, c)).join(' ') || t('plugins.blocked');

/** What a check found to update, per plugin and Agent (version '' when the source changed without a new one). */
export function updatesFound(plan: PluginPlan) {
  const found: Updates = {};
  for (const c of plan.changes) if (c.action === 'update-available') (found[c.name] ??= {})[c.target] = { version: c.binding?.version ?? '', commit: c.binding?.commit, digest: c.binding?.digest, id: c.binding?.id, source: c.binding?.source, sourceRef: c.binding?.sourceRef, plugin: c.binding?.plugin, entry: c.binding?.entry };
  return found;
}

/**
 * An Agent comes off once the result reports it updated, or already there (noop); one that
 * failed, was skipped, or never ran because the apply stopped first keeps its Update action.
 */
export function updatesLeft(updates: Updates, plan: PluginPlan, response: PluginResult) {
  const next = { ...updates };
  for (const c of plan.changes) {
    if (!completedUpdate(c, plan, response) || !next[c.name]) continue;
    next[c.name] = { ...next[c.name] };
    delete next[c.name][c.target];
  }
  return next;
}

function completedUpdate(c: PluginPlan['changes'][number], plan: PluginPlan, response: PluginResult) {
  const operations = plan.changes.filter((p) => p.name === c.name && p.target === c.target);
  const outcomes = (response.result?.results ?? []).filter((r) => r.name === c.name && r.target === c.target);
  return c === operations.at(-1) && ['update', 'install', 'noop'].includes(c.action)
    && outcomes.length === operations.length
    && outcomes.at(-1)?.status === (c.action === 'noop' ? 'unchanged' : 'installed');
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

/** The Agents a batch row did update: all of them when done, the rest of a row whose `failed` names only some. */
export function updatedTargets(item: BatchItem) {
  if (item.status === 'done') return item.targets;
  return (item.status === 'failed' || item.status === 'skipped') && item.failed ? item.targets.filter((x) => !item.failed!.includes(x)) : [];
}

/**
 * Each Agent's own move, from what it has to what its update brings. Agents of one plugin can come from
 * different sources, so they share one line only when every move is the same.
 */
export function versionMoves(inventory: PluginInventory | undefined, name: string, rows: { target: PluginTarget; version?: string; commit?: string }[]) {
  const moves = rows.map((r) => ({ target: r.target, from: installedVersion(inventory, name, r.target), to: r.version || undefined, fromCommit: inventory?.packages[name]?.bindings[r.target]?.commit, toCommit: r.commit }));
  const key = (m: (typeof moves)[number]) => [m.from, m.to, m.fromCommit, m.toCommit].join('\0');
  return { moves, shared: moves.every((m) => key(m) === key(moves[0])) };
}

export function installedVersion(inventory: PluginInventory | undefined, name: string, target: string) {
  const b = inventory?.packages[name]?.bindings[target];
  return b && inventory ? bindingVersion(inventory, target, b) : undefined;
}

/** A preview lists every binding; the ones it leaves alone (`idle`) fold away under the ones it changes (`active`). */
export function reviewChanges(plan: PluginPlan | undefined, inventory?: PluginInventory) {
  const changes = plan?.changes ?? [];
  return {
    active: changes.filter((c) => c.action !== 'noop' || c.guidance),
    idle: changes.filter((c) => c.action === 'noop' && !c.guidance).map((c) => ({ name: c.name, target: c.target, version: c.binding?.version ?? installedVersion(inventory, c.name, c.target) })),
  };
}

/** What the last run changed, failures first; Agents that ended the same way share a row. */
export function changedRuns(t: T, outcomes: PluginOutcome[]) {
  return byPlugin(outcomes.filter((r) => r.status !== 'unchanged' || r.guidance).map((r) => ({ ...r, note: r.message ? keyedMessage(t, r) : '' })), (r) => `${r.status}\0${r.note}`)
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

  /** A check is read-only and opens nothing: it only marks the rows. Quiet, as on page load, it also keeps its error to itself. */
  /** Resolves whether the check came back, so a check the user asked for can show its result. */
  const check = async (quiet = false) => {
    setChecking(true); if (!quiet) setFailure('');
    try { const plan = await pluginsApi.preview({ action: 'check' }); setUpdates(updatesFound(plan)); setChecked(plan); return true; }
    catch (e) { if (!quiet) setFailure((e as Error).message); return false; }
    finally { setChecking(false); }
  };
  /**
   * One server stream updates the picked plugins one after another, so one failure stops nothing else.
   * The server skips a plugin whose source moved past what the check showed: the user approved those versions.
   */
  const updateMany = async (picks: UpdatePick[]) => {
    setBusy(true); setFailure(''); setResult(null);
    // A retry puts its row back to waiting where it is, keeping every Agent it showed; a first run starts the batch.
    const names = new Set(picks.map((p) => p.name));
    const added = (p: UpdatePick): BatchItem => {
      const seen = updates[p.name];
      return { ...p, status: 'waiting', ...(seen && { versions: versionMoves(inventory, p.name, p.targets.map((target) => ({ target, ...seen[target] }))) }) };
    };
    setBatch((prev) => [...(prev ?? []).map((i) => (names.has(i.name) ? { ...i, status: 'waiting' as const, note: undefined } : i)), ...picks.filter((p) => !prev?.some((i) => i.name === p.name)).map(added)]);
    const set = (name: string, patch: Partial<BatchItem>) => setBatch((prev) => prev && prev.map((i) => (i.name === name ? { ...i, ...patch } : i)));
    let staleSeen = false;
    const finish = (e: Extract<PluginUpdateEvent, { event: 'result' }>) => {
      const targets = picks.find((p) => p.name === e.name)?.targets ?? [];
      if (e.stale) { staleSeen = true; setUpdates((prev) => Object.fromEntries(Object.entries(prev).filter(([name]) => name !== e.name))); set(e.name, { status: 'skipped', stale: true, note: t('plugins.updateDialog.stale') }); return; }
      if (e.plan?.blocked) { set(e.name, { status: 'failed', note: blockedText(t, e.plan), failed: targets }); return; }
      if (!e.plan || !e.result) { set(e.name, { status: 'failed', note: e.failure, failed: targets }); return; }
      const response = { result: e.result, failure: e.failure };
      const plan = e.plan;
      setUpdates((prev) => updatesLeft(prev, plan, response));
      const results = e.result.results;
      const failed = failureText(t, response);
      const failedTargets = results.filter((r) => r.status === 'failed').map((r) => r.target);
      const skipped = results.filter((r) => r.status === 'skipped');
      if (failed) set(e.name, { status: 'failed', note: failed, failed: failedTargets.length ? failedTargets : targets });
      else if (skipped.length) set(e.name, { status: 'skipped', note: skipped.map((r) => keyedMessage(t, r)).join(' ') || undefined, failed: skipped.map((r) => r.target) });
      else set(e.name, { status: 'done', note: results.filter((r) => r.guidance).map((r) => keyedMessage(t, r)).join(' ') || undefined, failed: undefined });
    };
    let broken = '';
    try {
      await pluginsApi.update(picks.map((p) => ({ ...p, seen: updates[p.name] })), (e) => {
        if (e.event === 'start') set(e.name, { status: 'running' });
        // What the Agent is about to do (an npm install, say) shows while it runs.
        else if (e.event === 'plan') set(e.name, { note: e.plan.changes.filter((c) => c.action !== 'noop' || c.guidance).map((c) => keyedMessage(t, c)).join(' ') || undefined });
        else if (e.event === 'result') finish(e);
        else if (e.failure) setFailure(e.failure);
      });
    } catch (e) { broken = (e as Error).message; setFailure(broken); }
    // A plugin the stream never finished was not updated here: a stop skips it, a stream that broke off fails it with why.
    const unfinished = (i: BatchItem): BatchItem => (broken ? { ...i, status: 'failed', note: broken, failed: picks.find((p) => p.name === i.name)?.targets } : { ...i, status: 'skipped' });
    setBatch((prev) => prev && prev.map((i) => (i.status === 'waiting' || i.status === 'running' ? unfinished(i) : i)));
    setBusy(false); refresh();
    // A source that moved is offered again only by a fresh check; the user started this, so its failure shows.
    if (staleSeen) void check();
  };

  // `key` names the control that started this, so only it shows a spinner.
  const preview = async (request: PluginRequest, key = '') => {
    setBusy(true); setWorking(key); setFailure(''); setResult(null);
    try {
      const plan = await pluginsApi.preview(request);
      setReview({ request, plan }); onPreviewed();
    }
    catch (e) { setFailure(pluginErrorMessage(e, t)); throw e; }
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

  /**
   * Each import is its own preview and apply, as a tick is: every write moves the revision, so a batch has
   * no one plan to review. An import adopts what the Agent has without touching it; a blocked one is skipped.
   */
  const importMany = async (requests: PluginRequest[]) => {
    setBusy(true); setWorking('import'); setFailure(''); setResult(null);
    const results: PluginOutcome[] = [];
    const failures: string[] = [];
    for (const request of requests) {
      try {
        const plan = await pluginsApi.preview(request);
        if (plan.blocked) { failures.push(`${request.plugin}: ${blockedText(t, plan)}`); continue; }
        const response = await pluginsApi.apply(request, plan.revision);
        results.push(...(response.result?.results ?? []));
        const failed = failureText(t, response);
        if (failed) failures.push(failed);
      } catch (e) { failures.push(`${request.plugin}: ${pluginErrorMessage(e, t)}`); }
    }
    setResult({ result: { results }, failure: failures.join(' ') }); setFailure(failures.join(' '));
    setBusy(false); setWorking(''); refresh();
  };

  const outcomes = result?.result?.results ?? [];
  return {
    busy, working, failure, review, updates, outcomes, checked, checking, batch,
    preview, begin, apply, selectTarget, check, updateMany, importMany,
    stopBatch: () => { pluginsApi.stopUpdate().catch((e: Error) => setFailure(e.message)); },
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
