import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import Switch from './Switch';

function Controlled() {
  const [on, setOn] = useState(false);
  return <Switch checked={on} onChange={setOn} aria-label="Force" />;
}

describe('Switch', () => {
  afterEach(cleanup);

  it('toggles once for each click, Space and Enter', async () => {
    render(<Controlled />);
    const sw = screen.getByRole('switch', { name: 'Force' });

    await userEvent.click(sw);
    expect(sw.getAttribute('aria-checked')).toBe('true');
    await userEvent.keyboard(' ');
    expect(sw.getAttribute('aria-checked')).toBe('false');
    await userEvent.keyboard('{Enter}');
    expect(sw.getAttribute('aria-checked')).toBe('true');
  });

  it('does not change while disabled', async () => {
    const onChange = vi.fn();
    render(<Switch checked={false} onChange={onChange} disabled aria-label="Force" />);

    await userEvent.click(screen.getByRole('switch'));

    expect(onChange).not.toHaveBeenCalled();
  });

  it('reports a mixed state and turns everything on from it', async () => {
    const onChange = vi.fn();
    render(<Switch checked={false} mixed onChange={onChange} aria-label="Folder" />);
    const sw = screen.getByRole('switch');
    expect(sw.getAttribute('aria-checked')).toBe('mixed');

    await userEvent.click(sw);

    expect(onChange).toHaveBeenCalledWith(true);
  });
});
