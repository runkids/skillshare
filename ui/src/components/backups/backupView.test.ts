import { describe, expect, it } from 'vitest';
import type { FileBackupVersion } from '../../api/client';
import { backupItem, filterItems, reasonKey } from './backupView';

const version = (v: Partial<FileBackupVersion>): FileBackupVersion => ({
  id: 'x',
  kind: 'history',
  reason: '',
  time: '',
  size: 0,
  preview: '',
  ...v,
});

describe('backupView', () => {
  it("reads a -agents snapshot as that target's agents folder", () => {
    expect(backupItem('claude-agents')).toEqual({
      name: 'claude-agents',
      target: 'claude',
      kind: 'agents',
    });
  });

  it("keeps one target's skills and agents folders under its filter", () => {
    const items = ['claude', 'claude-agents', 'codex'].map(backupItem);
    expect(filterItems(items, 'claude').map((i) => i.name)).toEqual(['claude', 'claude-agents']);
  });

  it('describes a version saved before reasons were recorded as a plain write', () => {
    expect(reasonKey(version({ reason: '' }))).toBe('backup.files.reason.write');
  });

  it('describes an origin that was no file as no file', () => {
    expect(reasonKey(version({ kind: 'origin', none: true }))).toBe('backup.files.origin.none');
  });
});
