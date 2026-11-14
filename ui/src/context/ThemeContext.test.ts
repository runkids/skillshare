import { afterEach, expect, it, vi } from 'vitest';

afterEach(() => vi.restoreAllMocks());

it('tells an embedding app that a new page is starting', async () => {
  const post = vi.fn();
  vi.spyOn(window, 'parent', 'get').mockReturnValue({ postMessage: post } as unknown as Window);
  vi.resetModules();
  await import('./ThemeContext');
  expect(post).toHaveBeenCalledWith({ type: 'theme-init' }, '*');
});
