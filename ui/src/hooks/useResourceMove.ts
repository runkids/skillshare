import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import { clearAuditCache } from '../lib/auditCache';
import { queryKeys } from '../lib/queryKeys';
import { invalidate } from '../lib/queryEvents';

const wire = (dest: string) => (dest === '' ? '.' : dest);

export function useResourceMove(names: string[], dest: string | null, completed: boolean) {
  const queryClient = useQueryClient();
  // What a move to `dest` would do: new paths, link names, refusals, filter warnings. Changes nothing, so
  // it is not worth repeating on focus, and it stops once the real move has run.
  const preview = useQuery({
    queryKey: queryKeys.movePreview(names, dest ?? ''),
    queryFn: () => api.moveResources({ names, dest: wire(dest ?? ''), dryRun: true }),
    enabled: dest !== null && !completed,
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  const move = useMutation({
    mutationFn: ({
      targets,
      destination,
      force,
    }: {
      targets: string[];
      destination: string;
      force: boolean;
    }) => api.moveResources({ names: targets, dest: wire(destination), force: force || undefined }),
    onSuccess: () => {
      clearAuditCache(queryClient);
      void invalidate(queryClient, 'skillsMoved');
    },
  });
  return { preview, move };
}
