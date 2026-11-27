import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import type { HookUnmanaged } from '../../api/hooks';
import HooksUnmanagedNote from './HooksUnmanagedNote';

const inline: HookUnmanaged = {
  target: 'codex',
  path: '/home/u/.codex/config.toml',
  names: ['Stop'],
  importable: false,
};

const show = (entries: HookUnmanaged[]) =>
  render(
    <I18nProvider>
      <HooksUnmanagedNote entries={entries} onImport={vi.fn()} />
    </I18nProvider>,
  );

describe('HooksUnmanagedNote', () => {
  it('names a source Skillshare only lists without offering Review for it', () => {
    show([inline]);
    expect(
      screen.getByText(
        'Codex also loads 1 hook from ~/.codex/config.toml; skillshare leaves it as is',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Left as is' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Review' })).not.toBeInTheDocument();
  });

  it('counts only importable hooks toward Review', () => {
    show([inline, { ...inline, path: '/home/u/.codex/hooks.json', importable: true }]);
    expect(screen.getByText('Codex has 1 hook skillshare does not manage yet')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Not managed yet' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Review' })).toBeInTheDocument();
  });
});
