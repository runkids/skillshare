import { apiFetch } from './http';
import type { ActiveProfileRequest, ProfileApplyResult, ProfileDraft, ProfileMembershipRequest, ProfileMutationResult, ProfilePreviewResult, ProfilesResponse } from './types/profiles';

export const profilesApi = {
  profiles: {
    list: () => apiFetch<ProfilesResponse>('/profiles'),
    create: (body: ProfileDraft & { name: string }) => apiFetch<ProfileMutationResult>('/profiles', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
    replace: (name: string, body: ProfileDraft) => apiFetch<ProfileMutationResult>(`/profiles/${encodeURIComponent(name)}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
    delete: (name: string) => apiFetch<ProfileMutationResult>(`/profiles/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }),
    assign: (name: string, body: ProfileMembershipRequest) => apiFetch<ProfileMutationResult>(`/profiles/${encodeURIComponent(name)}/skills`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
    preview: (body: Pick<ProfileDraft, 'skills'>) => apiFetch<ProfilePreviewResult>('/profiles/preview', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
    setActive: (body: ActiveProfileRequest) => apiFetch<ProfileApplyResult>('/profiles/active', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  },
};
