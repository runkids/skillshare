import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, expect, it, vi } from 'vitest';
import { api, type Skill } from '../../api/client';
import { queryKeys } from '../../lib/queryKeys';
import { useResourceToggle } from '../useResourceToggle';

afterEach(() => vi.restoreAllMocks());

function resource(flatName: string, kind: Skill['kind']): Skill {
  return { flatName, kind, name: flatName, sourcePath: '', relPath: flatName, isInRepo: false };
}

function setup() {
  const client = new QueryClient();
  const skill = resource('same', 'skill');
  const agent = resource('same', 'agent');
  const key = queryKeys.skills.all;
  client.setQueryData(key, { resources: [skill, agent, resource('other', 'skill')] });
  const feedback = { onMany: vi.fn(), onOne: vi.fn(), onError: vi.fn() };
  const { result } = renderHook(() => useResourceToggle(feedback), {
    wrapper: ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    ),
  });
  const flags = () =>
    client.getQueryData<{ resources: Skill[] }>(key)?.resources.map((s) => !!s.disabled);
  return { result, flags, skill, feedback };
}

it('keeps successful batch changes and does not touch a same-named agent', async () => {
  const { result, flags, feedback } = setup();
  const response = {
    summary: { updated: 1, unchanged: 0, failed: 1 },
    results: [
      { name: 'same', success: true, disabled: true },
      { name: 'other', success: false, disabled: false, error: 'read-only' },
    ],
  };
  vi.spyOn(api, 'batchToggleResources').mockResolvedValue(response);
  const input = { kind: 'skill' as const, names: ['same', 'other'], enable: false };
  await act(() => result.current.toggleMany.mutateAsync(input));
  expect(flags()).toEqual([true, false, false]);
  expect(feedback.onMany).toHaveBeenCalledWith(response, input);
});

it('runs a queued single toggle after a failed overlapping batch rolls back', async () => {
  const { result, flags, skill } = setup();
  let reject!: (e: Error) => void;
  vi.spyOn(api, 'batchToggleResources').mockImplementation(
    () =>
      new Promise((_, no) => {
        reject = no;
      }),
  );
  const single = vi
    .spyOn(api, 'disableResource')
    .mockResolvedValue({ name: 'same', success: true, disabled: true });
  act(() =>
    result.current.toggleMany.mutate({ kind: 'skill', names: ['same', 'other'], enable: false }),
  );
  await waitFor(() => expect(flags()).toEqual([true, false, true]));
  act(() => result.current.toggleOne.mutate({ s: skill, disable: true }));
  expect(single).not.toHaveBeenCalled();
  reject(new Error('write failed'));
  await waitFor(() => expect(result.current.toggleOne.isSuccess).toBe(true));
  expect(flags()).toEqual([true, false, false]);
});
