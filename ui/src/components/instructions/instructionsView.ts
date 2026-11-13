import { ApiError } from '../../api/client';
import type { ExtrasSyncResult, InstructionsWarning } from '../../api/client';
import type { useT } from '../../i18n';
import type { InstructionLocation, InstructionsAssignment, InstructionsEntry, SharedCopyResult, SharedInstructionsFile, SharedInstructionsTarget } from '../../api/client';
import { fileName, shortenHome } from '../../lib/paths';
import type { LineDecor } from '../CodeEditor';
import { buildSyncToast, sumAll } from '../../lib/extrasSyncToast';

export function sharedSyncFeedback(extras: ExtrasSyncResult[], success: string, t: ReturnType<typeof useT>): [string, 'success' | 'warning'] {
  const failed = extras.flatMap((e) => e.targets).find((r) => r.error);
  if (failed) throw new Error(failed.error);
  const totals = sumAll(extras);
  return totals.warnings.length > 0
    ? [buildSyncToast(t('extras.title'), t('extras.title'), totals, false, t), 'warning']
    : [success, 'success'];
}

/** Why a new shared file name would be refused, checked as the user types. Same rule as the server. */
export function sharedNameProblem(name: string, taken: string[]): 'invalid' | 'taken' | null {
  const n = name.trim();
  if (!n) return null;
  if (!/^[A-Za-z0-9][A-Za-z0-9_-]*$/.test(n)) return 'invalid';
  // Names are folder names; macOS and Windows ignore case.
  return takenName(n, taken) ? 'taken' : null;
}

/** The existing name that a new name collides with, ignoring case. */
export function takenName(name: string, taken: string[]): string | undefined {
  const n = name.trim().toLowerCase();
  return taken.find((x) => x.toLowerCase() === n);
}

/** Whether an extra is a shared AGENTS.md, shown on the AGENTS.md tab. Every other extra (folders and other single files) is on the first tab. */
export const isAgentsExtra = (extra: { file?: string }) => extra.file === 'AGENTS.md';

/** Whether name is an extra on the first tab (not a shared AGENTS.md), so a name clash should say so. */
export const isOtherExtra = (name: string, extras: { name: string; file?: string }[]) =>
  extras.some((e) => e.name === name && !isAgentsExtra(e));

/** A free name for a new shared file made from a target: the target's name, else <name>-2, <name>-3, … */
export function defaultShareName(target: string, taken: string[]): string {
  const base = sharedNameProblem(target, []) === 'invalid' ? target.replace(/[^A-Za-z0-9_-]+/g, '-').replace(/^[^A-Za-z0-9]+/, '') || 'shared' : target;
  let name = base;
  for (let n = 2; takenName(name, taken); n++) name = `${base}-${n}`;
  return name;
}

/** Why a target's instruction file path would be refused, checked as the user types. Same rule as the server. */
export function setupPathProblem(path: string, project: boolean): 'directory' | 'absolute' | 'relative' | null {
  const p = path.trim();
  if (!p) return null;
  if (p.endsWith('/') || p.endsWith('\\')) return 'directory';
  const rooted = p.startsWith('/') || /^[A-Za-z]:[\\/]/.test(p);
  if (project) return rooted || p.startsWith('~') ? 'absolute' : null;
  return rooted || p.startsWith('~/') ? null : 'relative';
}

/** A target's current file written as the location form takes it: ~/… in global mode, relative to projectRoot in a project. */
export function setupPathOf(path: string, project: boolean, projectRoot = ''): string {
  if (project) {
    const root = projectRoot.replace(/\\/g, '/');
    const absolute = path.replace(/\\/g, '/');
    return root && absolute.startsWith(`${root}/`) ? absolute.slice(root.length + 1) : path;
  }
  const short = shortenHome(path);
  return short.startsWith('~\\') ? `~/${short.slice(2).replace(/\\/g, '/')}` : short;
}

/** What saving a shared file did to its copies: targets updated, and problems to show, each named by target. */
export function saveCopiesSummary(copies: SharedCopyResult[]): { updated: string[]; warnings: string[]; errors: string[] } {
  return {
    updated: copies.filter((c) => !c.error).map((c) => c.target),
    warnings: copies.flatMap((c) => (c.warnings ?? []).map((w) => `${c.target}: ${w}`)),
    errors: copies.filter((c) => c.error).map((c) => `${c.target}: ${c.error}`),
  };
}

/** The read-order entries shown as a chain: files the target reads in turn, without a rules folder that holds no files. */
export const readChain = (entries: InstructionsEntry[]) =>
  entries.filter((e) => e.kind !== 'unread' && !(e.kind === 'rules' && !e.count));

/** An @path line: tool-specific import syntax, as the server's ImportLines sees it. */
export const isImportLine = (text: string) => /^\s*@\S+\s*$/.test(text);

/** 1-based first and last lines of the managed import block (markers included), as the server's ManagedImportLines sees it. */
export function managedBlock(lines: string[]): [number, number] | null {
  const begin = lines.findIndex((l) => l.trim() === '<!-- skillshare:instructions:begin -->');
  if (begin === -1) return null;
  const end = lines.findIndex((l, i) => i > begin && l.trim() === '<!-- skillshare:instructions:end -->');
  return end === -1 ? null : [begin + 1, end + 1];
}

/** The shared file an import line of the managed block points at: its folder under extras. */
export function sharedOfImport(line: string, names: string[]): string | undefined {
  const parts = line.trim().slice(1).split(/[\\/]/);
  return names.find((n) => n === parts[parts.length - 2]);
}

/**
 * An instruction file split for preview: the managed import block becomes the
 * list of what it imports (shared file names where known), and HTML comments,
 * which Markdown would show as text, are dropped.
 */
export function previewParts(content: string, names: string[]): { before: string; imports: string[]; after: string } {
  const strip = (s: string) => s.replace(/<!--[\s\S]*?-->/g, '');
  const lines = content.split('\n');
  const blk = managedBlock(lines);
  if (!blk) return { before: strip(content), imports: [], after: '' };
  const imports = lines.slice(blk[0], blk[1] - 1).filter(isImportLine).map((l) => sharedOfImport(l, names) ?? l.trim().slice(1));
  return { before: strip(lines.slice(0, blk[0] - 1).join('\n')), imports, after: strip(lines.slice(blk[1]).join('\n')) };
}

/**
 * Line styling for an instruction file: the managed import block and every
 * @import line are tinted; each @import line gets note(line, inBlock) at its end.
 */
export function importDecor(lines: string[], note: (line: string, inBlock: boolean) => string): LineDecor[] {
  const imports = new Set(importLines(lines.join('\n')));
  const blk = managedBlock(lines);
  return lines.map((line, i) => {
    const inBlock = blk !== null && i + 1 >= blk[0] && i + 1 <= blk[1];
    if (imports.has(i + 1)) return { block: true, note: note(line, inBlock) };
    return inBlock ? { block: true } : null;
  });
}

/** 1-based numbers of the @import lines outside code fences. */
export function importLines(content: string): number[] {
  const out: number[] = [];
  let fence = false;
  content.split('\n').forEach((line, i) => {
    if (line.trim().startsWith('```')) fence = !fence;
    else if (!fence && isImportLine(line)) out.push(i + 1);
  });
  return out;
}

/** "3", "3–4" or "3, 7–8": line numbers folded into ranges. */
export function lineRanges(lines: number[]): string {
  const parts: string[] = [];
  for (let i = 0; i < lines.length; i++) {
    let j = i;
    while (j + 1 < lines.length && lines[j + 1] === lines[j] + 1) j++;
    parts.push(j > i ? `${lines[i]}–${lines[j]}` : `${lines[i]}`);
    i = j;
  }
  return parts.join(', ');
}

export type DiffLine = { kind: 'add' | 'del' | 'same'; text: string };

/** A line diff (LCS) of two small files, for change previews. */
export function lineDiff(before: string, after: string): DiffLine[] {
  const a = before ? before.replace(/\n$/, '').split('\n') : [];
  const b = after ? after.replace(/\n$/, '').split('\n') : [];
  const lcs = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
    }
  }
  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length || j < b.length) {
    if (i < a.length && j < b.length && a[i] === b[j]) {
      out.push({ kind: 'same', text: a[i] });
      i++;
      j++;
    } else if (j < b.length && (i >= a.length || lcs[i][j + 1] >= lcs[i + 1][j])) {
      out.push({ kind: 'add', text: b[j++] });
    } else {
      out.push({ kind: 'del', text: a[i++] });
    }
  }
  return out;
}

/** Tone of an assignment's status for .ss-st. */
export function statusTone(status: string): string {
  if (status === 'synced') return 'ok';
  if (status === 'no source') return 'bad';
  return 'warn';
}

/** The status that most needs attention among a target's shared files. */
export function worstStatus(assigned: InstructionsAssignment[]): string | null {
  const order = ['no source', 'modified', 'drift', 'not synced', 'synced'];
  let worst: string | null = null;
  for (const a of assigned) {
    if (worst === null || order.indexOf(a.status) < order.indexOf(worst)) worst = a.status;
  }
  return worst;
}

export const formatSize = (bytes: number) => (bytes < 1024 ? `${bytes} B` : `${(bytes / 1024).toFixed(1)} KB`);

/** Shared files over the limit of a target that cuts its global file short. */
export function overLimit(target: SharedInstructionsTarget, files: SharedInstructionsFile[]): SharedInstructionsFile[] {
  if (!target.max_chars) return [];
  return files.filter((f) => f.chars > target.max_chars! && target.assigned.some((a) => a.name === f.name));
}

/** Names of the shared files a target uses. */
export const usesOf = (target: SharedInstructionsTarget) => target.assigned.map((a) => a.name);

/** Targets connected to a shared file, leaving out ones that only read another target's file. */
export const connectedTo = (targets: SharedInstructionsTarget[], name: string) =>
  targets.filter((tg) => !tg.same_as && usesOf(tg).includes(name));

/** A mode that keeps a managed part of the target file (an @import line or a content block) rather than replacing the whole file; several can share one file. */
export const managedMode = (mode: string) => mode === 'import' || mode === 'prepend' || mode === 'append';

/** Whether the target's file already holds a shared file in a managed part (an @import line or a block), so another one can join it. */
export const holdsManaged = (target: SharedInstructionsTarget) => target.assigned.some((a) => managedMode(a.mode));

/** What a target should use after connecting it: a target holding managed files adds one more, a link or copy target holds one. */
export const connectExtras = (target: SharedInstructionsTarget, name: string) =>
  (target.import || holdsManaged(target) ? [...usesOf(target).filter((n) => n !== name), name] : [name]);

/** Connected targets a sync would fix: the file or its link is gone, or points elsewhere. */
export const needsSync = (targets: SharedInstructionsTarget[], name: string) =>
  connectedTo(targets, name).filter((tg) => ['not synced', 'drift'].includes(tg.assigned.find((a) => a.name === name)!.status));

export type ConnectStep = { target: string; extras: string[]; note: 'import' | 'block' | 'link' | 'tooLong' | 'held'; other?: string; max?: number };

/**
 * One step per target that would be connected to file, with what changes for it.
 * A target another shared file holds (its file links to or copies that file,
 * or it is assigned in link or copy mode) is skipped (held): connecting it
 * would change that file or silently take the target away from it.
 */
export function connectPlan(targets: SharedInstructionsTarget[], file: SharedInstructionsFile): ConnectStep[] {
  return targets.filter((tg) => !tg.same_as && !usesOf(tg).includes(file.name)).map((tg) => {
    const step = { target: tg.name, extras: connectExtras(tg, file.name) };
    if (tg.linked_shared && tg.linked_shared !== file.name) return { ...step, note: 'held', other: tg.linked_shared };
    const holder = tg.assigned.find((a) => !managedMode(a.mode));
    if (holder) return { ...step, note: 'held', other: holder.name };
    if (tg.max_chars && file.chars > tg.max_chars) return { ...step, note: 'tooLong', max: tg.max_chars };
    return { ...step, note: tg.import ? 'import' : holdsManaged(tg) ? 'block' : 'link' };
  });
}

export type RestoreStep = { target: string; note: 'import' | 'importKeep' | 'block' | 'blockKeep' | 'link' | 'modified'; others?: string[] };

/** One step per target that would stop using name, with what happens to its file. */
export function restorePlan(targets: SharedInstructionsTarget[], name: string): RestoreStep[] {
  return connectedTo(targets, name).map((tg) => {
    const a = tg.assigned.find((x) => x.name === name)!;
    const others = usesOf(tg).filter((n) => n !== name);
    if (a.status === 'modified') return { target: tg.name, note: 'modified' };
    if (a.mode === 'import') return others.length ? { target: tg.name, note: 'importKeep', others } : { target: tg.name, note: 'import' };
    if (managedMode(a.mode)) return others.length ? { target: tg.name, note: 'blockKeep', others } : { target: tg.name, note: 'block' };
    return { target: tg.name, note: 'link' };
  });
}

export type SharedMode = 'import' | 'prepend' | 'append' | 'symlink' | 'copy';
export type ModeOption = { mode: SharedMode; isDefault: boolean; blocked?: 'fileLinks' | 'several' | 'noImport' };

/** The mode a newly connected target gets, as the server picks it: import where the tool reads @import lines; a block when its file already holds a shared file in one; else a link, or a copy when file links are unavailable (Windows without Developer Mode). */
export const defaultMode = (target: SharedInstructionsTarget, fileLinks: boolean): SharedMode =>
  (target.import ? 'import' : holdsManaged(target) ? 'prepend' : fileLinks ? 'symlink' : 'copy');

/** The picker's value for an assignment's mode: a single file written with merge is linked, as symlink does. */
export const pickedMode = (mode: string) => (mode === 'merge' ? 'symlink' : mode);

/** The modes a target can get a shared file with, and why one cannot be picked: the two that keep the tool's own file first, then the two that replace it, which hold one shared file only. */
export function modeOptions(target: SharedInstructionsTarget, fileLinks: boolean): ModeOption[] {
  const modes: SharedMode[] = ['import', 'prepend', 'append', 'symlink', 'copy'];
  const def = defaultMode(target, fileLinks);
  return modes.map((mode) => {
    const blocked = mode === 'import' && !target.import ? 'noImport'
      : !managedMode(mode) && target.assigned.length > 1 ? 'several'
        : mode === 'symlink' && !fileLinks ? 'fileLinks' : undefined;
    return blocked ? { mode, isDefault: mode === def, blocked } : { mode, isDefault: mode === def };
  });
}

export type RowHint =
  | { kind: 'sameAs'; name: string }
  | { kind: 'folderLink' }
  | { kind: 'directory' }
  | { kind: 'notSynced' | 'drift'; mode: string }
  | { kind: 'noSource' }
  | { kind: 'tooLong'; max: number }
  | { kind: 'usesOther'; name: string }
  | { kind: 'alsoUses'; names: string[] };

/** The one line under a target row that says what matters most about it for file. */
export function rowHint(target: SharedInstructionsTarget, file: SharedInstructionsFile): RowHint | null {
  if (target.same_as) return { kind: 'sameAs', name: target.same_as };
  const a = target.assigned.find((x) => x.name === file.name);
  if (a?.reason === 'folder_link') return { kind: 'folderLink' };
  if (a?.reason === 'directory') return { kind: 'directory' };
  if (a?.status === 'not synced' || a?.status === 'drift') return { kind: a.status === 'drift' ? 'drift' : 'notSynced', mode: a.mode };
  if (a?.status === 'no source') return { kind: 'noSource' };
  if (target.max_chars && file.chars > target.max_chars) return { kind: 'tooLong', max: target.max_chars };
  const others = usesOf(target).filter((n) => n !== file.name);
  if (!others.length) return null;
  // A managed part joins the others; a link or copy would take the other file's place.
  const joins = target.import || (a ? managedMode(a.mode) : holdsManaged(target));
  return joins ? { kind: 'alsoUses', names: others } : { kind: 'usesOther', name: others[0] };
}

/** A mode that writes the shared file into the target as a content block, so Collect and Reapply touch only that block. */
export const blockMode = (mode: string) => mode === 'prepend' || mode === 'append';

/** A managed block in a target file: the source it was written from and its 1-based line range, markers included. */
export interface ManagedBlock { src: string; start: number; end: number }

// ponytail: markers inside code fences are taken as real; the server's parser decides what counts, this only tints lines.
/** The managed blocks in a target file's content. */
export function managedBlocks(content: string): ManagedBlock[] {
  const out: ManagedBlock[] = [];
  let open: ManagedBlock | null = null;
  content.split('\n').forEach((raw, i) => {
    const line = raw.trim();
    const m = /^<!-- skillshare:extra src="([^"]+)" sha256=[0-9a-f]{16} -->$/.exec(line);
    if (m && !open) open = { src: m[1], start: i + 1, end: i + 1 };
    else if (line === '<!-- /skillshare:extra -->' && open) { out.push({ ...open, end: i + 1 }); open = null; }
  });
  return out;
}

/** The shared file a block was written from, by the extras folder named in its source path. */
export const blockOwner = (block: ManagedBlock, names: string[]) => names.find((n) => block.src.split('/').slice(0, -1).at(-1) === n) ?? null;

/** Other locations a sync would fix: the file or its link is gone, or points elsewhere. */
export const staleLocations = (locations: InstructionLocation[]) => locations.filter((l) => ['not synced', 'drift'].includes(l.status));

/** The modes a location can get: a folder has no tool deciding for it, so symlink is the default where file links work. */
export function locationModeOptions(fileLinks: boolean): ModeOption[] {
  const def: SharedMode = fileLinks ? 'symlink' : 'copy';
  return (['symlink', 'copy', 'import', 'prepend', 'append'] as const).map((mode) => (mode === 'symlink' && !fileLinks
    ? { mode, isDefault: false, blocked: 'fileLinks' as const } : { mode, isDefault: mode === def }));
}

/** The file a location writes: the folder as typed, then the custom name or the shared file's own. */
export function locationFile(folder: string, as: string, file: string): string {
  const dir = folder.trim().replace(/[\\/]+$/, '');
  const sep = dir.includes('\\') && !dir.includes('/') ? '\\' : '/';
  return `${dir}${sep}${as.trim() || file}`;
}

/** The file a project location writes, relative to the project root: ./docs/ai/AGENTS.md, or ./AGENTS.md for the root. */
export function projectLocationFile(folder: string, as: string, file: string): string {
  const dir = folder.trim().replace(/\\/g, '/').replace(/^(\.\/)+/, '').replace(/\/+$/, '');
  const name = as.trim() || file;
  if (dir === '' || dir === '.') return `./${name}`;
  // A path the server will refuse (../x, /x, ~/x) is shown as typed, not as ./../x.
  return /^(\.\.|\/|~)/.test(dir) ? `${dir}/${name}` : `./${dir}/${name}`;
}

/** How a location row names its file: relative to the project root in a project, else by its full path. */
export const locationLabel = (l: InstructionLocation, project: boolean) =>
  (project ? projectLocationFile(l.path, '', fileName(l.file)) : shortenHome(l.file));

/** A project's shared file named from the project root (.skillshare/extras/…), else by its full path. */
export function projectSourcePath(path: string): string {
  const i = path.search(/[\\/]\.skillshare[\\/]/);
  return i >= 0 ? path.slice(i + 1).replace(/\\/g, '/') : shortenHome(path);
}

export type LocationHint = 'folderLink' | 'directory' | 'noSource' | 'notSynced' | 'drift' | 'driftImport' | 'import';

/** The one line under a location row: a problem first, else what import leaves in the file. */
export function locationHint(l: InstructionLocation): LocationHint | null {
  if (l.reason === 'folder_link') return 'folderLink';
  if (l.reason === 'directory') return 'directory';
  if (l.status === 'no source') return 'noSource';
  // modified: the row shows a note with collect and reapply instead.
  if (l.status === 'modified') return null;
  if (l.status === 'not synced') return 'notSynced';
  if (l.status === 'drift') return l.mode === 'import' ? 'driftImport' : 'drift';
  return l.mode === 'import' ? 'import' : null;
}

/** Line count of a file as an editor shows it: a trailing newline does not start a new line. */
export const lineCount = (content: string) => (content ? content.replace(/\n$/, '').split('\n').length : 0);

/** Translate backend messages while keeping unknown codes readable. */
export function instructionsWarningMessage(warning: InstructionsWarning, t: ReturnType<typeof useT>): string {
  const params = Object.fromEntries(Object.entries(warning.params).map(([key, value]) => [key, shortenHome(value)]));
  return t(`instructions.warning.${warning.code}`, params, warning.message)
    .replace(/\/(?:Users|home)\/[^/\s]+|[A-Z]:\\Users\\[^\\\s]+/gi, shortenHome);
}

export function instructionsErrorMessage(error: unknown, t: ReturnType<typeof useT>): string {
  let message = (error as Error).message;
  if (error instanceof ApiError && error.code?.startsWith('instructions_')) {
    const params = Object.fromEntries(Object.entries(error.params ?? {}).map(([key, value]) => [key, typeof value === 'string' ? shortenHome(value) : String(value)]));
    message = t(`instructions.error.${error.code}`, params, message);
  }
  return message.replace(/\/(?:Users|home)\/[^/\s]+|[A-Z]:\\Users\\[^\\\s]+/gi, shortenHome);
}
