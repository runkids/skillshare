import { auditApi } from './audit';
import { backupsApi } from './backups';
import { configApi } from './config';
import { extrasApi } from './extras';
import { gitApi } from './git';
import { installApi } from './install';
import { instructionsApi } from './instructions';
import { logApi } from './log';
import { memoryApi } from './memory';
import { resourcesApi } from './resources';
import { syncApi } from './sync';
import { systemApi } from './system';
import { targetsApi } from './targets';
import { updateApi } from './update';

export { ApiError, parseApiErrorPayload, apiFetch } from './http';
export * from './types/audit';
export * from './types/backups';
export * from './types/config';
export * from './types/diagnostics';
export * from './types/extras';
export * from './types/git';
export * from './types/install';
export * from './types/instructions';
export * from './types/log';
export * from './types/resources';
export * from './types/sync';
export * from './types/targets';
export * from './types/update';

// Typed API helpers. Each domain module owns its methods; this object only composes them.
export const api = {
  ...resourcesApi,
  ...targetsApi,
  ...syncApi,
  ...installApi,
  ...updateApi,
  ...systemApi,
  ...configApi,
  ...backupsApi,
  ...extrasApi,
  ...instructionsApi,
  ...logApi,
  ...memoryApi,
  ...auditApi,
  ...gitApi,

  // Aliases go through `api` so tests that replace it still intercept them.
  getSkill: (name: string, kind?: 'skill' | 'agent') =>
    api.getResource(name, kind),
  deleteSkill: (name: string, kind?: 'skill' | 'agent') =>
    api.deleteResource(name, kind),
  disableSkill: (name: string, kind?: 'skill' | 'agent') =>
    api.disableResource(name, kind),
  enableSkill: (name: string, kind?: 'skill' | 'agent') =>
    api.enableResource(name, kind),
};
