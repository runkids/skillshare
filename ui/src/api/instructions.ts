import { http } from './http';
import type {
  ConvertMethod,
  InstructionsChange,
  InstructionsWarning,
  ProjectInstructions,
  SharedCopyResult,
  SharedInstructionsFile,
  SharedInstructionsTarget,
  SharedRestorePreview,
  TargetFile,
  TargetFileList,
  TargetInstructions,
  TargetInstructionsSetup,
} from './types/instructions';

export const instructionsApi = {
  getTargetInstructions: (name: string) =>
    http.get<TargetInstructions>(`/targets/${encodeURIComponent(name)}/instructions`),
  putTargetInstructions: (name: string, content: string) =>
    http.put<{ success: boolean }>(`/targets/${encodeURIComponent(name)}/instructions`, {
      content,
    }),
  convertTargetInstructions: (
    name: string,
    body: {
      method: ConvertMethod;
      keep_tool_lines: boolean;
      share_as?: string;
      share_into?: string;
      apply: boolean;
    },
  ) =>
    http.post<{ changes: InstructionsChange[] }>(
      `/targets/${encodeURIComponent(name)}/instructions/convert`,
      body,
    ),
  setTargetInstructionsSetup: (name: string, setup: TargetInstructionsSetup) =>
    http.put<{ success: boolean }>(
      `/targets/${encodeURIComponent(name)}/instructions/setup`,
      setup,
    ),
  removeTargetInstructionsSetup: (name: string) =>
    http.delete<{ success: boolean }>(`/targets/${encodeURIComponent(name)}/instructions/setup`),
  // A target's other files
  listTargetFiles: (name: string) =>
    http.get<TargetFileList>(`/targets/${encodeURIComponent(name)}/files`),
  getTargetFile: (name: string, path: string) =>
    http.get<TargetFile & { content: string }>(
      `/targets/${encodeURIComponent(name)}/files/content?path=${encodeURIComponent(path)}`,
    ),
  putTargetFile: (name: string, path: string, content: string) =>
    http.put<TargetFile & { content: string }>(
      `/targets/${encodeURIComponent(name)}/files/content?path=${encodeURIComponent(path)}`,
      { content },
    ),
  addTargetFile: (name: string, path: string) =>
    http.post<TargetFileList>(`/targets/${encodeURIComponent(name)}/files`, { path }),
  removeTargetFile: (name: string, path: string) =>
    http.delete<TargetFileList>(
      `/targets/${encodeURIComponent(name)}/files?path=${encodeURIComponent(path)}`,
    ),
  listSharedInstructions: () =>
    http.get<{
      files: SharedInstructionsFile[];
      targets: SharedInstructionsTarget[];
      file_links: boolean;
    }>('/instructions'),
  createSharedInstructions: (body: { name: string; content?: string; from_target?: string }) =>
    http.post<{ success: boolean; path: string }>('/instructions', body),
  getSharedInstructionsContent: (name: string) =>
    http.get<{ name: string; path: string; exists: boolean; content: string }>(
      `/instructions/${encodeURIComponent(name)}/content`,
    ),
  /** Saves the shared file; its copy targets are rewritten too (copies). */
  putSharedInstructionsContent: (name: string, content: string) =>
    http.put<{ success: boolean; copies?: SharedCopyResult[] }>(
      `/instructions/${encodeURIComponent(name)}/content`,
      { content },
    ),
  /** Sets exactly which shared files each target uses; [] restores their own files. */
  assignSharedInstructions: (targets: string[], extras: string[]) =>
    http.post<{ success: boolean; errors: string[]; warnings?: InstructionsWarning[] }>(
      '/instructions/assign',
      { targets, extras },
    ),
  restoreSharedInstructions: (name: string, target: string) =>
    http.post<{ success: boolean; errors: string[] }>(
      `/instructions/${encodeURIComponent(name)}/restore`,
      { target },
    ),
  setSharedInstructionsMode: (name: string, target: string, mode: string) =>
    http.put<{ success: boolean; warnings?: InstructionsWarning[] }>(
      `/instructions/${encodeURIComponent(name)}/targets/${encodeURIComponent(target)}/mode`,
      { mode },
    ),
  getSharedRestorePreview: (name: string, target: string) =>
    http.get<SharedRestorePreview>(
      `/instructions/${encodeURIComponent(name)}/restore-preview?target=${encodeURIComponent(target)}`,
    ),
  addInstructionLocation: (name: string, body: { path: string; as?: string; mode?: string }) =>
    http.post<{ success: boolean; warnings?: InstructionsWarning[] }>(
      `/instructions/${encodeURIComponent(name)}/locations`,
      body,
    ),
  setInstructionLocationMode: (name: string, path: string, mode: string) =>
    http.put<{ success: boolean; warnings?: InstructionsWarning[] }>(
      `/instructions/${encodeURIComponent(name)}/locations/mode`,
      { path, mode },
    ),
  removeInstructionLocation: (name: string, path: string) =>
    http.delete<{ success: boolean; warnings?: InstructionsWarning[] }>(
      `/instructions/${encodeURIComponent(name)}/locations?path=${encodeURIComponent(path)}`,
    ),
  getLocationRestorePreview: (name: string, path: string) =>
    http.get<SharedRestorePreview>(
      `/instructions/${encodeURIComponent(name)}/locations/restore-preview?path=${encodeURIComponent(path)}`,
    ),
  /** Settles a modified target, or a location when `on` has its `path`. */
  resolveSharedInstructions: (
    name: string,
    on: { target: string } | { path: string },
    action: 'collect' | 'reapply',
  ) =>
    http.post<{ success: boolean }>(`/instructions/${encodeURIComponent(name)}/resolve`, {
      ...on,
      action,
    }),
  getProjectInstructions: () => http.get<ProjectInstructions>('/instructions/project'),
  putProjectInstructions: (content: string) =>
    http.put<{ success: boolean }>('/instructions/project', { content }),
  addProjectInstructionsShim: (target: string) =>
    http.post<{ success: boolean }>('/instructions/project/shim', { target }),
};
