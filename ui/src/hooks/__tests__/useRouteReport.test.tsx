import { act, renderHook } from '@testing-library/react';
import { MemoryRouter, useNavigate } from 'react-router-dom';
import type { ReactNode } from 'react';
import { afterEach, expect, it, vi } from 'vitest';
import { useRouteReport } from '../useRouteReport';

const wrapper = ({ children }: { children: ReactNode }) => (
  <MemoryRouter initialEntries={['/skills?kind=skill']}>{children}</MemoryRouter>
);
const useBoth = () => { useRouteReport(); return useNavigate(); };

afterEach(() => vi.restoreAllMocks());

it('posts the router path to the parent when embedded, on mount and on navigation', () => {
  const post = vi.fn();
  vi.spyOn(window, 'parent', 'get').mockReturnValue({ postMessage: post } as unknown as Window);
  const { result } = renderHook(useBoth, { wrapper });
  expect(post).toHaveBeenLastCalledWith({ type: 'route-change', path: '/skills?kind=skill' }, '*');
  act(() => result.current('/targets'));
  expect(post).toHaveBeenLastCalledWith({ type: 'route-change', path: '/targets' }, '*');
});

it('posts nothing when not embedded', () => {
  const post = vi.spyOn(window, 'postMessage');
  renderHook(useRouteReport, { wrapper });
  expect(post).not.toHaveBeenCalled();
});
