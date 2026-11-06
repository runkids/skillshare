import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { I18nProvider } from '../../i18n';
import AddProjectDialog from './AddProjectDialog';

vi.mock('../../api/client', async (load) => ({ ...await load<typeof import('../../api/client')>(), api: { saveProject: vi.fn() } }));

vi.mock('./ProjectTools', () => ({ default: ({ onChange }: { onChange: (next: string[]) => void }) => <button type="button" onClick={() => onChange(['claude'])}>pick claude</button> }));

const tools = [{ name: 'claude', skillsPath: '.claude/skills', agentsPath: '' }];

const storagePaths = { shared: '/home/me/.config/skillshare/config.yaml', local: '/home/me/.local/state/skillshare/projects/a.yaml' };

const open = (defaultStorage?: 'shared' | 'local') => render(
  <I18nProvider><AddProjectDialog tools={tools} common={['claude']} defaultStorage={defaultStorage} storagePaths={storagePaths} existing={[]} onClose={() => {}} onAdded={() => {}} /></I18nProvider>,
);

const submit = async () => {
  await userEvent.type(screen.getByLabelText('Project folder'), '~/work/docs');
  await userEvent.click(screen.getByRole('button', { name: 'pick claude' }));
  await userEvent.click(screen.getByRole('button', { name: 'Add project' }));
};

beforeEach(() => {
  Element.prototype.scrollIntoView = vi.fn();
  vi.clearAllMocks();
  vi.mocked(api.saveProject).mockResolvedValue({ success: true, root: '~/work/docs', origin: 'shared', originPath: '' });
});

describe('Add project dialog', () => {
  it('starts on this machine when the server defaults to local', async () => {
    open('local');
    await submit();
    await waitFor(() => expect(api.saveProject).toHaveBeenCalledWith(expect.objectContaining({ create: true, storage: 'local' })));
  });

  it('saves in config.yaml by default', async () => {
    open();
    await submit();
    await waitFor(() => expect(api.saveProject).toHaveBeenCalledWith(expect.objectContaining({ storage: 'shared' })));
  });

  it('saves on this machine only after picking it in the Saved to menu', async () => {
    open();
    await userEvent.click(screen.getByRole('combobox', { name: 'Saved to' }));
    await userEvent.click(screen.getByRole('option', { name: /This machine only/ }));
    await submit();
    await waitFor(() => expect(api.saveProject).toHaveBeenCalledWith(expect.objectContaining({ storage: 'local' })));
  });
});
