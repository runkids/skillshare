import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import AddFileDialog from './AddFileDialog';

vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return { ...actual, api: { ...actual.api, addTargetFile: vi.fn() } };
});

const renderDialog = (onAdded = vi.fn()) => {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <I18nProvider>
        <ToastProvider>
          <AddFileDialog
            target="pi"
            root="/home/me/.pi/agent"
            onClose={vi.fn()}
            onAdded={onAdded}
          />
        </ToastProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  return onAdded;
};

describe('Add file dialog', () => {
  beforeEach(() => vi.clearAllMocks());

  it('adds a file and opens its tab', async () => {
    vi.mocked(api.addTargetFile).mockResolvedValue({
      target: 'pi',
      project: false,
      root: '/home/me/.pi/agent',
      files: [
        {
          path: 'SYSTEM.md',
          abs: '/home/me/.pi/agent/SYSTEM.md',
          builtin: false,
          exists: false,
          size: 0,
        },
      ],
    });
    const user = userEvent.setup();
    const onAdded = renderDialog();
    await user.type(screen.getByLabelText('File name'), 'SYSTEM.md');
    await user.click(screen.getByRole('button', { name: 'Add' }));
    await waitFor(() => expect(onAdded).toHaveBeenCalledWith('SYSTEM.md'));
    expect(api.addTargetFile).toHaveBeenCalledWith('pi', 'SYSTEM.md');
  });
});
