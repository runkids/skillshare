import { http } from './http';
import type {
  GitBranches,
  GitCheckoutResponse,
  GitPullResolution,
  GitStatus,
  PullResponse,
  PushResponse,
} from './types/git';

export const gitApi = {
  gitStatus: () => http.get<GitStatus>('/git/status'),
  gitSetRoot: (scope: string, remoteURL?: string) =>
    http.post<{ success: boolean; scope: string; gitRoot: string }>(
      '/git/root',
      remoteURL ? { scope, remoteURL } : { scope },
    ),
  gitBranches: (opts?: { fetch?: boolean }) =>
    http.get<GitBranches>(`/git/branches${opts?.fetch ? '?fetch=true' : ''}`),
  gitCheckout: (branch: string) => http.post<GitCheckoutResponse>('/git/checkout', { branch }),
  gitAbsorbNested: (subdirs: string[]) =>
    http.post<{ success: boolean; disabled: string[] }>('/git/absorb-nested', { subdirs }),
  gitCommit: (opts: { message?: string; dryRun?: boolean }) =>
    http.post<PushResponse>('/git/commit', opts),
  gitDiscard: (opts: { dryRun?: boolean }) => http.post<PushResponse>('/git/discard', opts),
  push: (opts: { message?: string; dryRun?: boolean }) => http.post<PushResponse>('/push', opts),
  /** force replaces local files with the remote when a first pull cannot merge (error code merge_failed). */
  pull: (opts?: {
    dryRun?: boolean;
    force?: boolean;
    resolution?: GitPullResolution;
    alwaysSync?: boolean;
  }) => http.post<PullResponse>('/pull', opts ?? {}),
};
