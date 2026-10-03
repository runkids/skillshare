import { apiFetch } from './client';

/** What a Pi target's settings select for an extension; whether Pi loaded it is never known here. */
export type PiSelection = 'loads' | 'skipped' | 'unknown' | 'none';
export type PiExtensionAction = 'select' | 'exclude' | 'default';

export interface PiExtensionRow {
  path: string;
  file: 'present' | 'missing';
  selection: PiSelection;
  /** default, rule, glob, emptyList, file; for a project also inherited, project or unnamed. */
  origin: string;
  rule?: string;
  globs?: string[];
  editable: boolean;
}

/** One entry of settings.json "packages". `rules` is its extensions list as written, null when it has none. */
export interface PiExtensionPackage {
  index: number;
  source: string;
  identity: string;
  kind: 'npm' | 'git' | 'local' | 'unknown' | '';
  form: 'string' | 'object';
  scope: 'global' | 'project';
  /** Project only: delta, deltaOnly, replaces, projectOnly, or global for a global entry the project keeps. */
  shape?: string;
  install: 'present' | 'missing' | 'unknown';
  problem?: string;
  /** Why rows that can be read have no switch; credentials and reference only for a global package in a project. */
  readOnly?: 'singleFile' | 'otherResources' | 'credentials' | 'reference';
  rules: string[] | null;
  globalRules?: string[];
  /** Names of the entry's other keys; their values never leave the server. */
  otherKeys: string[];
  managedBy?: string;
  rows: PiExtensionRow[];
}

export interface PiExtensionFolder {
  path: string;
  kind: 'folder' | 'settings';
  /** Project view only: whose settings decide these rows. */
  scope?: 'global' | 'project';
  problem?: string;
  rows: { path: string; file: 'present' | 'missing'; selection: PiSelection; provenance: 'native' | 'extras'; extra?: string }[];
}

export interface PiExtensionsView {
  target: string;
  scope: 'global' | 'account' | 'project';
  settingsPath: string;
  globalSettingsPath?: string;
  version: string;
  minVersion: string;
  editable: boolean;
  /** fork, noCli, unsupportedVersion or settings. */
  readOnly?: string;
  problem?: string;
  revision: string;
  packages: PiExtensionPackage[];
  folders: PiExtensionFolder[];
  trust?: { saved: string; default: string };
}

/**
 * One extension of one package, addressed by the settings file its entry is in. In a
 * project, a global package is changed by a new project override; the global file is
 * never written.
 */
export interface PiExtensionChange { scope: 'global' | 'project'; index: number; source: string; path: string; action: PiExtensionAction }

export interface PiExtensionsPlan {
  revision: string;
  settingsPath: string;
  entries: {
    scope?: 'global' | 'project'; index: number; source: string; identity: string; before: string[] | null; after: string[] | null; converted: boolean; keptKeys: string[];
    /** Project only: a new override written with `reference` as its source, or an override left with no rule and removed. */
    created?: boolean; removed?: boolean; reference?: string;
  }[];
  rows: { scope?: 'global' | 'project'; index: number; path: string; before: PiSelection; after: PiSelection; removed: string[]; added: string[] }[];
  backupId?: string;
}

const base = (target: string) => `/targets/${encodeURIComponent(target)}/pi-extensions`;

export const piExtensionsApi = {
  get: (target: string) => apiFetch<PiExtensionsView>(base(target)),
  /** Writes nothing; the plan's revision is what apply checks the file against. */
  preview: (target: string, changes: PiExtensionChange[]) =>
    apiFetch<PiExtensionsPlan>(`${base(target)}/preview`, { method: 'POST', body: JSON.stringify({ changes }) }),
  apply: (target: string, changes: PiExtensionChange[], revision: string) =>
    apiFetch<PiExtensionsPlan>(`${base(target)}/apply`, { method: 'POST', body: JSON.stringify({ changes, revision }) }),
};

/** A target whose Agent is Pi: pi itself or a Pi account. A project's Pi has its tab on the project page. */
export const isPiTarget = (t: { name: string; agent?: string }) => t.name === 'pi' || t.agent === 'pi';
