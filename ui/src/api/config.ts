import { apiFetch } from './http';
import type { AgentignoreResponse, AvailableTarget, ConfigSaveResponse, SkillfollowResponse, SkillignoreResponse } from './types/config';

export const configApi = {
  getConfig: () => apiFetch<{ config: unknown; raw: string }>('/config'),
  /** Settings the Settings page owns; the rest of config.yaml stays untouched. */
  patchConfig: (body: { mode?: string; logMaxEntries?: number; backupMaxCount?: number; backupMaxSizeMB?: number }) =>
    apiFetch<{ mode: string; logMaxEntries: number | null }>('/config', {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  putConfig: (raw: string) =>
    apiFetch<ConfigSaveResponse>('/config', {
      method: 'PUT',
      body: JSON.stringify({ raw }),
    }),
  availableTargets: () => apiFetch<{ targets: AvailableTarget[] }>('/config/available-targets'),
  getSkillignore: () => apiFetch<SkillignoreResponse>('/skillignore'),
  putSkillignore: (raw: string) =>
    apiFetch<{ success: boolean }>('/skillignore', {
      method: 'PUT',
      body: JSON.stringify({ raw }),
    }),
  getSkillfollow: () => apiFetch<SkillfollowResponse>('/skillfollow'),
  /** Writes one declaration file; empty content with delete removes it. */
  putSkillfollow: (file: 'base' | 'local', content: string) =>
    apiFetch<SkillfollowResponse>('/skillfollow', {
      method: 'PUT',
      body: JSON.stringify({ file, content, delete: content === '' }),
    }),
  getAgentignore: () => apiFetch<AgentignoreResponse>('/agentignore'),
  putAgentignore: (raw: string) =>
    apiFetch<{ success: boolean }>('/agentignore', {
      method: 'PUT',
      body: JSON.stringify({ raw }),
    }),
};
