import { apiFetch } from './http';
import type { ActiveProfileRequest, ProfileApplyResult, ProfilesResponse } from './types/profiles';

export const profilesApi = {
  profiles: {
    list: () => apiFetch<ProfilesResponse>('/profiles'),
    setActive: (body: ActiveProfileRequest) => apiFetch<ProfileApplyResult>('/profiles/active', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  },
};
