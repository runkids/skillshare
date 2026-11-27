import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { invalidate } from '../../lib/queryEvents';
import { queryKeys } from '../../lib/queryKeys';
import { bindingVersion, byPlugin, pluginsApi, syncAction, targetMap } from '../../api/plugins';
import type {
  PluginCheckEvent,
  PluginInventory,
  PluginOutcome,
  PluginPlan,
  PluginRequest,
  PluginResult,
  PluginTarget,
  PluginUpdate,
  PluginUpdateEvent,
} from '../../api/plugins';
import { ApiError } from '../../api/client';
import type { useT } from '../../i18n';
import { keyedMessage } from './outcomeText';
import { pluginErrorMessage } from './pluginError';

type T = ReturnType<typeof useT>;
type Updates = Record<string, Record<PluginTarget, PluginUpdate>>;
/** The `working` key of a group's action: its spinner shows on the group's menu button. */
export const groupKey = (names: string[]) => `group:${names[0]}`;
/**
 * What an action on several packages at once (a group's menu, the selection bar) sends for each.
 * A package whose own switch is locked (an Agent being cleaned up or on another account) is left
 * as it is; one on another account would also block the others' removal in the same request.
 * Updates are only what the last check found: the server skips an Agent with no version the user saw.
 */
export function bulkActions(inventory: PluginInventory, names: string[], pending: UpdatePick[]) {
  const bindingsOf = (name: string) => Object.values(inventory.packages[name]?.bindings ?? {});
  const free = names.filter(
    (name) => !bindingsOf(name).some((b) => b?.cleanup || b?.contextChanged),
  );
  return {
    off: free.filter((name) => bindingsOf(name).some((b) => b?.sync === false)),
    on: free.filter((name) => bindingsOf(name).some((b) => b && b.sync !== false)),
    removable: names.filter((name) => !bindingsOf(name).some((b) => b?.contextChanged)),
    picks: pending.filter((p) => names.includes(p.name)),
  };
}
/** Where a package was added from, read the way its row reads it. */
export function packageSource(inventory: PluginInventory, name: string) {
  const pack = inventory.packages[name];
  const from = Object.values(pack.bindings).find((b) => b?.source);
  const source = from?.source ?? pack.source;
  return source ? { source, ref: from?.sourceRef ?? pack.sourceRef ?? '' } : undefined;
}
/**
 * The packages grouped by the source and ref they were added from, in the order they first appear.
 * One with no source, or alone in its source, is a group of one.
 */
export function groupBySource(inventory: PluginInventory, names: string[]) {
  const groups = new Map<string, string[]>();
  for (const name of names) {
    const from = packageSource(inventory, name);
    const key = from ? `${from.source}\0${from.ref}` : `\0${name}`;
    groups.set(key, [...(groups.get(key) ?? []), name]);
  }
  return [...groups.values()];
}
/** One plugin and the Agents to update it on. */
export interface UpdatePick {
  name: string;
  targets: PluginTarget[];
}
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
  return Object.entries(inventory?.packages ?? {})
    .filter(([, p]) => {
      const b = p.bindings[target];
      return b && !b.id && !b.cleanup;
    })
    .map(([name]) => name)
    .sort();
}

export function importRequest(from: string, plugin: string, selected?: string): PluginRequest {
  const name = selected ?? '';
  return { action: 'import', from, plugin, ...(name ? { name } : {}) };
}

/** What is still to update after the last check: the Agents it found behind that are still bound, still ticked, and can be updated from here, per plugin. */
export function updatesPending(updates: Updates, inventory?: PluginInventory): UpdatePick[] {
  const targets = targetMap(inventory?.targetDefinitions);
  return Object.entries(updates)
    .map(([name, u]) => ({
      name,
      targets: Object.keys(u).filter((target) => {
        const b = inventory?.packages[name]?.bindings[target];
        return b && b.sync !== false && targets[target]?.operations.includes('update');
      }),
    }))
    .filter((p) => p.targets.length > 0);
}

/** The server joins each failed target's English text; show the translated messages once each instead. */
export function failureText(t: T, response: PluginResult) {
  const failed = new Set(
    (response.result?.results ?? [])
      .filter((r) => r.status === 'failed')
      .map((r) => keyedMessage(t, r)),
  );
  return response.failure && failed.size ? [...failed].join(' ') : response.failure;
}

/** Why a plan is blocked, in the user's language. */
const blockedText = (t: T, plan: PluginPlan) =>
  plan.changes
    .filter((c) => c.action === 'blocked')
    .map((c) => keyedMessage(t, c))
    .filter(Boolean)
    .join(' ') || t('plugins.blocked');

/** What a check found to update, per plugin and Agent (version '' when the source changed without a new one). */
export function updatesFound(plan: PluginPlan) {
  const found: Updates = {};
  for (const c of plan.changes)
    if (c.action === 'update-available')
      (found[c.name] ??= {})[c.target] = {
        version: c.binding?.version ?? '',
        commit: c.binding?.commit,
        digest: c.binding?.digest,
        id: c.binding?.id,
        source: c.binding?.source,
        sourceRef: c.binding?.sourceRef,
        plugin: c.binding?.plugin,
        entry: c.binding?.entry,
      };
  return found;
}

/** What one plugin's check answer leaves of updates: an Agent the check could not reach keeps the update found before. */
function withChecked(updates: Updates, name: string, plan: PluginPlan | null): Updates {
  const before = updates[name] ?? {};
  const found = plan ? (updatesFound(plan)[name] ?? {}) : { ...before };
  for (const c of plan?.changes ?? [])
    if (c.name === name && c.action === 'blocked' && before[c.target])
      found[c.target] = before[c.target];
  const next = { ...updates };
  if (Object.keys(found).length) next[name] = found;
  else delete next[name];
  return next;
}

/**
 * What the update dialog offers: each plugin's check answer in name order, where an update kept from an earlier check,
 * for an Agent this check could not reach or a plugin it never answered for, stands in for what this check said.
 */
export function checkedPlan(
  plans: Record<string, PluginPlan>,
  updates: Updates,
): PluginPlan | null {
  const names = [...new Set([...Object.keys(plans), ...Object.keys(updates)])].sort();
  if (names.length === 0) return null;
  const changes = names.flatMap((name) => {
    const kept = updates[name] ?? {};
    const answered = (plans[name]?.changes ?? []).filter(
      (c) => !(c.action === 'blocked' && kept[c.target]),
    );
    const offered = Object.entries(kept)
      .filter(
        ([target]) => !answered.some((c) => c.target === target && c.action === 'update-available'),
      )
      .map(([target, u]) => ({
        name,
        target,
        id: u.id ?? '',
        action: 'update-available',
        message: 'Source content changed; review an update before applying.',
        messageKey: 'plugins.note.sourceChanged',
        binding: {
          id: u.id ?? '',
          version: u.version,
          commit: u.commit,
          digest: u.digest,
          source: u.source,
          sourceRef: u.sourceRef,
          plugin: u.plugin,
          entry: u.entry,
        },
      }));
    return [...answered, ...offered];
  });
  return {
    revision: names.map((n) => plans[n]?.revision ?? '').join(''),
    blocked: changes.some((c) => c.action === 'blocked'),
    changes,
  };
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

function completedUpdate(
  c: PluginPlan['changes'][number],
  plan: PluginPlan,
  response: PluginResult,
) {
  const operations = plan.changes.filter((p) => p.name === c.name && p.target === c.target);
  const outcomes = (response.result?.results ?? []).filter(
    (r) => r.name === c.name && r.target === c.target,
  );
  return (
    c === operations.at(-1) &&
    ['update', 'install', 'noop'].includes(c.action) &&
    outcomes.length === operations.length &&
    outcomes.at(-1)?.status === (c.action === 'noop' ? 'unchanged' : 'installed')
  );
}

/** What Sync would do: every binding whose tick differs from what its Agent has. */
export function syncTodo(inventory?: PluginInventory) {
  return Object.entries(inventory?.packages ?? {})
    .flatMap(([name, pack]) =>
      Object.entries(pack.bindings).map(([target, b]) => ({
        name,
        target,
        word: syncAction(
          b!,
          inventory?.hosts.find((h) => h.target === target),
        ),
      })),
    )
    .filter((x) => x.word);
}

/**
 * A package Pi installs itself (no source Skillshare copies from) and bound only to Pi targets
 * is a Pi package: it gets its own list, in the same rows. A plugin Skillshare installs stays above.
 */
export function piPackageTest(inventory: PluginInventory | undefined) {
  const piTargets = new Set(
    (inventory?.targetDefinitions ?? []).filter((d) => d.npm).map((d) => d.target),
  );
  return (name: string) => {
    // A run can name a package config no longer has, such as one it just removed.
    const pack = inventory?.packages[name];
    if (!pack) return false;
    const bound = Object.entries(pack.bindings);
    return (
      !pack.source &&
      bound.length > 0 &&
      bound.every(([target, b]) => piTargets.has(target) && !b?.source)
    );
  };
}

/** A package bound only to the Herdr app, a mod or a launcher, gets its own list too. */
export function herdrPackageTest(inventory: PluginInventory | undefined) {
  return (name: string) => {
    const bound = Object.keys(inventory?.packages[name]?.bindings ?? {});
    return bound.length > 0 && bound.every((target) => target === 'herdr-app');
  };
}

/** The Agents a batch row did update: all of them when done, the rest of a row whose `failed` names only some. */
export function updatedTargets(item: BatchItem) {
  if (item.status === 'done') return item.targets;
  return (item.status === 'failed' || item.status === 'skipped') && item.failed
    ? item.targets.filter((x) => !item.failed!.includes(x))
    : [];
}

/**
 * Each Agent's own move, from what it has to what its update brings. Agents of one plugin can come from
 * different sources, so they share one line only when every move is the same.
 */
export function versionMoves(
  inventory: PluginInventory | undefined,
  name: string,
  rows: { target: PluginTarget; version?: string; commit?: string }[],
) {
  const moves = rows.map((r) => ({
    target: r.target,
    from: installedVersion(inventory, name, r.target),
    to: r.version || undefined,
    fromCommit: inventory?.packages[name]?.bindings[r.target]?.commit,
    toCommit: r.commit,
  }));
  const key = (m: (typeof moves)[number]) => [m.from, m.to, m.fromCommit, m.toCommit].join('\0');
  return { moves, shared: moves.every((m) => key(m) === key(moves[0])) };
}

export function installedVersion(
  inventory: PluginInventory | undefined,
  name: string,
  target: string,
) {
  const b = inventory?.packages[name]?.bindings[target];
  return b && inventory ? bindingVersion(inventory, target, b) : undefined;
}

/** A preview lists every binding; the ones it leaves alone (`idle`) fold away under the ones it changes (`active`). */
export function reviewChanges(plan: PluginPlan | undefined, inventory?: PluginInventory) {
  const changes = plan?.changes ?? [];
  return {
    active: changes.filter((c) => c.action !== 'noop' || c.guidance),
    idle: changes
      .filter((c) => c.action === 'noop' && !c.guidance)
      .map((c) => ({
        name: c.name,
        target: c.target,
        version: c.binding?.version ?? installedVersion(inventory, c.name, c.target),
      })),
  };
}

/** What the last run changed, failures first; Agents that ended the same way share a row. */
export function changedRuns(t: T, outcomes: PluginOutcome[]) {
  return byPlugin(
    outcomes
      .filter((r) => r.status !== 'unchanged' || r.guidance)
      .map((r) => ({ ...r, note: r.message ? keyedMessage(t, r) : '' })),
    (r) => `${r.status}\0${r.note}`,
  ).sort((a, b) => Number(b.status === 'failed') - Number(a.status === 'failed'));
}

/**
 * The Plugins page's preview → review → apply flow and what it derives from the inventory.
 * A change is written only by `apply`, and only with the revision its preview returned.
 */
export function usePluginsFlow({
  t,
  inventory,
  refresh,
  onPreviewed,
}: {
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
  const cache = useQueryClient();
  const key = queryKeys.pluginCheck;
  type Snapshot = {
    updates: Updates;
    answers: Record<string, PluginCheckEvent>;
    done: number;
  };
  const total = Object.keys(inventory?.packages ?? {}).length;
  const query = useQuery({
    queryKey: key,
    enabled: total > 0,
    retry: false,
    queryFn: async ({ signal }): Promise<Snapshot> => {
      const before = cache.getQueryData<Snapshot>(key);
      let updates = before?.updates ?? {};
      const answers: Snapshot['answers'] = {};
      let done = 0;
      // An invalidation before the first answer must replace this stream, not join it.
      cache.setQueryData<Snapshot>(
        key,
        { updates, answers: before?.answers ?? {}, done },
        { updatedAt: 1 },
      );
      await pluginsApi.check((answer) => {
        if (signal.aborted) return;
        const { name, plan } = answer;
        updates = withChecked(updates, name, plan);
        answers[name] = answer;
        // Partial streams remain stale; only a completed check starts the freshness window.
        cache.setQueryData<Snapshot>(
          key,
          {
            updates,
            answers: { ...before?.answers, ...answers },
            done: ++done,
          },
          { updatedAt: 1 },
        );
      }, signal);
      return {
        updates: Object.fromEntries(
          Object.entries(updates).filter(([name]) => Object.hasOwn(answers, name)),
        ),
        answers,
        done,
      };
    },
  });
  const updates = query.data?.updates ?? {};
  const setUpdates = (change: (prev: Updates) => Updates) =>
    cache.setQueryData<Snapshot>(key, (prev) => prev && { ...prev, updates: change(prev.updates) });
  const checkedPlans: Record<string, PluginPlan> = {};
  const checkFailures: Record<string, string> = {};
  for (const { name, plan, error, error_code, error_params } of Object.values(
    query.data?.answers ?? {},
  )) {
    if (plan) checkedPlans[name] = plan;
    if (error)
      checkFailures[name] = pluginErrorMessage(
        new ApiError(200, error, { code: error_code, params: error_params }),
        t,
      );
    else if (plan?.blocked) checkFailures[name] = blockedText(t, plan);
  }
  const checking = query.isFetching;
  const checkProgress = { done: query.data?.done ?? 0, total };
  const check = async () => {
    setFailure('');
    const result = await query.refetch({ cancelRefetch: false });
    if (result.error) setFailure(result.error.message);
    return !result.error;
  };
  const [batch, setBatch] = useState<BatchItem[] | null>(null);

  /**
   * One server stream updates the picked plugins one after another, so one failure stops nothing else.
   * The server skips a plugin whose source moved past what the check showed: the user approved those versions.
   */
  const updateMany = async (picks: UpdatePick[]) => {
    setBusy(true);
    setFailure('');
    setResult(null);
    // A retry puts its row back to waiting where it is, keeping every Agent it showed; a first run starts the batch.
    const names = new Set(picks.map((p) => p.name));
    const added = (p: UpdatePick): BatchItem => {
      const seen = updates[p.name];
      return {
        ...p,
        status: 'waiting',
        ...(seen && {
          versions: versionMoves(
            inventory,
            p.name,
            p.targets.map((target) => ({ target, ...seen[target] })),
          ),
        }),
      };
    };
    setBatch((prev) => [
      ...(prev ?? []).map((i) =>
        names.has(i.name) ? { ...i, status: 'waiting' as const, note: undefined } : i,
      ),
      ...picks.filter((p) => !prev?.some((i) => i.name === p.name)).map(added),
    ]);
    const set = (name: string, patch: Partial<BatchItem>) =>
      setBatch((prev) => prev && prev.map((i) => (i.name === name ? { ...i, ...patch } : i)));
    let staleSeen = false;
    const finish = (e: Extract<PluginUpdateEvent, { event: 'result' }>) => {
      const targets = picks.find((p) => p.name === e.name)?.targets ?? [];
      if (e.stale) {
        staleSeen = true;
        setUpdates((prev) =>
          Object.fromEntries(Object.entries(prev).filter(([name]) => name !== e.name)),
        );
        set(e.name, { status: 'skipped', stale: true, note: t('plugins.updateDialog.stale') });
        return;
      }
      if (e.plan?.blocked) {
        set(e.name, { status: 'failed', note: blockedText(t, e.plan), failed: targets });
        return;
      }
      if (!e.plan || !e.result) {
        set(e.name, { status: 'failed', note: e.failure, failed: targets });
        return;
      }
      const response = { result: e.result, failure: e.failure };
      const plan = e.plan;
      setUpdates((prev) => updatesLeft(prev, plan, response));
      const results = e.result.results;
      const failed = failureText(t, response);
      const failedTargets = results.filter((r) => r.status === 'failed').map((r) => r.target);
      const skipped = results.filter((r) => r.status === 'skipped');
      if (failed)
        set(e.name, {
          status: 'failed',
          note: failed,
          failed: failedTargets.length ? failedTargets : targets,
        });
      else if (skipped.length)
        set(e.name, {
          status: 'skipped',
          note:
            skipped
              .map((r) => keyedMessage(t, r))
              .filter(Boolean)
              .join(' ') || undefined,
          failed: skipped.map((r) => r.target),
        });
      else
        set(e.name, {
          status: 'done',
          note:
            results
              .filter((r) => r.guidance)
              .map((r) => keyedMessage(t, r))
              .join(' ') || undefined,
          failed: undefined,
        });
    };
    let broken = '';
    try {
      await pluginsApi.update(
        picks.map((p) => ({ ...p, seen: updates[p.name] })),
        (e) => {
          if (e.event === 'start') set(e.name, { status: 'running' });
          // What the Agent is about to do (an npm install, say) shows while it runs.
          else if (e.event === 'plan')
            set(e.name, {
              note:
                e.plan.changes
                  .filter((c) => c.action !== 'noop' || c.guidance)
                  .map((c) => keyedMessage(t, c))
                  .filter(Boolean)
                  .join(' ') || undefined,
            });
          else if (e.event === 'result') finish(e);
          else if (e.failure) setFailure(e.failure);
        },
      );
    } catch (e) {
      broken = (e as Error).message;
      setFailure(broken);
    }
    // A plugin the stream never finished was not updated here: a stop skips it, a stream that broke off fails it with why.
    const unfinished = (i: BatchItem): BatchItem =>
      broken
        ? {
            ...i,
            status: 'failed',
            note: broken,
            failed: picks.find((p) => p.name === i.name)?.targets,
          }
        : { ...i, status: 'skipped' };
    setBatch(
      (prev) =>
        prev &&
        prev.map((i) => (i.status === 'waiting' || i.status === 'running' ? unfinished(i) : i)),
    );
    setBusy(false);
    refresh();
    // A source that moved is offered again only by a fresh check; the user started this, so its failure shows.
    if (staleSeen) void check();
  };

  // `key` names the control that started this, so only it shows a spinner.
  const preview = async (request: PluginRequest, key = '') => {
    setBusy(true);
    setWorking(key);
    setFailure('');
    setResult(null);
    try {
      const plan = await pluginsApi.preview(request);
      setReview({ request, plan });
      onPreviewed();
    } catch (e) {
      setFailure(pluginErrorMessage(e, t));
      throw e;
    } finally {
      setBusy(false);
      setWorking('');
    }
  };
  const begin = (request: PluginRequest, key = '') => {
    void preview(request, key).catch(() => {});
  };
  const apply = async () => {
    if (!review) return;
    setBusy(true);
    setFailure('');
    try {
      const response = await pluginsApi.apply(review.request, review.plan.revision);
      setResult(response);
      setFailure(failureText(t, response));
      setReview(null);
      if (review.request.action === 'update')
        setUpdates((prev) => updatesLeft(prev, review.plan, response));
      refresh();
    } catch (e) {
      setFailure((e as Error).message);
      refresh();
    } finally {
      setBusy(false);
    }
  };
  /**
   * A tick on a row is its own preview and apply, with no review in between. Several names, from a
   * group, go as one request; without a target every Agent of each package turns on or off.
   */
  const setSync = async (
    names: string[],
    target: PluginTarget | undefined,
    selected: boolean,
    key: string,
  ): Promise<boolean> => {
    setBusy(true);
    setWorking(key);
    setFailure('');
    setResult(null);
    try {
      const request: PluginRequest = {
        action: selected ? 'enable' : 'disable',
        ...(names.length === 1 ? { name: names[0] } : { names }),
        ...(target && { targets: [target] }),
      };
      const plan = await pluginsApi.preview(request);
      const response = await pluginsApi.apply(request, plan.revision);
      if (response.failure) setFailure(failureText(t, response));
      else {
        // Show the tick at once. The refetch below asks every Agent's CLI and takes seconds, and until
        // it answers the row would show the old state again.
        // Only what the plan saved: a skipped Agent, as when Herdr app's home is absent, keeps its state.
        const saved = plan.changes.filter((c) => c.action === 'selection');
        const tick = (inv?: PluginInventory) => {
          if (!inv) return inv;
          const packages = { ...inv.packages };
          for (const { name, target: at } of saved) {
            const pkg = packages[name];
            const binding = pkg?.bindings[at];
            if (!pkg || !binding) continue;
            packages[name] = {
              ...pkg,
              bindings: { ...pkg.bindings, [at]: { ...binding, sync: selected } },
            };
          }
          return { ...inv, packages };
        };
        cache.setQueryData<PluginInventory>(queryKeys.plugins, tick);
        cache.setQueryData<PluginInventory>(queryKeys.pluginPackages, tick);
      }
      // Not refresh(): that reruns the update check, which asks every Agent again for nothing.
      void invalidate(cache, 'pluginTargetsToggled');
      return !response.failure;
    } catch (e) {
      setFailure((e as Error).message);
      return false;
    } finally {
      setBusy(false);
      setWorking('');
    }
  };
  const selectTarget = (name: string, target: PluginTarget, selected: boolean) =>
    setSync([name], target, selected, `${name}:${target}`);
  /** Turns every Agent of these packages on or off; `key` is the group's spinner. Resolves to whether it saved. */
  const selectMany = (names: string[], selected: boolean, key: string) =>
    setSync(names, undefined, selected, key);

  /**
   * Each import is its own preview and apply, as a tick is: every write moves the revision, so a batch has
   * no one plan to review. An import adopts what the Agent has without touching it; a blocked one is skipped.
   */
  const importMany = async (requests: PluginRequest[]) => {
    setBusy(true);
    setWorking('import');
    setFailure('');
    setResult(null);
    const results: PluginOutcome[] = [];
    const failures: string[] = [];
    for (const request of requests) {
      try {
        const plan = await pluginsApi.preview(request);
        if (plan.blocked) {
          failures.push(`${request.plugin}: ${blockedText(t, plan)}`);
          continue;
        }
        const response = await pluginsApi.apply(request, plan.revision);
        results.push(...(response.result?.results ?? []));
        const failed = failureText(t, response);
        if (failed) failures.push(failed);
      } catch (e) {
        failures.push(`${request.plugin}: ${pluginErrorMessage(e, t)}`);
      }
    }
    setResult({ result: { results }, failure: failures.join(' ') });
    setFailure(failures.join(' '));
    setBusy(false);
    setWorking('');
    refresh();
  };

  const outcomes = result?.result?.results ?? [];
  return {
    busy,
    working,
    failure,
    review,
    updates,
    outcomes,
    checked: checkedPlan(checkedPlans, updates),
    checking,
    checkProgress,
    checkFailures,
    batch,
    preview,
    begin,
    apply,
    selectTarget,
    selectMany,
    check,
    updateMany,
    importMany,
    stopBatch: () => {
      pluginsApi.stopUpdate().catch((e: Error) => setFailure(e.message));
    },
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
