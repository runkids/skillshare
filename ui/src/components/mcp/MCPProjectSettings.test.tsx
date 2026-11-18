import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { ProjectTargets } from './MCPProjectSettings';

const renderTargets = (value: string[] | undefined, onChange = vi.fn()) => {
  render(<I18nProvider><ProjectTargets value={value} defaults={['claude']} offered={['claude', 'opencode', 'pi']} onChange={onChange} /></I18nProvider>);
  return onChange;
};

describe('project targets', () => {
  it('starts the project its own list from the global targets on the first tick', async () => {
    const user = userEvent.setup();
    const onChange = renderTargets(undefined);
    await user.click(screen.getByRole('checkbox', { name: /OpenCode/ }));
    expect(onChange).toHaveBeenCalledWith(['claude', 'opencode']);
  });

  it('builds a second quick tick on the first, before the saved list comes back', async () => {
    const user = userEvent.setup();
    const onChange = renderTargets(undefined);
    await user.click(screen.getByRole('checkbox', { name: /OpenCode/ }));
    await user.click(screen.getByRole('checkbox', { name: /Pi/ }));
    expect(onChange).toHaveBeenLastCalledWith(['claude', 'opencode', 'pi']);
  });

  it('shows the global targets again when the first tick fails to save', async () => {
    const user = userEvent.setup();
    renderTargets(undefined, vi.fn().mockResolvedValue(false));
    await user.click(screen.getByRole('checkbox', { name: /OpenCode/ }));
    expect(screen.getByRole('checkbox', { name: /Claude/ })).toHaveAttribute('aria-checked', 'true');
  });

  it('drops a list saved by the first tick even before it comes back', async () => {
    const user = userEvent.setup();
    const onChange = renderTargets(undefined, vi.fn().mockResolvedValue(true));
    await user.click(screen.getByRole('checkbox', { name: /OpenCode/ }));
    await user.click(screen.getByRole('button', { name: 'Back to global' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Use global targets' }));
    expect(onChange).toHaveBeenLastCalledWith(undefined);
  });

  it("asks before going back to the global targets, which drops the project's own list", async () => {
    const user = userEvent.setup();
    const onChange = renderTargets(['opencode', 'pi']);
    await user.click(screen.getByRole('button', { name: 'Back to global' }));
    expect(onChange).not.toHaveBeenCalled();
  });

  it('follows the global targets once that is confirmed', async () => {
    const user = userEvent.setup();
    const onChange = renderTargets(['opencode', 'pi']);
    await user.click(screen.getByRole('button', { name: 'Back to global' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Use global targets' }));
    expect(onChange).toHaveBeenCalledWith(undefined);
  });
});
