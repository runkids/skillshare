import { http } from './http';
import type {
  AgentignoreResponse,
  AvailableTarget,
  ConfigSaveResponse,
  SkillignoreResponse,
} from './types/config';

export const configApi = {
  getConfig: () => http.get<{ config: unknown; raw: string }>('/config'),
  /** Settings the Settings page owns; the rest of config.yaml stays untouched. */
  patchConfig: (body: {
    mode?: string;
    logMaxEntries?: number;
    backupMaxCount?: number;
    backupMaxSizeMB?: number;
    autoUpdateCheck?: boolean;
  }) => http.patch<{ mode: string; logMaxEntries: number | null }>('/config', body),
  putConfig: (raw: string) => http.put<ConfigSaveResponse>('/config', { raw }),
  availableTargets: () => http.get<{ targets: AvailableTarget[] }>('/config/available-targets'),
  getSkillignore: () => http.get<SkillignoreResponse>('/skillignore'),
  putSkillignore: (raw: string) => http.put<{ success: boolean }>('/skillignore', { raw }),
  getAgentignore: () => http.get<AgentignoreResponse>('/agentignore'),
  putAgentignore: (raw: string) => http.put<{ success: boolean }>('/agentignore', { raw }),
};
