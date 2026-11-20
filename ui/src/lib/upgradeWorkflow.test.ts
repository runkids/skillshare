import { afterEach, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import { upgradeAndReconnect } from './upgradeWorkflow';
afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});
it.each([true, false])(
  'reloads only if the restarted server recovers (recovers=%s)',
  async (recovers) => {
    vi.useFakeTimers();
    vi.spyOn(api, 'upgradeApp').mockResolvedValue({ ok: true, updated: true });
    vi.spyOn(api, 'restartApp').mockResolvedValue({ ok: true, restarting: true });
    const health = vi.spyOn(api, 'health').mockRejectedValue(new Error('offline'));
    if (recovers)
      health
        .mockRejectedValueOnce(new Error('offline'))
        .mockResolvedValueOnce({ status: 'ok', version: '1', uptime_seconds: 1 });
    const reload = vi.fn();
    const result = upgradeAndReconnect(vi.fn(), reload);
    await vi.runAllTimersAsync();
    expect(await result).toBe(recovers ? 'reloaded' : 'manual');
    expect(reload).toHaveBeenCalledTimes(recovers ? 1 : 0);
    expect(health).toHaveBeenCalledTimes(recovers ? 2 : 40);
  },
);
