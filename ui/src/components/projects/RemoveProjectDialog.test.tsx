import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import RemoveProjectDialog from './RemoveProjectDialog';
import type { ProjectRow } from './projectView';

const project = { root: '~/work/docs', path: '/work/docs', name: 'docs', declared: true, origin: 'local', sharedCopy: true, targets: [], skills: null, agents: null, groups: [], missing: false, hasOwnConfig: false } as ProjectRow;

const open = (p: ProjectRow) => {
  const onConfirm = vi.fn();
  render(<I18nProvider><RemoveProjectDialog project={p} loading={false} onCancel={() => {}} onConfirm={onConfirm} /></I18nProvider>);
  return onConfirm;
};

describe('Remove project dialog', () => {
  it('removes only this machine\'s copy unless config.yaml is also chosen', async () => {
    const onConfirm = open(project);
    await userEvent.click(screen.getByRole('button', { name: 'Remove project' }));
    expect(onConfirm).toHaveBeenLastCalledWith(false);
    await userEvent.click(screen.getByRole('radio', { name: /Also remove it from config.yaml/ }));
    await userEvent.click(screen.getByRole('button', { name: 'Remove project' }));
    expect(onConfirm).toHaveBeenLastCalledWith(true);
  });

  it('warns that removing a shared project affects every machine', () => {
    open({ ...project, origin: 'shared', sharedCopy: false });
    expect(screen.getByText(/every machine that uses it loses it/)).toBeTruthy();
    expect(screen.queryByRole('radio')).toBeNull();
  });
});
