import { describe, expect, it } from 'vitest';
import { buildSyncToast, sumAll, sumEntry } from './extrasSyncToast';
import type { ExtrasSyncResult } from '../api/client';
import type { TranslationParams } from '../i18n';

const t = (key: string, params?: TranslationParams, fallback?: string) => {
  switch (key) {
    case 'extras.toast.nErrors':
      return `${params?.errors ?? 0} error(s)`;
    case 'extras.toast.syncedNFiles':
      return `${params?.synced ?? 0} file(s) to ${params?.targets ?? 0} target(s)`;
    default:
      return fallback ?? key;
  }
};

describe('extras sync toast helpers', () => {
  it('includes the first per-file extension error in a failed sync toast', () => {
    const entry: ExtrasSyncResult = {
      name: 'codex-agents',
      targets: [
        {
          target: '~/.codex/agents',
          mode: 'copy',
          synced: 0,
          skipped: 0,
          pruned: 0,
          errors: [
            'reviewer.md: extension "codex-agents" failed: codex-agents: missing required markdown body (Codex custom agents require developer_instructions)',
          ],
        },
      ],
    };

    const toast = buildSyncToast('Extras synced', 'Extras sync failed', sumEntry(entry), false, t);

    expect(toast).toContain('Extras sync failed');
    expect(toast).toContain('1 error(s)');
    expect(toast).toContain('reviewer.md: extension "codex-agents" failed');
    expect(toast).toContain('developer_instructions');
  });

  it('keeps partial sync totals while surfacing the first error detail', () => {
    const entries: ExtrasSyncResult[] = [
      {
        name: 'rules',
        targets: [{ target: '/tmp/rules', mode: 'copy', synced: 2, skipped: 0, pruned: 0 }],
      },
      {
        name: 'codex-agents',
        targets: [
          {
            target: '~/.codex/agents',
            mode: 'copy',
            synced: 0,
            skipped: 0,
            pruned: 0,
            errors: ['agent.md: extension "codex-agents" failed: missing required frontmatter'],
          },
        ],
      },
    ];

    const toast = buildSyncToast('Extras synced', 'Extras sync failed', sumAll(entries), false, t);

    expect(toast).toContain('Extras synced');
    expect(toast).toContain('2 file(s) to 2 target(s)');
    expect(toast).toContain('agent.md: extension "codex-agents" failed');
  });

  it('summarizes additional errors after the first detail', () => {
    const entry: ExtrasSyncResult = {
      name: 'codex-agents',
      targets: [
        {
          target: '~/.codex/agents',
          mode: 'copy',
          synced: 0,
          skipped: 0,
          pruned: 0,
          errors: ['a.md: failed', 'b.md: failed', 'c.md: failed'],
        },
      ],
    };

    const toast = buildSyncToast('Extras synced', 'Extras sync failed', sumEntry(entry), false, t);

    expect(toast).toContain('3 error(s)');
    expect(toast).toContain('a.md: failed');
    expect(toast).toContain('+2 more');
    expect(toast).not.toContain('b.md: failed');
  });
});
