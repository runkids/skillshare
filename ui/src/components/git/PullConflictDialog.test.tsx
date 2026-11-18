import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import PullConflictDialog from './PullConflictDialog';

const version = { deleted: false, content: '', noPreview: true };
const conflict = {
  localHash: 'aaa',
  remoteHash: 'bbb',
  files: ['a.md', 'b.md'].map((path) => ({ path, local: version, remote: version })),
};

describe('Pull conflict dialog', () => {
  it('keeps one side for every file at once', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <I18nProvider>
        <PullConflictDialog conflict={conflict} onCancel={vi.fn()} onConfirm={onConfirm} />
      </I18nProvider>,
    );
    await user.click(screen.getByRole('button', { name: 'Keep all remote' }));
    await user.click(screen.getByRole('button', { name: 'Apply choices and pull' }));
    expect(onConfirm).toHaveBeenCalledWith({
      localHash: 'aaa',
      remoteHash: 'bbb',
      choices: { 'a.md': 'remote', 'b.md': 'remote' },
    });
  });
});
