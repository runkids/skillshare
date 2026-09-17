import { describe, expect, it } from 'vitest';
import type { SyncMatrixEntry, Target } from '../api/client';
import { targetFilterPatch } from './targetFilter';

const entry = (status: SyncMatrixEntry['status']): SyncMatrixEntry => ({ skill: 'pdf', target: 'claude', status, reason: '' });
const target = (filters: Partial<Target>): Target => ({
  name: 'claude', path: '', mode: 'merge', targetNaming: 'flat', status: '', linkedCount: 0, localCount: 0,
  include: [], exclude: [], expectedSkillCount: 0, ...filters,
});

describe('targetFilterPatch', () => {
  it('turns a synced skill off by adding it to the exclude list', () => {
    expect(targetFilterPatch(entry('synced'), target({ exclude: ['old'] }), 'skill', 'pdf')).toEqual({ include: undefined, exclude: ['old', 'pdf'] });
  });

  it('turns an exactly excluded skill back on by removing the entry', () => {
    expect(targetFilterPatch(entry('excluded'), target({ exclude: ['pdf', 'old'] }), 'skill', 'pdf')).toEqual({ include: undefined, exclude: ['old'] });
  });

  it('refuses when a glob excludes the skill, since removing it would affect others', () => {
    expect(targetFilterPatch(entry('excluded'), target({ exclude: ['pdf*'] }), 'skill', 'pdf')).toBeNull();
  });

  it('adds a skill missing from the include list', () => {
    expect(targetFilterPatch(entry('not_included'), target({ include: ['docx'] }), 'skill', 'pdf')).toEqual({ include: ['docx', 'pdf'], exclude: undefined });
  });

  it('writes agent filters without the .md extension', () => {
    expect(targetFilterPatch(entry('synced'), target({ agentExclude: [] }), 'agent', 'reviewer.md')).toEqual({ agent_include: undefined, agent_exclude: ['reviewer'] });
  });

  it('leaves symlink-mode and targets-field decisions alone', () => {
    expect(targetFilterPatch(entry('na'), target({}), 'skill', 'pdf')).toBeNull();
  });
});
