import { afterEach, describe, expect, it, vi } from 'vitest';
import { segSlide } from './segSlide';

/** jsdom has no ResizeObserver; this one reports a resize only for elements being observed. */
let resize: (el: Element) => void;
class FakeResizeObserver {
  constructor(cb: () => void) {
    const targets = new Set<Element>();
    this.observe = (el) => void targets.add(el);
    resize = (el) => {
      if (targets.has(el)) cb();
    };
  }
  observe: (el: Element) => void;
  disconnect() {}
}

describe('segSlide', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('follows the selected item when a sibling resizes but the full-width row does not', () => {
    vi.stubGlobal('ResizeObserver', FakeResizeObserver);
    const row = document.createElement('nav');
    row.innerHTML = '<a>Installed 9</a><a class="on">Trash</a>';
    let left = 100;
    Object.defineProperty(row.lastElementChild, 'offsetLeft', { get: () => left });

    const detach = segSlide(row);
    // The first label grows (a count, another language, a late font) and pushes the second along.
    left = 108;
    resize(row.firstElementChild!);

    expect(row.style.getPropertyValue('--seg-x')).toBe('108px');
    detach?.();
  });
});
