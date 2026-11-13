import { act, render } from '@testing-library/react';
import { vi } from 'vitest';
import App from '../App';
import { api, type ProfileDefinition, type ProfilesResponse } from '../api/client';
import { mcpApi } from '../api/mcp';
import { hooksApi } from '../api/hooks';
import { queryClient } from '../lib/queryClient';

export const definition = (name = 'work', extra: Partial<ProfileDefinition> = {}): ProfileDefinition => ({
  name, description: 'Focus', skills: { include: ['alpha'], exclude: [] }, selected: 1,
  unmatched: [], usable: true, ...extra,
});
export function profileFixtures(project = false, rows = [definition()]) {
  vi.restoreAllMocks();
  localStorage.clear();
  localStorage.setItem('skillshare.tour.completed', 'true');
  queryClient.clear();
  queryClient.setDefaultOptions({ queries: { retry: false }, mutations: { retry: false } });
  vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }));
  HTMLElement.prototype.scrollIntoView = vi.fn();
  vi.spyOn(api, 'getOverview').mockResolvedValue({ source: '/fixture/source', skillCount: 2, variantCount: 0, agentCount: 0, topLevelCount: 2, targetCount: 0, mode: 'copy', version: 'dev', trackedRepos: [], isProjectMode: project, configDir: '/fixture', projectRoot: project ? '/fixture/project' : undefined });
  vi.spyOn(api, 'listTargets').mockResolvedValue({ targets: [], sourceSkillCount: 2 });
  const ignored = { ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [] };
  vi.spyOn(api, 'diff').mockResolvedValue({ diffs: [], ...ignored });
  vi.spyOn(api, 'diffExtras').mockResolvedValue({ extras: [] });
  vi.spyOn(api, 'gitStatus').mockResolvedValue({ isRepo: false } as Awaited<ReturnType<typeof api.gitStatus>>);
  vi.spyOn(mcpApi, 'list').mockResolvedValue({ plan: null } as Awaited<ReturnType<typeof mcpApi.list>>);
  vi.spyOn(hooksApi, 'list').mockResolvedValue({ plan: null } as Awaited<ReturnType<typeof hooksApi.list>>);
  vi.spyOn(api, 'getVersionCheck').mockResolvedValue({ cliVersion: 'dev', skillVersion: 'dev', cliUpdateAvailable: false, skillUpdateAvailable: false });
  const catalog: ProfilesResponse = { active: '', state: 'none', pending: false, profiles: rows };
  const changed = { changed: true, needsSync: false, skills: null };
  return {
    catalog,
    list: vi.spyOn(api.profiles, 'list').mockResolvedValue(catalog),
    create: vi.spyOn(api.profiles, 'create').mockResolvedValue(changed),
    replace: vi.spyOn(api.profiles, 'replace').mockResolvedValue(changed),
    remove: vi.spyOn(api.profiles, 'delete').mockResolvedValue(changed),
    assign: vi.spyOn(api.profiles, 'assign').mockResolvedValue(changed),
    preview: vi.spyOn(api.profiles, 'preview').mockResolvedValue({ selected: ['alpha'], unmatched: [] }),
    sync: vi.spyOn(api, 'sync').mockResolvedValue({ results: [], ...ignored }),
  };
}
export async function mountProfiles(path = '/profiles') {
  await act(async () => {
    window.history.replaceState(null, '', path);
    window.dispatchEvent(new PopStateEvent('popstate'));
  });
  return render(<App />);
}
