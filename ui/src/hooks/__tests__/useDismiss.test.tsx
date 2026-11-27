import { fireEvent, renderHook } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { useDismiss } from '../useDismiss';

const mount = (open = true) => {
  const box = document.body.appendChild(document.createElement('div'));
  const inner = box.appendChild(document.createElement('button'));
  const ref = { current: box };
  const close = vi.fn();
  const hook = renderHook((isOpen: boolean) => useDismiss(ref, isOpen, close), {
    initialProps: open,
  });
  return { inner, close, ...hook };
};
const dismissBoth = () => {
  fireEvent.mouseDown(document.body);
  fireEvent.keyDown(document.body, { key: 'Escape' });
};

afterEach(() => document.body.replaceChildren());

it('closes on a mousedown outside', () => {
  const { close } = mount();
  fireEvent.mouseDown(document.body);
  expect(close).toHaveBeenCalledTimes(1);
});

it('stays open on a mousedown inside', () => {
  const { inner, close } = mount();
  fireEvent.mouseDown(inner);
  expect(close).not.toHaveBeenCalled();
});

it('closes on Escape only, passing the event', () => {
  const { inner, close } = mount();
  fireEvent.keyDown(inner, { key: 'Enter' });
  expect(close).not.toHaveBeenCalled();
  fireEvent.keyDown(inner, { key: 'Escape' });
  expect(close).toHaveBeenCalledWith(expect.any(KeyboardEvent));
});

it('does nothing while closed', () => {
  const { close } = mount(false);
  dismissBoth();
  expect(close).not.toHaveBeenCalled();
});

it.each(['close', 'unmount'])('stops listening after %s', (how) => {
  const { close, rerender, unmount } = mount();
  if (how === 'close') rerender(false);
  else unmount();
  dismissBoth();
  expect(close).not.toHaveBeenCalled();
});
