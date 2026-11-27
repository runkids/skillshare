import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import Tabs, { TabButton, TabLink } from './Tabs';

/** A tab row with the given button selected. `handoff` outlives a test, so keys are unique. */
const row = (selected: number, slideKey: string) => (
  <Tabs role="tablist" slideKey={slideKey}>
    {[0, 1].map((i) => (
      <TabButton key={i} on={i === selected} onClick={() => {}}>
        Tab {i}
      </TabButton>
    ))}
  </Tabs>
);

describe('Tabs', () => {
  it('marks only the selected TabLink as the current page', () => {
    render(
      <MemoryRouter>
        <Tabs>
          <TabLink to="/a" on>
            A
          </TabLink>
          <TabLink to="/b" on={false}>
            B
          </TabLink>
        </Tabs>
      </MemoryRouter>,
    );
    const [a, b] = screen.getAllByRole('link');
    expect(a).toHaveClass('on');
    expect(a).toHaveAttribute('aria-current', 'page');
    expect(b).not.toHaveClass('on');
    expect(b).not.toHaveAttribute('aria-current');
  });

  it('renders TabButtons as tabs with their selected state', () => {
    render(row(0, 'roles'));
    const [a, b] = screen.getAllByRole('tab');
    expect(a).toHaveClass('on');
    expect(a).toHaveAttribute('aria-selected', 'true');
    expect(b).not.toHaveClass('on');
    expect(b).toHaveAttribute('aria-selected', 'false');
  });

  it('shows a count only when given one, zero included', () => {
    render(
      <Tabs role="tablist">
        <TabButton on count={0}>
          A
        </TabButton>
        <TabButton on={false}>B</TabButton>
      </Tabs>,
    );
    const [a, b] = screen.getAllByRole('tab');
    expect(a.querySelector('.ss-cnt')).toHaveTextContent('0');
    expect(b.querySelector('.ss-cnt')).toBeNull();
  });

  it('places the underline without sliding on a first mount', () => {
    render(row(0, 'first'));
    expect(screen.getByRole('tablist')).toHaveAttribute('data-slide', 'still');
  });

  it('slides when a keyed row replaces one that had another tab selected', () => {
    render(row(0, 'handoff')).unmount();
    render(row(1, 'handoff'));
    expect(screen.getByRole('tablist')).toHaveAttribute('data-slide', 'move');
  });
});
