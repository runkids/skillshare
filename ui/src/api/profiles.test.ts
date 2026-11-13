import { afterEach, expect, it, vi } from 'vitest';
import { api, ApiError } from './client';

afterEach(() => vi.unstubAllGlobals());

it('lists server selections without collapsing pass-through and empty definitions', async () => {
  const data = { active: '', state: 'none', pending: false, profiles: [
    { name: 'all', skills: null, selected: 3, unmatched: [], usable: true },
    { name: 'empty', skills: { include: [], exclude: [] }, selected: 0, unmatched: [], usable: true },
  ] };
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(data)));
  vi.stubGlobal('fetch', fetch);
  expect(await api.profiles.list()).toEqual(data);
  expect(fetch).toHaveBeenCalledWith('/api/profiles', expect.any(Object));
});

it.each([
  { name: null, dryRun: true },
  { name: 'work', discardEdits: true, allowEmpty: true },
])('posts exactly the reviewed switch body %j', async (body) => {
  const result = { profile: { previous: 'old', active: body.name ?? '', pending: true }, complete: false, targets: [] };
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(result)));
  vi.stubGlobal('fetch', fetch);
  expect(await api.profiles.setActive(body)).toEqual(result);
  expect(fetch).toHaveBeenCalledWith('/api/profiles/active', expect.objectContaining({ method: 'POST', body: JSON.stringify(body) }));
});

it('retains the typed refusal and serialized blocked entries in ApiError', async () => {
  const params = { blocked: JSON.stringify([{ target: 'claude', entry: 'a', class: 'modified_copy', reason: 'edited' }]) };
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
    error: 'switch blocked', error_code: 'profile_blocked', error_params: params,
  }), { status: 409 })));
  const error = await api.profiles.setActive({ name: 'work' }).catch((e: unknown) => e);
  expect(error).toBeInstanceOf(ApiError);
  expect(error).toMatchObject({ status: 409, code: 'profile_blocked', params, message: 'switch blocked' });
});

it.each([
  ['create', [{ name: 'work' }], 'POST', '/api/profiles', { name: 'work' }],
  ['create', [{ name: 'work', skills: { include: [] } }], 'POST', '/api/profiles', { name: 'work', skills: { include: [] } }],
  ['replace', ['work', { description: 'Focus' }], 'PUT', '/api/profiles/work', { description: 'Focus' }],
  ['replace', ['work', { skills: null }], 'PUT', '/api/profiles/work', { skills: null }],
  ['replace', ['work', { name: 'renamed', skills: { include: [] } }], 'PUT', '/api/profiles/work', { name: 'renamed', skills: { include: [] } }],
  ['delete', ['work'], 'DELETE', '/api/profiles/work', undefined],
  ['assign', ['work', { add: ['team__review'], remove: ['alpha'] }], 'POST', '/api/profiles/work/skills', { add: ['team__review'], remove: ['alpha'] }],
  ['preview', [{ skills: { include: ['team__*', 'missing-*'], exclude: ['old'] } }], 'POST', '/api/profiles/preview', { skills: { include: ['team__*', 'missing-*'], exclude: ['old'] } }],
] as const)('forwards %s definition requests without collapsing selection presence', async (method, args, verb, path, body) => {
  const result = method === 'preview'
    ? { selected: ['team__review'], unmatched: ['missing-*'] }
    : { changed: true, needsSync: method === 'replace' || method === 'assign', skills: method === 'assign' ? [{ name: 'team__review', result: 'added' }] : null };
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(result)));
  vi.stubGlobal('fetch', fetch);
  const invoke = Reflect.get(api.profiles, method);
  expect(invoke).toBeTypeOf('function');
  expect(await invoke(...args)).toEqual(result);
  expect(fetch).toHaveBeenCalledExactlyOnceWith(path, expect.objectContaining({ method: verb }));
  expect(fetch.mock.calls[0][1].body).toBe(body === undefined ? undefined : JSON.stringify(body));
});
