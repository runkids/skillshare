import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type BatchToggleResult, type Skill } from '../api/client';
import { invalidate } from '../lib/queryEvents';
import { queryKeys } from '../lib/queryKeys';

type One = { s: Skill; disable: boolean };
type Many = { names: string[]; enable: boolean; kind: Skill['kind'] };
type Feedback = {
  onOne: (input: One) => void;
  onMany: (result: BatchToggleResult, input: Many) => void;
  onError: (error: Error) => void;
};
type List = Awaited<ReturnType<typeof api.listSkills>>;

// Both endpoints write the same flags. Queue writes (including reconciliation) across hook instances.
const scope = { id: 'resource-toggle' };
const queryKey = queryKeys.skills.all;

export function useResourceToggle(feedback: Feedback) {
  const client = useQueryClient();

  async function optimistic<T>(
    kind: Skill['kind'],
    names: string[],
    disabled: boolean,
    request: () => Promise<T>,
    failedNames?: (result: T) => string[],
  ) {
    await client.cancelQueries({ queryKey, exact: true });
    const selected = new Set(names);
    const previous = new Map<string, boolean | undefined>();
    client.setQueryData<List>(
      queryKey,
      (list) =>
        list && {
          ...list,
          resources: list.resources.map((s) => {
            if (s.kind !== kind || !selected.has(s.flatName)) return s;
            previous.set(s.flatName, s.disabled);
            return { ...s, disabled };
          }),
        },
    );
    function restore(failed: Set<string>) {
      if (!failed.size) return;
      client.setQueryData<List>(
        queryKey,
        (list) =>
          list && {
            ...list,
            resources: list.resources.map((s) =>
              s.kind === kind && failed.has(s.flatName) && previous.has(s.flatName)
                ? { ...s, disabled: previous.get(s.flatName) }
                : s,
            ),
          },
      );
    }
    try {
      const result = await request();
      if (failedNames) restore(new Set(failedNames(result)));
      return result;
    } catch (error) {
      restore(selected);
      throw error;
    }
  }

  const toggleOne = useMutation({
    scope,
    mutationFn: ({ s, disable }: One) =>
      optimistic(s.kind, [s.flatName], disable, () =>
        disable ? api.disableResource(s.flatName, s.kind) : api.enableResource(s.flatName, s.kind),
      ),
    onSuccess: (_, input) => feedback.onOne(input),
    onError: feedback.onError,
    onSettled: () => invalidate(client, 'skillSyncChanged'),
  });
  const toggleMany = useMutation({
    scope,
    mutationFn: ({ kind, names, enable }: Many) =>
      optimistic(
        kind,
        names,
        !enable,
        () => api.batchToggleResources(names, enable, kind),
        (result) => result.results.filter((r) => !r.success).map((r) => r.name),
      ),
    onSuccess: (result, input) => feedback.onMany(result, input),
    onError: feedback.onError,
    onSettled: () => invalidate(client, 'skillsToggled'),
  });
  return { toggleOne, toggleMany };
}
