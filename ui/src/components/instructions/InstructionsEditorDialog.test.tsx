import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import InstructionsEditorDialog from './InstructionsEditorDialog';
import { ApiError } from '../../api/client';

vi.mock('../CodeEditor', () => ({
  default: ({ value, onChange, ariaLabel }: { value: string; onChange: (v: string) => void; ariaLabel: string }) => <textarea aria-label={ariaLabel} value={value} onChange={(e) => onChange(e.target.value)} />,
}));

const renderDialog = (onSave = vi.fn().mockResolvedValue(undefined), onClose = () => {}) => {
  render(
    <I18nProvider><ToastProvider>
      <InstructionsEditorDialog title="personal" path="/h/personal/AGENTS.md" content={'# Rules\n'} onSave={onSave} onClose={onClose} />
    </ToastProvider></I18nProvider>,
  );
  return onSave;
};

describe('Instructions editor dialog', () => {
  it('preserves a memory draft after conflict and requires confirmation even for the save shortcut', async () => {
    const onSave = vi.fn().mockRejectedValueOnce(new ApiError(409, 'Changed outside', { code: 'memory_conflict' })).mockResolvedValue(undefined);
    const onReadLatest = vi.fn().mockResolvedValue('# Latest external version');
    render(<I18nProvider><ToastProvider><InstructionsEditorDialog title="note.md" path="/memory/note.md" content={'# Original\n'} onSave={onSave} onReadLatest={onReadLatest} onClose={() => {}} /></ToastProvider></I18nProvider>);
    const user = userEvent.setup();
    const input = screen.getByRole('textbox', { name: 'note.md' });
    await user.type(input, 'my draft');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByText('Latest saved version')).toBeInTheDocument();
    expect(input).toHaveValue('# Original\nmy draft');
    fireEvent.keyDown(window, { key: 's', metaKey: true });
    expect(await screen.findByRole('dialog', { name: 'Save my draft' })).toBeInTheDocument();
    expect(onSave).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(input).toHaveValue('# Original\nmy draft');
    await user.click(screen.getByRole('button', { name: 'Save my draft' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
  });
  it('previews the unsaved draft, with HTML comments left out', async () => {
    renderDialog();
    const user = userEvent.setup();

    await user.type(screen.getByRole('textbox', { name: 'personal' }), '<!-- hidden -->{enter}## Draft heading');
    await user.click(screen.getByRole('tab', { name: 'Preview' }));

    expect(screen.getByRole('heading', { name: 'Draft heading' })).toBeInTheDocument();
    expect(screen.queryByText(/hidden/)).not.toBeInTheDocument();
  });

  it('saves the draft with ⌘S while previewing', async () => {
    const onSave = renderDialog();
    const user = userEvent.setup();

    await user.type(screen.getByRole('textbox', { name: 'personal' }), 'more');
    await user.click(screen.getByRole('tab', { name: 'Preview' }));
    fireEvent.keyDown(window, { key: 's', metaKey: true });

    await waitFor(() => expect(onSave).toHaveBeenCalledWith('# Rules\nmore'));
  });

  it('asks before closing with an unsaved edit', async () => {
    renderDialog();
    const user = userEvent.setup();

    await user.type(screen.getByRole('textbox', { name: 'personal' }), 'more');
    await user.click(screen.getByRole('button', { name: 'Close' }));

    expect(screen.getByRole('dialog', { name: 'Unsaved Changes' })).toBeInTheDocument();
  });

  it('closes when the unsaved edit is discarded', async () => {
    const onClose = vi.fn();
    renderDialog(undefined, onClose);
    const user = userEvent.setup();

    await user.type(screen.getByRole('textbox', { name: 'personal' }), 'more');
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: 'Discard' }));

    expect(onClose).toHaveBeenCalledOnce();
  });

  it('does not save with ⌘S while a confirm is open over it', async () => {
    const onSave = renderDialog();
    const user = userEvent.setup();

    await user.type(screen.getByRole('textbox', { name: 'personal' }), 'more');
    await user.click(screen.getByRole('button', { name: 'Close' }));
    fireEvent.keyDown(window, { key: 's', metaKey: true });

    expect(onSave).not.toHaveBeenCalled();
  });
});
