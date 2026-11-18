import { QueryClient } from '@tanstack/react-query';
import { expect, it } from 'vitest';
import { optimisticTargets } from './optimisticTargets';
import { queryKeys } from './queryKeys';
import type { Skill } from '../api/client';

it('rolls back only the failed target edit, preserving a completed toggle and same-named agent', async () => {
  const client = new QueryClient();
  const key = queryKeys.skills.all;
  const skill = (name: string, kind: Skill['kind']): Skill => ({
    name,
    flatName: name,
    kind,
    relPath: name,
    sourcePath: '',
    isInRepo: false,
    targets: ['old'],
  });
  const a = skill('a', 'skill'),
    b = skill('b', 'skill'),
    agent = skill('a', 'agent');
  client.setQueryData(key, { resources: [a, b, agent] });
  const undo = await optimisticTargets(client, 'skill', 'a', 'new');
  const read = () => client.getQueryData<{ resources: Skill[] }>(key)!.resources;
  expect(read().map((s) => s.targets)).toEqual([['new'], ['old'], ['old']]);
  client.setQueryData(key, {
    resources: read().map((s) => (s === b ? { ...s, disabled: true } : s)),
  });
  undo?.();
  expect(read()).toEqual([a, { ...b, disabled: true }, agent]);
});
