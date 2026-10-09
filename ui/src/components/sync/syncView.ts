import { api, type DiffTarget, type ExtraDiffResult, type Skill, type SyncMatrixEntry, type SyncResponse, type Target } from '../../api/client';
import { hooksApi, type HookPlan } from '../../api/hooks';
import { mcpApi, type MCPPlan } from '../../api/mcp';
import { formatAgentDisplayName } from '../../lib/resourceNames';
import { groupByFile as hookFiles } from '../hooks/hooksView';
import { groupByFile, projectOf, type MCPChange } from '../mcp/mcpView';

export type Part = 'skill' | 'agent' | 'extra' | 'mcp' | 'hooks';
export type RowIcon = 'add' | 'adopt' | 'update' | 'remove' | 'kept' | 'conflict';

export interface ChangeRow {
  key: string;
  part: Part;
  name: string;
  icon: RowIcon;
  /** i18n key, or null when `detail` is backend text shown as is */
  text: string | null;
  detail?: string;
  /** False for rows sync leaves alone (kept local copies, conflicts) */
  counts: boolean;
  /** Edited inside the target: sync keeps it unless Force is on */
  edited?: boolean;
  /** An MCP switch: added, it turns a global server off in a project; removed, back on */
  switch?: boolean;
}

export interface ChangeGroup {
  key: string;
  part: 'target' | 'extra' | 'mcp' | 'hooks';
  name: string;
  mode?: string;
  path?: string;
  /** Set on Claude Code's off list, which sits in the global file: the project it is for */
  project?: string;
  rows: ChangeRow[];
}

/** The server's reason (CopyToLinkReason) for a copy left by copy mode that a merge sync turns into a link. */
const COPY_TO_LINK = 'copy mode copy (sync replaces with link)';

const editedRow = (force: boolean) => ({ icon: force ? 'update' : 'kept', text: force ? 'sync.row.forceReplace' : 'sync.row.kept', counts: force, edited: true }) as const;

const flatKey = (name: string) => name.replace(/\//g, '__').replace(/\.md$/i, '');

/**
 * One group per target with pending skill or agent changes, in name order (the API order changes between requests).
 * Local-only items are listed elsewhere. `ignored` names the .skillignore/.agentignore matches, which prune like deleted items.
 */
export function resourceGroups(diffs: DiffTarget[], targets: Target[], parts: Set<Part>, force: boolean, ignored: { skill?: string[]; agent?: string[] } = {}): { groups: ChangeGroup[]; inSync: string[] } {
  const groups: ChangeGroup[] = [];
  const inSync: string[] = [];
  const ignoredKeys = { skill: new Set((ignored.skill ?? []).map(flatKey)), agent: new Set((ignored.agent ?? []).map(flatKey)) };
  for (const d of [...diffs].sort((a, b) => a.target.localeCompare(b.target))) {
    const target = targets.find((t) => t.name === d.target);
    const rows: ChangeRow[] = [];
    for (const item of d.items ?? []) {
      const part = item.kind === 'agent' ? 'agent' : 'skill';
      if (!parts.has(part) || item.action === 'local') continue;
      // Agents always sync as symlinks, whatever the target mode.
      const copy = part === 'skill' && target?.mode === 'copy';
      const row = { key: `${d.target}/${part}/${item.skill}`, part, name: part === 'agent' ? formatAgentDisplayName(item.skill) : item.skill } as const;
      // A kept item: a local folder holds the new name, so sync keeps the old entry, with or without force.
      if (item.skill === '(target naming)' || item.action === 'kept') rows.push({ ...row, icon: 'kept', text: null, detail: item.reason, counts: false });
      else if (item.skill === '(entire directory)') rows.push({ ...row, name: target?.path ?? item.skill, icon: 'add', text: 'sync.row.folder', counts: true });
      else if (item.action === 'link') rows.push({ ...row, icon: 'add', text: item.reason?.startsWith('missing') ? 'sync.row.recopy' : copy ? 'sync.row.newCopy' : 'sync.row.newLink', counts: true });
      else if (item.action === 'update') rows.push({ ...row, icon: 'update', text: item.reason === COPY_TO_LINK ? 'sync.row.copyToLink' : copy ? 'sync.row.updateCopy' : 'sync.row.updateLink', counts: true });
      else if (item.action === 'prune') rows.push({ ...row, icon: 'remove', text: ignoredKeys[part].has(flatKey(item.skill)) ? 'sync.row.pruneIgnored' : 'sync.row.prune', counts: true });
      else if (item.action === 'skip') rows.push({ ...row, ...editedRow(force) });
    }
    if (rows.length) groups.push({ key: `target/${d.target}`, part: 'target', name: d.target, mode: target?.mode, path: target?.path, rows });
    else inSync.push(d.target);
  }
  return { groups, inSync };
}

// A regular file where a link belongs, or a copy that differs, is only overwritten with force (syncOneExtraFile).
const EXTRA_EDITED = new Set(['not a symlink', 'content differs']);

export function extraGroups(diffs: ExtraDiffResult[], force: boolean): ChangeGroup[] {
  return diffs.filter((d) => d.items?.length).map((d) => ({
    key: `extra/${d.name}/${d.target}`,
    part: 'extra',
    name: d.name,
    mode: d.mode,
    path: d.target,
    // Without a source folder the sync only creates it, so nothing lands in the target.
    rows: d.items.map((i) => (i.reason === 'no source directory'
      ? { key: `${d.name}/${d.target}/*`, part: 'extra', name: d.name, icon: 'kept', text: 'sync.row.extraNoSource', counts: false }
      : {
        key: `${d.name}/${d.target}/${i.file}`,
        part: 'extra',
        name: i.file,
        ...(EXTRA_EDITED.has(i.reason) ? editedRow(force) : {
          icon: i.action === 'update' ? 'update' : 'add',
          text: i.action === 'update' ? 'sync.row.extraUpdate' : 'sync.row.extraCreate',
          counts: true,
        }),
      })),
  }));
}

const MCP_ICON: Record<string, RowIcon> = { add: 'add', adopt: 'adopt', update: 'update', remove: 'remove', conflict: 'conflict' };

export function mcpGroups(plan: MCPPlan | null | undefined): ChangeGroup[] {
  return groupByFile((plan?.changes ?? []).filter((c) => MCP_ICON[c.action])).map((file) => ({
    key: `mcp/${file.key}`,
    part: 'mcp',
    name: file.target,
    path: file.path,
    project: file.offListFor,
    rows: file.changes.map((c: MCPChange) => ({
      key: `${file.key}/${c.name}`,
      part: 'mcp',
      name: c.name,
      icon: MCP_ICON[c.action],
      // A switch-only entry adds or removes no server: it turns one off for a project, or back on.
      text: c.action === 'conflict' ? null : c.switch && c.action !== 'update' ? `sync.row.mcp.switch.${c.action}` : `sync.row.mcp.${c.action}`,
      detail: [c.message, ...(c.fields?.added ?? []).map((key) => `+ ${key}`), ...(c.fields?.updated ?? []).map((key) => `~ ${key}`), ...(c.fields?.removed ?? []).map((key) => `− ${key}`)].filter(Boolean).join(' · ') || undefined,
      counts: c.action !== 'conflict',
      switch: c.switch,
    })),
  }));
}

const HOOK_ICON: Record<string, RowIcon> = { add: 'add', adopt: 'adopt', update: 'update', restore: 'update', remove: 'remove', conflict: 'conflict' };

/** One group per native file; a project's file names its folder. */
export function hooksGroups(plan: HookPlan | null | undefined): ChangeGroup[] {
  return hookFiles((plan?.changes ?? []).filter((c) => HOOK_ICON[c.action])).map((file) => ({
    key: `hooks/${file.path}`,
    part: 'hooks',
    name: file.target,
    path: file.path,
    project: file.changes[0].root,
    rows: file.changes.map((c) => ({
      key: `${file.path}/${c.name}`,
      part: 'hooks',
      name: c.name,
      icon: HOOK_ICON[c.action],
      text: c.action === 'conflict' ? null : `sync.row.hooks.${c.action === 'restore' ? 'update' : c.action}`,
      detail: c.message,
      counts: c.action !== 'conflict',
    })),
  }));
}

/** Targets already in sync, the global ones apart from each project's: a project target is `<project>@<tool>`. */
export function groupInSync(names: string[]) {
  const global: string[] = [];
  const projects = new Map<string, string[]>();
  for (const name of names) {
    const at = name.lastIndexOf('@');
    if (at < 0) global.push(name);
    else projects.set(name.slice(0, at), [...(projects.get(name.slice(0, at)) ?? []), name.slice(at + 1)]);
  }
  return { global, projects: [...projects].map(([project, tools]) => ({ project, tools })) };
}

/** Ignored names by folder, so a shared prefix such as `security/` is written once. */
export function groupByFolder(names: string[]) {
  const folders = new Map<string, string[]>();
  for (const name of names) {
    const slash = name.lastIndexOf('/');
    const folder = slash < 0 ? '' : name.slice(0, slash + 1);
    folders.set(folder, [...(folders.get(folder) ?? []), name.slice(slash + 1)]);
  }
  return [...folders].map(([folder, items]) => ({ folder, items }));
}

/** Targets getting exactly the same changes, as one set: twelve targets adding the same 38 skills read as one list. */
export function changeSets(groups: ChangeGroup[]): { key: string; targets: ChangeGroup[]; rows: ChangeRow[] }[] {
  const sets = new Map<string, { key: string; targets: ChangeGroup[]; rows: ChangeRow[] }>();
  for (const g of groups) {
    const sig = g.rows.map((r) => `${r.part}\t${r.name}\t${r.icon}\t${r.text}\t${r.detail ?? ''}`).sort().join('\n');
    const set = sets.get(sig);
    if (set) set.targets.push(g);
    else sets.set(sig, { key: g.key, targets: [g], rows: g.rows });
  }
  return [...sets.values()].sort((a, b) => b.rows.length - a.rows.length);
}

/** Rows per icon, conflicts counted as kept: both leave the target's copy alone. */
export function tally(rows: ChangeRow[]): Partial<Record<RowIcon, number>> {
  const n: Partial<Record<RowIcon, number>> = {};
  for (const r of rows) {
    const icon = r.icon === 'conflict' ? 'kept' : r.icon;
    n[icon] = (n[icon] ?? 0) + 1;
  }
  return n;
}

/**
 * Whether a target gets a skill: by its filters, or always when it links the whole source folder.
 * The matrix marks symlink-mode targets `na` since filters don't apply, yet every source skill is live there.
 */
export const receivesSkill = (e: SyncMatrixEntry) => e.status === 'synced' || e.reasonCode === 'sync_matrix.symlink_filters_not_applicable';

/**
 * The skills Discard all moves to trash: never synced anywhere yet, so new in every target that should get them.
 * `expected` names the targets a skill syncs to (the sync matrix). A skill already in one of them was synced before
 * and stays, even when a new target is about to get it. Install times can't tell this: tracked repo skills have none.
 * A tracked repo counts only as a whole, since uninstalling one skill of it removes the repo.
 */
export function discardable(groups: ChangeGroup[], skills: Skill[], expected: (flatName: string) => string[]): Skill[] {
  const added = new Map<string, Set<string>>();
  for (const g of groups) {
    for (const r of g.rows) {
      if (r.part !== 'skill' || r.icon !== 'add' || r.text === 'sync.row.recopy') continue;
      added.set(r.name, (added.get(r.name) ?? new Set()).add(g.name));
    }
  }
  const fresh = (s: Skill) => s.kind === 'skill' && added.has(s.flatName) && expected(s.flatName).every((t) => added.get(s.flatName)!.has(t));
  // A tracked repo uninstalls whole, so it qualifies only when every skill in it is new.
  const keptRepos = new Set(skills.filter((s) => s.kind === 'skill' && s.repoPath && !fresh(s)).map((s) => s.repoPath));
  return skills.filter((s) => fresh(s) && !(s.repoPath && keptRepos.has(s.repoPath)));
}

export const countChanges = (groups: ChangeGroup[]) => groups.reduce((n, g) => n + g.rows.filter((r) => r.counts).length, 0);
/** Changes a plain sync of every part would apply (the sidebar badge). A blocked MCP or hooks plan applies nothing. */
export function pendingCount(diffs: DiffTarget[], targets: Target[], extras: ExtraDiffResult[], plan: MCPPlan | null | undefined, hooks?: HookPlan | null): number {
  const parts = new Set<Part>(['skill', 'agent', 'extra', 'mcp', 'hooks']);
  return countChanges([...resourceGroups(diffs, targets, parts, false).groups, ...extraGroups(extras, false), ...(plan?.blocked ? [] : mcpGroups(plan)), ...(hooks?.blocked ? [] : hooksGroups(hooks))]);
}
export const countEdited = (groups: ChangeGroup[]) => groups.reduce((n, g) => n + g.rows.filter((r) => r.edited).length, 0);

/** Thrown when the MCP plan no longer matches the one on screen. */
export const MCP_CHANGED = 'mcp-changed';
/** Thrown when the hooks plan no longer matches the one on screen. */
export const HOOKS_CHANGED = 'hooks-changed';

/** The plan's changes for one mcp.projects root. */
export const projectChanges = (plan: MCPPlan | null | undefined, root: string) => (plan?.changes ?? []).filter((c) => projectOf([root], c) === root);

// Revisions move on any config write, including the resource sync; only the reviewed changes must hold.
const changeKey = (plan: MCPPlan, root?: string) => (root ? projectChanges(plan, root) : plan.changes).filter((c) => c.action !== 'unchanged').map((c) => JSON.stringify([c.target, c.name, c.action])).sort().join();

const hookKey = (plan: HookPlan, root?: string) => plan.changes.filter((c) => (root ? c.root === root : true) && c.action !== 'unchanged').map((c) => JSON.stringify([c.target, c.path, c.name, c.action])).sort().join();

export interface SyncRun {
  resources: 'skill' | 'agent' | 'both' | null;
  extras: boolean;
  /** The MCP plan the user reviewed, or null to leave MCP alone */
  mcp: MCPPlan | null;
  /** The hooks plan the user reviewed, or null to leave hooks alone */
  hooks?: HookPlan | null;
  force: boolean;
  /** Write one project only: `root` as declared under projects, `path` its folder (the mcp.projects key) */
  project?: { root: string; path: string };
}

/** A target that failed to sync; the rest of the run still went ahead. */
export interface SyncFailure {
  target: string;
  /** 'config': the target's settings are invalid, so it was skipped */
  part: 'skill' | 'agent' | 'extra' | 'config';
  /** The extra's name, for an extras target */
  extra?: string;
  error: string;
  /** A symlink points elsewhere; Force replaces it */
  conflict?: boolean;
}

// OS error texts (Go's syscall messages) and the explanation each gets; read-only first, as it also denies writes.
const KNOWN_ERRORS: [string, string][] = [
  ['read-only file system', 'sync.result.why.readOnly'],
  ['permission denied', 'sync.result.why.permission'],
  ['access is denied', 'sync.result.why.permission'],
  ['not a directory', 'sync.result.why.notDir'],
  ['the directory name is invalid', 'sync.result.why.notDir'],
  ['no such file or directory', 'sync.result.why.missing'],
  ['cannot find the path specified', 'sync.result.why.missing'],
  ['cannot find the file specified', 'sync.result.why.missing'],
];

/** An i18n key explaining a failed target in plain words, or null when the error is not a known case. */
export function failureExplanation(f: SyncFailure): string | null {
  if (f.part === 'config') return 'sync.result.why.config';
  if (f.conflict) return 'sync.result.why.conflict';
  const error = f.error.toLowerCase();
  return KNOWN_ERRORS.find(([text]) => error.includes(text))?.[1] ?? null;
}

/** The sync warnings that are not also reported as a failed target. */
export const otherWarnings = (res: SyncResponse | null | undefined) => {
  const failed = new Set((res?.failed ?? []).map((f) => f.message));
  return (res?.warnings ?? []).filter((w) => !failed.has(w));
};

/**
 * Writes each included part in order. MCP is checked before anything is written and again right before it applies.
 * A failed target does not stop the run: it is returned in `failures`, like `skillshare sync --all` reports it.
 */
export async function runSync(run: SyncRun) {
  const reviewed = run.mcp;
  const recheck = async () => {
    const fresh = await mcpApi.preview();
    const root = run.project?.path;
    const blocked = root ? projectChanges(fresh, root).some((c) => c.action === 'conflict') : fresh.blocked;
    if (blocked || changeKey(fresh, root) !== changeKey(reviewed!, root)) throw new Error(MCP_CHANGED);
    return fresh.revision;
  };
  const reviewedHooks = run.hooks;
  const recheckHooks = async () => {
    const fresh = await hooksApi.preview();
    const root = run.project?.path;
    const blocked = root ? fresh.changes.some((c) => c.root === root && c.action === 'conflict') : fresh.blocked;
    if (blocked || fresh.fingerprint !== reviewedHooks!.fingerprint || hookKey(fresh, root) !== hookKey(reviewedHooks!, root)) throw new Error(HOOKS_CHANGED);
    return fresh.revision;
  };
  if (reviewed) await recheck();
  if (reviewedHooks) await recheckHooks();
  let resources: SyncResponse | undefined;
  if (run.resources) {
    resources = await api.sync({ force: run.force, ...(run.resources !== 'both' && { kind: run.resources }), ...(run.project && { project: run.project.root }) });
  }
  const failures: SyncFailure[] = (resources?.failed ?? []).map(({ target, part, error, conflict }) => ({ target, part, error, conflict }));
  if (run.extras) {
    const extras = await api.syncExtras({ force: run.force });
    for (const e of extras.extras) {
      for (const t of e.targets) {
        const error = t.error || t.errors?.join('; ');
        if (error) failures.push({ target: t.target, part: 'extra', extra: e.name, error });
      }
    }
  }
  if (reviewed) {
    if (run.project) await mcpApi.syncProject(run.project.path, await recheck());
    else await mcpApi.configure({}, await recheck(), true);
  }
  if (reviewedHooks) {
    if (run.project) await hooksApi.syncProject(run.project.path, await recheckHooks());
    else await hooksApi.configure({}, await recheckHooks(), true);
  }
  return { resources, failures };
}
