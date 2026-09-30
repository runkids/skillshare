import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it } from 'vitest';
import { RailRow } from './StatusRail';

it('names a rail row by its target and says whether its details are open', async () => {
  const user = userEvent.setup();
  render(<RailRow target="claude" label="Claude" path="~/.claude/settings.json" detail={<span>Loads on start</span>} />);
  const row = screen.getByRole('button', { name: 'Claude' });
  expect(row).toHaveAttribute('aria-expanded', 'false');
  await user.click(row);
  expect(row).toHaveAttribute('aria-expanded', 'true');
});
