import type { Skill, SourceLink } from '../../api/client';
import { repoOf, sourceLinkOf } from '../../lib/resourceGrouping';

/* Pure helpers behind the Skills page tree view: folder tree, visible rows, selection. */

export interface TargetSummary {
  display: string; // "claude" | "claude, cursor" | "4 targets"
  targets: string[]; // sorted union
  isUniform: boolean; // every skill has the same target set
}

export interface FolderNode {
  name: string;
  path: string;
  children: Map<string, FolderNode>;
  skills: Skill[];
  count: number; // skills in this folder and every subfolder
  link?: SourceLink;
  repo?: boolean; // tracked repo root: _team, or org/_team when installed with --into
}

/**
 * One visible line of the tree. A folder row can stand for a chain of folders that
 * each hold nothing but the next one ("plugins / caveman / skills"): `names` lists the
 * chain and `node` is its deepest folder, whose path is the row's id and collapse key.
 */
export type TreeRow =
  | {
      type: 'folder';
      id: string;
      node: FolderNode;
      names: string[];
      depth: number;
      collapsed: boolean;
      repo: boolean;
    }
  | { type: 'item'; id: string; skill: Skill; depth: number };

const ALL_TARGETS: TargetSummary = { display: '', targets: [], isUniform: true };

export const folderId = (path: string) => `f:${path}`;
export const skillId = (flatName: string) => `s:${flatName}`;

/** Normalize skill targets: ["*"] or empty/null → [] (meaning All). */
export function normalizeTargets(targets?: string[] | null): string[] {
  if (!targets || targets.length === 0 || targets.includes('*')) return [];
  return targets;
}

export function summarize(skills: Skill[]): TargetSummary {
  const sets = skills.map((s) => [...normalizeTargets(s.targets)].sort());
  if (sets.length === 0) return ALL_TARGETS;
  const first = sets[0];
  const isUniform = sets.every(
    (x) => x.length === first.length && x.every((v, i) => v === first[i]),
  );
  const union = [...new Set(sets.flat())].sort();
  const shown = isUniform ? first : union;
  return {
    display: shown.length > 3 ? `${shown.length} targets` : shown.join(', '),
    targets: shown,
    isUniform,
  };
}

/** A tracked repo root, as marked by buildTree from its skills' repo paths. */
export function isRepoRoot(node: FolderNode): boolean {
  return !!node.repo;
}

export function buildTree(skills: Skill[], links: SourceLink[] = []): FolderNode {
  const root: FolderNode = { name: '', path: '', children: new Map(), skills: [], count: 0 };
  for (const skill of skills) {
    const link = sourceLinkOf(skill);
    const repo = repoOf(skill);
    const slash = skill.relPath.lastIndexOf('/');
    let node = root;
    // A repo's root SKILL.md sits in the repo's own folder, so the repo gets its row.
    const parent =
      skill.relPath === repo ? repo : slash > 0 ? skill.relPath.slice(0, slash) : link?.name;
    if (parent) {
      for (const seg of parent.split('/')) {
        if (!node.children.has(seg)) {
          const path = node.path ? `${node.path}/${seg}` : seg;
          node.children.set(seg, { name: seg, path, children: new Map(), skills: [], count: 0 });
        }
        node = node.children.get(seg)!;
        if (node.path === link?.name) node.link = link;
        if (node.path === repo) node.repo = true;
      }
    }
    node.skills.push(skill);
  }
  for (const link of links) {
    const node = root.children.get(link.name);
    if (node) node.link = link;
    else
      root.children.set(link.name, {
        name: link.name,
        path: link.name,
        children: new Map(),
        skills: [],
        count: 0,
        link,
      });
  }
  const finish = (node: FolderNode): number => {
    node.count = node.skills.length;
    for (const child of node.children.values()) node.count += finish(child);
    return node.count;
  };
  finish(root);
  return root;
}

const sortedChildren = (node: FolderNode) =>
  [...node.children.values()].sort((a, b) => a.name.localeCompare(b.name));

export function flattenTree(
  root: FolderNode,
  collapsed: ReadonlySet<string>,
  expandAll: boolean,
): TreeRow[] {
  const rows: TreeRow[] = [];
  const walk = (node: FolderNode, depth: number) => {
    for (const child of sortedChildren(node)) {
      const repo = isRepoRoot(child);
      let deepest = child;
      const names = [child.name];
      // Repo and link roots keep their own row so their tags and actions stay visible.
      while (!repo && !child.link && deepest.skills.length === 0 && deepest.children.size === 1) {
        const next = deepest.children.values().next().value!;
        if (isRepoRoot(next)) break; // an --into repo (org/_team) keeps its own row too
        deepest = next;
        names.push(deepest.name);
      }
      const isCollapsed = !expandAll && collapsed.has(deepest.path);
      rows.push({
        type: 'folder',
        id: folderId(deepest.path),
        node: deepest,
        names,
        depth,
        collapsed: isCollapsed,
        repo,
      });
      if (!isCollapsed) walk(deepest, depth + 1);
    }
    for (const skill of node.skills)
      rows.push({ type: 'item', id: skillId(skill.flatName), skill, depth });
  };
  walk(root, 0);
  return rows;
}

export function folderPaths(node: FolderNode): string[] {
  return [...node.children.values()].flatMap((c) => [c.path, ...folderPaths(c)]);
}

/** Every skill in a folder and its subfolders, in tree order. */
export function skillsUnder(node: FolderNode): Skill[] {
  return [...sortedChildren(node).flatMap(skillsUnder), ...node.skills];
}

export function findFolder(root: FolderNode, path: string): FolderNode | undefined {
  let node: FolderNode | undefined = root;
  for (const seg of path.split('/')) node = node?.children.get(seg);
  return node;
}

/**
 * Tree lines for each row: `through` lists the ancestor levels whose line runs on past the row,
 * and `last` says the row is the final child of its folder, so its elbow ends the line.
 */
export function treeLines(rows: TreeRow[]): { through: number[]; last: boolean }[] {
  const below: boolean[] = []; // below[d]: a later row at depth d comes before anything shallower
  const out: { through: number[]; last: boolean }[] = [];
  for (let i = rows.length - 1; i >= 0; i--) {
    const d = rows[i].depth;
    out[i] = {
      through: Array.from({ length: Math.max(d - 1, 0) }, (_, g) => g).filter((g) => below[g + 1]),
      last: !below[d],
    };
    below[d] = true;
    below.length = d + 1;
  }
  return out;
}

/** Ids of the visible rows from `anchor` to `id`, both included. Without a visible anchor, just `id`. */
export function rangeIds(rows: TreeRow[], anchor: string | null, id: string): string[] {
  const a = anchor ? rows.findIndex((r) => r.id === anchor) : -1;
  const b = rows.findIndex((r) => r.id === id);
  if (a < 0 || b < 0) return [id];
  return rows.slice(Math.min(a, b), Math.max(a, b) + 1).map((r) => r.id);
}

/** The skills a selection stands for: a folder means everything under it. No duplicates, first-seen order. */
export function selectedSkills(
  root: FolderNode,
  selection: ReadonlySet<string>,
  byName: ReadonlyMap<string, Skill>,
): Skill[] {
  const out = new Map<string, Skill>();
  for (const id of selection) {
    const found = id.startsWith('f:') ? findFolder(root, id.slice(2)) : undefined;
    const list = found
      ? skillsUnder(found)
      : id.startsWith('s:')
        ? [byName.get(id.slice(2))].filter((s): s is Skill => !!s)
        : [];
    for (const s of list) out.set(s.flatName, s);
  }
  return [...out.values()];
}

/** Whether a row sits inside a selected folder, which draws it with the softer selected tint. */
export function insideSelection(row: TreeRow, selection: ReadonlySet<string>): boolean {
  const path = row.type === 'folder' ? row.node.path : row.skill.relPath;
  for (const id of selection) {
    if (id.startsWith('f:') && path.startsWith(`${id.slice(2)}/`)) return true;
  }
  return false;
}
