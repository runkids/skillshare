import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import { queryKeys } from '../lib/queryKeys';
import { invalidate } from '../lib/queryEvents';

/** Poll only the local preference, never a release endpoint. */
export function useUpdatePolicy() {
  const queryClient = useQueryClient();
  return useQuery({
    queryKey: queryKeys.updatePolicy,
    queryFn: async () => {
      const previous = queryClient.getQueryData<{ enabled: boolean }>(queryKeys.updatePolicy);
      const next = await api.getUpdatePolicy();
      if (previous && previous.enabled !== next.enabled) void invalidate(queryClient, 'updatePolicyChanged');
      return next;
    },
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
  });
}
