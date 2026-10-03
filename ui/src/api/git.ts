import { apiFetch } from './http';
import type { GitBranches, GitCheckoutResponse, GitPullResolution, GitStatus, PullResponse, PushResponse } from './types/git';

export const gitApi = {
  gitStatus: () => apiFetch<GitStatus>('/git/status'),
  gitSetRoot: (scope: string, remoteURL?: string) =>
    apiFetch<{ success: boolean; scope: string; gitRoot: string }>('/git/root', {
      method: 'POST',
      body: JSON.stringify(remoteURL ? { scope, remoteURL } : { scope }),
    }),
  gitBranches: (opts?: { fetch?: boolean }) =>
    apiFetch<GitBranches>(`/git/branches${opts?.fetch ? '?fetch=true' : ''}`),
  gitCheckout: (branch: string) =>
    apiFetch<GitCheckoutResponse>('/git/checkout', {
      method: 'POST',
      body: JSON.stringify({ branch }),
    }),
  gitAbsorbNested: (subdirs: string[]) =>
    apiFetch<{ success: boolean; disabled: string[] }>('/git/absorb-nested', {
      method: 'POST',
      body: JSON.stringify({ subdirs }),
    }),
  gitCommit: (opts: { message?: string; dryRun?: boolean }) =>
    apiFetch<PushResponse>('/git/commit', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  gitDiscard: (opts: { dryRun?: boolean }) =>
    apiFetch<PushResponse>('/git/discard', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  push: (opts: { message?: string; dryRun?: boolean }) =>
    apiFetch<PushResponse>('/push', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  /** force replaces local files with the remote when a first pull cannot merge (error code merge_failed). */
  pull: (opts?: { dryRun?: boolean; force?: boolean; resolution?: GitPullResolution; alwaysSync?: boolean }) =>
    apiFetch<PullResponse>('/pull', {
      method: 'POST',
      body: JSON.stringify(opts ?? {}),
    }),
};
