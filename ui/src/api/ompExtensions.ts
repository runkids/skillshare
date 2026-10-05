import { apiFetch } from './client';

/** Where an extension row comes from: a file in the extensions folder, a settings entry, a plugin, or a Skillshare hook. */
export type OmpExtensionSource = 'native' | 'configured' | 'plugin' | 'hook';
/** What the settings select for the file; whether Oh My Pi loaded it is never known here. */
export type OmpSelection = 'selected' | 'disabled' | 'shadowed' | 'unknown';
export type OmpScope = 'global' | 'project' | 'account';

export interface OmpExtensionRow {
  /** Addresses the row in a change; stable across a refresh of the same file. */
  key: string;
  path: string;
  name: string;
  derivedId: string;
  source: OmpExtensionSource;
  scope: OmpScope;
  selection: OmpSelection;
  /** The settings' answer: null when it cannot be told. Never says whether omp loaded the file. */
  enabled: boolean | null;
  /** A switch is offered only here; `readOnlyReason` says why not, as the server wrote it. */
  selectable: boolean;
  readOnlyReason: string;
  /** `native` is unmanaged by Skillshare; `hooks` only when the hooks ledger proves it. */
  owner: 'native' | 'hooks' | 'plugin' | 'unknown';
  notes: string[];
  /** Set only for a hooks-owned row: the target whose Hooks tab manages it. */
  hooksTarget?: string;
  /** Native plugin identity/root and its declared version, when inspected. */
  pluginName?: string;
  pluginRoot?: string;
  pluginVersion?: string;
}

/** GET /targets/{name}/omp-extensions. `revision` is what a preview checks the settings file against. */
export interface OmpExtensionsView {
  target: string;
  scope: OmpScope;
  root: string;
  settingsPath: string;
  revision: string;
  /** The whole target is listed only; `reasons` say why. */
  readOnly: boolean;
  reasons: string[];
  warnings: string[];
  rows: OmpExtensionRow[];
}

export interface OmpExtensionChange { key: string; enabled: boolean }

/** The preview's `revision` is a plan token; apply sends it back, so only that reviewed plan is written. */
export interface OmpExtensionsPlan {
  revision: string;
  settingsPath: string;
  rows: { key: string; derivedId: string; before: boolean; after: boolean }[];
  warnings: string[];
  backupId?: string;
}

const base = (target: string) => `/targets/${encodeURIComponent(target)}/omp-extensions`;
const post = (target: string, verb: string, changes: OmpExtensionChange[], revision: string) =>
  apiFetch<OmpExtensionsPlan>(`${base(target)}/${verb}`, { method: 'POST', body: JSON.stringify({ changes, revision }) });

export const ompExtensionsApi = {
  get: (target: string) => apiFetch<OmpExtensionsView>(base(target)),
  /** Writes nothing; `revision` is the view's. */
  preview: (target: string, changes: OmpExtensionChange[], revision: string) => post(target, 'preview', changes, revision),
  /** `revision` is the previewed plan's. */
  apply: (target: string, changes: OmpExtensionChange[], revision: string) => post(target, 'apply', changes, revision),
};

/** A target whose Agent is Oh My Pi: omp itself or an omp account. A project's omp has its tab on the project page. */
export const isOmpTarget = (t: { name: string; agent?: string }) => t.name === 'omp' || t.agent === 'omp';
