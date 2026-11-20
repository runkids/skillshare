import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useSlow } from '../useSlow';

describe('useSlow', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('turns true only once the wait has lasted, and resets when it ends', () => {
    const { result, rerender } = renderHook(({ active }) => useSlow(active, 5000), {
      initialProps: { active: true },
    });
    act(() => {
      vi.advanceTimersByTime(4999);
    });
    expect(result.current).toBe(false);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(true);
    rerender({ active: false });
    expect(result.current).toBe(false);
  });
});
