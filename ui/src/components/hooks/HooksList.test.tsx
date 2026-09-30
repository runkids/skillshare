import { fireEvent, render, screen, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import HooksList from './HooksList';

const command = (event: string, bash: string) => ({ events: { [event]: [{ hooks: [{ type: 'command', command: bash }] }] } });

it('lists one line per event and command, naming the targets that run it', () => {
  const entries = { start: { bindings: {
    claude: command('SessionStart', 'echo hi'),
    cursor: { events: { sessionStart: [{ command: 'echo hi' }] } },
    antigravity: command('Stop', 'echo bye'),
  } } };
  render(<I18nProvider><HooksList entries={entries} plan={null} onToggle={vi.fn()} onMenu={vi.fn()} /></I18nProvider>);
  const card = screen.getByRole('article', { name: 'start' });
  const start = within(card).getByText('SessionStart').parentElement!;
  expect(within(card).queryByText('sessionStart')).not.toBeInTheDocument();
  expect(within(start).getByText('Claude')).toBeInTheDocument();
  expect(within(start).getByText('Cursor')).toBeInTheDocument();
  expect(within(start).queryByText('Antigravity')).not.toBeInTheDocument();
});

it('fades the header targets that a hovered line does not run on', () => {
  const entries = { start: { bindings: { claude: command('SessionStart', 'echo hi'), antigravity: command('Stop', 'echo bye') } } };
  render(<I18nProvider><HooksList entries={entries} plan={null} onToggle={vi.fn()} onMenu={vi.fn()} /></I18nProvider>);
  const card = screen.getByRole('article', { name: 'start' });
  fireEvent.mouseEnter(within(card).getByText('SessionStart').parentElement!);
  expect(within(card).getByTitle(/^Antigravity/)).toHaveClass('opacity-25');
});
