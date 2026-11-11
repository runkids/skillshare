import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import { queryKeys } from '../lib/queryKeys';

/** Poll only the local preference, never a release endpoint. */
export function useUpdatePolicy() {
  return useQuery({
    queryKey: queryKeys.updatePolicy,
    queryFn: () => api.getUpdatePolicy(),
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
  });
}
