import { describe, expect, it } from 'vitest';
import { buildSyncToast, sumAll, sumEntry, syncToastType } from './extrasSyncToast';

const t = (key: string, _params?: unknown, fallback?: string) => fallback ?? key;

describe('interrupted extras sync result consumers', () => {
  it.each(['single', 'all'] as const)('shows entry warnings through the %s toast owner', (scope) => {
    const warning = 'extra "rules" has an interrupted publication; run: skillshare install example-org/hub --kind extra --name rules -g';
    // This is the API result when the live source exists but its marker remains.
    const entry = { name: 'rules', targets: [], warnings: [warning] };
    const totals = scope === 'single' ? sumEntry(entry) : sumAll([entry]);
    const message = buildSyncToast('Extras synced', 'Extras sync failed', totals, false, t);

    expect(message).toContain(warning);
    expect(message).not.toContain('no files in source');
    expect(syncToastType(totals)).not.toBe('success');
    expect(totals.synced).toBe(0);
    expect(totals.targets).toBe(0);
  });

  it('keeps ordinary empty-source success without an interruption warning', () => {
    const totals = sumEntry({ name: 'ordinary', targets: [] });

    expect(buildSyncToast('Extras synced', 'Extras sync failed', totals, false, t)).toBe('Extras synced — no files in source');
    expect(syncToastType(totals)).toBe('success');
  });

  it('keeps ordinary local sync counts and success', () => {
    const totals = sumEntry({ name: 'local', targets: [{ target: '/fixture/target', mode: 'copy', synced: 2, skipped: 0, pruned: 0 }] });

    expect(buildSyncToast('Extras synced', 'Extras sync failed', totals, false, t)).toBe('Extras synced — 2 files to 1 target');
    expect(syncToastType(totals)).toBe('success');
    expect(totals.synced).toBe(2);
    expect(totals.targets).toBe(1);
  });
});
