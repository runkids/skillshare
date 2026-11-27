import { http, type SuccessResponse } from './http';
import type {
  CreateExtraRequest,
  EditExtraTarget,
  ExtensionActionResult,
  ExtensionInfo,
  Extra,
  ExtraDiffResult,
  ExtraEditResult,
  ExtraFilterEntry,
  ExtrasSyncResult,
} from './types/extras';

const extraPath = (name: string) => `/extras/${encodeURIComponent(name)}`;

export const extrasApi = {
  listExtras: () => http.get<{ extras: Extra[] }>('/extras'),
  diffExtras: (name?: string) =>
    http.get<{ extras: ExtraDiffResult[] }>(
      `/extras/diff${name ? '?name=' + encodeURIComponent(name) : ''}`,
    ),
  createExtra: (input: CreateExtraRequest) => http.post<SuccessResponse>('/extras', input),
  syncExtras: (options: { name?: string; dry_run?: boolean; force?: boolean } = {}) =>
    http.post<{ extras: ExtrasSyncResult[] }>('/extras/sync', options),
  deleteExtra: (name: string) => http.delete<SuccessResponse>(extraPath(name)),

  // undefined leaves the extension unchanged; '' clears it; a name forces copy mode.
  setExtraMode: (
    name: string,
    target: string,
    mode: string,
    flatten?: boolean,
    extension?: string,
  ) => http.patch<SuccessResponse>(`${extraPath(name)}/mode`, { target, mode, flatten, extension }),
  addExtraTarget: (
    name: string,
    target: { path: string; mode?: string; flatten?: boolean; as?: string },
  ) => http.post<SuccessResponse>(`${extraPath(name)}/targets`, target),
  previewExtraFilter: (name: string, include: string[], exclude: string[]) =>
    http.post<{ files: ExtraFilterEntry[]; unmatched: string[] | null }>(
      `${extraPath(name)}/preview`,
      { include, exclude },
    ),
  // A new path or file name also clears the old place and syncs.
  editExtraTarget: (name: string, path: string, target: EditExtraTarget) =>
    http.put<ExtraEditResult>(`${extraPath(name)}/targets`, { path, target }),
  // source: omitted = unchanged, '' = the default folder; a new source relinks every target.
  editExtra: (name: string, data: { name?: string; source?: string }) =>
    http.patch<ExtraEditResult>(extraPath(name), data),
  removeExtraTarget: (name: string, path: string) =>
    http.delete<SuccessResponse>(`${extraPath(name)}/targets`, { path }),

  listExtraExtensions: () => http.get<{ extensions: string[] }>('/extras/extensions'),
  listExtensions: () => http.get<{ extensions: ExtensionInfo[] }>('/extensions'),
  installExtension: (name: string) =>
    http.post<ExtensionActionResult>('/extensions/install', { name }),
  openExtensionsDir: () => http.post<{ path: string }>('/extensions/open'),
  /** The extensions folder, listed and read in the page, where no file manager can open it. */
  extensionFiles: () => http.get<{ files: string[] }>('/extensions/files'),
  extensionFile: (path: string) =>
    http.get<{ content: string }>(
      `/extensions/files/${path.split('/').map(encodeURIComponent).join('/')}`,
    ),
  removeExtension: (name: string) =>
    http.delete<ExtensionActionResult>(`/extensions/${encodeURIComponent(name)}`),
};
