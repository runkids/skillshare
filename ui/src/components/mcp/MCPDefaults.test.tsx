import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import MCPDefaults from './MCPDefaults';

const renderDefaults = (targets: string[], onSave = vi.fn()) =>
  render(
    <I18nProvider>
      <MCPDefaults targets={targets} offered={['claude', 'opencode', 'pi']} onSave={onSave} />
    </I18nProvider>,
  );

describe('MCP defaults', () => {
  it('saves the default targets when a target is ticked', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    renderDefaults(['claude'], onSave);
    await user.click(screen.getByRole('button', { name: 'Choose the default Agents' }));
    await user.click(screen.getByRole('checkbox', { name: 'Pi' }));
    expect(onSave).toHaveBeenCalledWith({ targets: ['claude', 'pi'] });
  });
});
