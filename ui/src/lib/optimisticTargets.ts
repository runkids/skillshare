import type { QueryClient } from '@tanstack/react-query';
import type { Skill } from '../api/client';
import { queryKeys } from './queryKeys';
type SkillsData = { resources: Skill[] };

export async function optimisticTargets(
  queryClient: QueryClient,
  kind: Skill['kind'],
  name: string,
  target: string | null,
) {
  const key = queryKeys.skills.all;
  await queryClient.cancelQueries({ queryKey: key, exact: true });
  const previous = queryClient
    .getQueryData<SkillsData>(key)
    ?.resources.find((s) => s.kind === kind && s.flatName === name);
  if (!previous) return;
  const patchTargets = (targets: Skill['targets']) =>
    queryClient.setQueryData<SkillsData>(
      key,
      (list) =>
        list && {
          ...list,
          resources: list.resources.map((s) =>
            s.kind === previous.kind && s.flatName === name ? { ...s, targets } : s,
          ),
        },
    );
  patchTargets(target ? [target] : undefined);
  return () => patchTargets(previous.targets);
}
