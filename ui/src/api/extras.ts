import { apiFetch } from './http';
import type { ExtensionInfo, Extra, ExtraDiffResult, ExtraEditResult, ExtraFilterEntry, ExtraRemoveResult, ExtrasSyncResult } from './types/extras';

export const extrasApi = {
  listExtras: () => apiFetch<{ extras: Extra[] }>('/extras'),
  diffExtras: (name?: string) =>
    apiFetch<{ extras: ExtraDiffResult[] }>(`/extras/diff${name ? '?name=' + encodeURIComponent(name) : ''}`),
  createExtra: (data: {
    name: string;
    source?: string;
    folder?: string; // a folder in the shared extras folder; the server stores it as the source
    file?: string; // single-file extra: the file in the source folder
    targets: Array<{ path: string; mode: string; flatten?: boolean; extension?: string; as?: string }>;
  }) =>
    apiFetch<{ success: boolean }>('/extras', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  syncExtras: (opts?: { name?: string; dry_run?: boolean; force?: boolean }) =>
    apiFetch<{ extras: ExtrasSyncResult[] }>('/extras/sync', {
      method: 'POST',
      body: JSON.stringify(opts ?? {}),
    }),
  deleteExtra: (name: string) =>
    apiFetch<ExtraRemoveResult>(`/extras/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  listExtraExtensions: () => apiFetch<{ extensions: string[] }>('/extras/extensions'),
  // Extensions management (Config page)
  listExtensions: () => apiFetch<{ extensions: ExtensionInfo[] }>('/extensions'),
  installExtension: (name: string) =>
    apiFetch<{ success: boolean; name: string }>('/extensions/install', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),
  openExtensionsDir: () =>
    apiFetch<{ path: string }>('/extensions/open', { method: 'POST' }),
  removeExtension: (name: string) =>
    apiFetch<{ success: boolean; name: string }>(`/extensions/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }),
  // extension: undefined = leave unchanged; '' = clear; name = set (forces copy mode)
  setExtraMode: (name: string, target: string, mode: string, flatten?: boolean, extension?: string) =>
    apiFetch<{ success: boolean }>(`/extras/${encodeURIComponent(name)}/mode`, {
      method: 'PATCH',
      body: JSON.stringify({
        target,
        mode,
        ...(flatten !== undefined && { flatten }),
        ...(extension !== undefined && { extension }),
      }),
    }),
  addExtraTarget: (name: string, target: { path: string; mode?: string; flatten?: boolean; as?: string }) =>
    apiFetch<{ success: boolean }>(`/extras/${encodeURIComponent(name)}/targets`, {
      method: 'POST',
      body: JSON.stringify(target),
    }),
  previewExtraFilter: (name: string, include: string[], exclude: string[]) =>
    apiFetch<{ files: ExtraFilterEntry[]; unmatched: string[] | null }>(`/extras/${encodeURIComponent(name)}/preview`, {
      method: 'POST',
      body: JSON.stringify({ include, exclude }),
    }),
  // A new path or file name also clears the old place and syncs.
  editExtraTarget: (name: string, path: string, target: { path: string; mode: string; flatten: boolean; extension: string; as: string; include: string[]; exclude: string[] }) =>
    apiFetch<ExtraEditResult>(`/extras/${encodeURIComponent(name)}/targets`, {
      method: 'PUT',
      body: JSON.stringify({ path, target }),
    }),
  // source: omitted = unchanged, '' = the default folder; a new source relinks every target.
  editExtra: (name: string, data: { name?: string; source?: string }) =>
    apiFetch<ExtraEditResult>(`/extras/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),
  removeExtraTarget: (name: string, path: string) =>
    apiFetch<{ success: boolean }>(`/extras/${encodeURIComponent(name)}/targets`, {
      method: 'DELETE',
      body: JSON.stringify({ path }),
    }),
};
