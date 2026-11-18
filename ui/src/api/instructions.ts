import { apiFetch } from './http';
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
    apiFetch<TargetInstructions>(`/targets/${encodeURIComponent(name)}/instructions`),
  putTargetInstructions: (name: string, content: string) =>
    apiFetch<{ success: boolean }>(`/targets/${encodeURIComponent(name)}/instructions`, {
      method: 'PUT',
      body: JSON.stringify({ content }),
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
    apiFetch<{ changes: InstructionsChange[] }>(
      `/targets/${encodeURIComponent(name)}/instructions/convert`,
      {
        method: 'POST',
        body: JSON.stringify(body),
      },
    ),
  setTargetInstructionsSetup: (name: string, setup: TargetInstructionsSetup) =>
    apiFetch<{ success: boolean }>(`/targets/${encodeURIComponent(name)}/instructions/setup`, {
      method: 'PUT',
      body: JSON.stringify(setup),
    }),
  removeTargetInstructionsSetup: (name: string) =>
    apiFetch<{ success: boolean }>(`/targets/${encodeURIComponent(name)}/instructions/setup`, {
      method: 'DELETE',
    }),
  // A target's other files
  listTargetFiles: (name: string) =>
    apiFetch<TargetFileList>(`/targets/${encodeURIComponent(name)}/files`),
  getTargetFile: (name: string, path: string) =>
    apiFetch<TargetFile & { content: string }>(
      `/targets/${encodeURIComponent(name)}/files/content?path=${encodeURIComponent(path)}`,
    ),
  putTargetFile: (name: string, path: string, content: string) =>
    apiFetch<TargetFile & { content: string }>(
      `/targets/${encodeURIComponent(name)}/files/content?path=${encodeURIComponent(path)}`,
      {
        method: 'PUT',
        body: JSON.stringify({ content }),
      },
    ),
  addTargetFile: (name: string, path: string) =>
    apiFetch<TargetFileList>(`/targets/${encodeURIComponent(name)}/files`, {
      method: 'POST',
      body: JSON.stringify({ path }),
    }),
  removeTargetFile: (name: string, path: string) =>
    apiFetch<TargetFileList>(
      `/targets/${encodeURIComponent(name)}/files?path=${encodeURIComponent(path)}`,
      { method: 'DELETE' },
    ),
  listSharedInstructions: () =>
    apiFetch<{
      files: SharedInstructionsFile[];
      targets: SharedInstructionsTarget[];
      file_links: boolean;
    }>('/instructions'),
  createSharedInstructions: (body: { name: string; content?: string; from_target?: string }) =>
    apiFetch<{ success: boolean; path: string }>('/instructions', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  getSharedInstructionsContent: (name: string) =>
    apiFetch<{ name: string; path: string; exists: boolean; content: string }>(
      `/instructions/${encodeURIComponent(name)}/content`,
    ),
  /** Saves the shared file; its copy targets are rewritten too (copies). */
  putSharedInstructionsContent: (name: string, content: string) =>
    apiFetch<{ success: boolean; copies?: SharedCopyResult[] }>(
      `/instructions/${encodeURIComponent(name)}/content`,
      {
        method: 'PUT',
        body: JSON.stringify({ content }),
      },
    ),
  /** Sets exactly which shared files each target uses; [] restores their own files. */
  assignSharedInstructions: (targets: string[], extras: string[]) =>
    apiFetch<{ success: boolean; errors: string[]; warnings?: InstructionsWarning[] }>(
      '/instructions/assign',
      {
        method: 'POST',
        body: JSON.stringify({ targets, extras }),
      },
    ),
  restoreSharedInstructions: (name: string, target: string) =>
    apiFetch<{ success: boolean; errors: string[] }>(
      `/instructions/${encodeURIComponent(name)}/restore`,
      {
        method: 'POST',
        body: JSON.stringify({ target }),
      },
    ),
  setSharedInstructionsMode: (name: string, target: string, mode: string) =>
    apiFetch<{ success: boolean; warnings?: InstructionsWarning[] }>(
      `/instructions/${encodeURIComponent(name)}/targets/${encodeURIComponent(target)}/mode`,
      {
        method: 'PUT',
        body: JSON.stringify({ mode }),
      },
    ),
  getSharedRestorePreview: (name: string, target: string) =>
    apiFetch<SharedRestorePreview>(
      `/instructions/${encodeURIComponent(name)}/restore-preview?target=${encodeURIComponent(target)}`,
    ),
  addInstructionLocation: (name: string, body: { path: string; as?: string; mode?: string }) =>
    apiFetch<{ success: boolean; warnings?: InstructionsWarning[] }>(
      `/instructions/${encodeURIComponent(name)}/locations`,
      {
        method: 'POST',
        body: JSON.stringify(body),
      },
    ),
  setInstructionLocationMode: (name: string, path: string, mode: string) =>
    apiFetch<{ success: boolean; warnings?: InstructionsWarning[] }>(
      `/instructions/${encodeURIComponent(name)}/locations/mode`,
      {
        method: 'PUT',
        body: JSON.stringify({ path, mode }),
      },
    ),
  removeInstructionLocation: (name: string, path: string) =>
    apiFetch<{ success: boolean; warnings?: InstructionsWarning[] }>(
      `/instructions/${encodeURIComponent(name)}/locations?path=${encodeURIComponent(path)}`,
      { method: 'DELETE' },
    ),
  getLocationRestorePreview: (name: string, path: string) =>
    apiFetch<SharedRestorePreview>(
      `/instructions/${encodeURIComponent(name)}/locations/restore-preview?path=${encodeURIComponent(path)}`,
    ),
  /** Settles a modified target, or a location when `on` has its `path`. */
  resolveSharedInstructions: (
    name: string,
    on: { target: string } | { path: string },
    action: 'collect' | 'reapply',
  ) =>
    apiFetch<{ success: boolean }>(`/instructions/${encodeURIComponent(name)}/resolve`, {
      method: 'POST',
      body: JSON.stringify({ ...on, action }),
    }),
  getProjectInstructions: () => apiFetch<ProjectInstructions>('/instructions/project'),
  putProjectInstructions: (content: string) =>
    apiFetch<{ success: boolean }>('/instructions/project', {
      method: 'PUT',
      body: JSON.stringify({ content }),
    }),
  addProjectInstructionsShim: (target: string) =>
    apiFetch<{ success: boolean }>('/instructions/project/shim', {
      method: 'POST',
      body: JSON.stringify({ target }),
    }),
};
