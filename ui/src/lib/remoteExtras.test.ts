import { describe, expect, it } from 'vitest';
import { extraRestoreCommand, extraRemovalMessage } from './remoteExtras';
import type { Extra } from '../api/client';

const t = (key: string, params?: Record<string, unknown>) => `${key}:${JSON.stringify(params)}`;

describe('remote extra guidance', () => {
  it('quotes shell metacharacters in an explicit restore command', () => {
    const extra = { name: 'rules', remote: 'example-org/hub', branch: "team's; branch" } as Extra;
    expect(extraRestoreCommand(extra, true)).toBe("skillshare install example-org/hub --kind extra --name rules --branch 'team'\"'\"'s; branch' -p");
    expect(extraRestoreCommand({ ...extra, branch: undefined }, false)).toBe('skillshare install example-org/hub --kind extra --name rules -g');
  });

  it('uses structured removal outcomes and retains output paths without English backend warnings', () => {
    const extra = { name: 'rules', source_dir: '/fixture/rules' } as Extra;
    const msg = extraRemovalMessage(extra, { success: true, remote: 'org/hub', kept_edited_dir: true, kept_outputs: ['/fixture/output'], warnings: ['backend prose'] }, t);
    expect(msg).toContain('extras.remote.retained:');
    expect(msg).toContain('/fixture/rules');
    expect(msg).toContain('/fixture/output');
    expect(msg).not.toContain('backend prose');
    expect(extraRemovalMessage(extra, { success: true, remote: 'org/hub', deleted_dir: true }, t)).toContain('extras.remote.deleted:');
    expect(extraRemovalMessage(extra, { success: true, remote: 'org/hub', missing_dir: true }, t)).toContain('extras.remote.alreadyMissing:');
    expect(extraRemovalMessage(extra, { success: true }, t)).toContain('extras.toast.removed:');
  });
});
