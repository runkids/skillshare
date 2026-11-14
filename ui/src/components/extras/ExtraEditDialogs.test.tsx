import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import { api, type Extra } from '../../api/client';
import { EditExtraDialog } from './ExtraEditDialogs';

vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return { ...actual, api: { ...actual.api, editExtra: vi.fn() } };
});

const remote: Extra = {
  name: 'remote-rules', remote: 'example-org/hub//rules', branch: 'main',
  source_dir: '/fixture/extras/remote-rules', source_type: 'default',
  source_exists: true, file_count: 1, targets: [],
};

describe('remote extra identity protection', () => {
  it('cannot submit a remote rename or source edit', async () => {
    const user = userEvent.setup();
    render(<I18nProvider><ToastProvider><EditExtraDialog extra={remote} sharedDir="/fixture/extras" onClose={vi.fn()} onSaved={vi.fn()} /></ToastProvider></I18nProvider>);
    expect(screen.getByLabelText('Name')).toBeDisabled();
    expect(screen.getByRole('combobox')).toBeDisabled();
    const save = screen.getByRole('button', { name: 'Save' });
    expect(save).toBeDisabled();
    await user.click(save);
    expect(api.editExtra).not.toHaveBeenCalled();
  });
});
