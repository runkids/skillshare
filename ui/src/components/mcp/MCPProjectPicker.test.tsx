import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import MCPProjectPicker from './MCPProjectPicker';

describe('MCP project picker', () => {
  it("adds a folder under another home rather than ticking this home's folder of that name", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <I18nProvider>
        <MCPProjectPicker roots={['/home/alice/repo']} saved={[]} picked={[]} onChange={onChange} />
      </I18nProvider>,
    );
    await user.click(screen.getByRole('button', { name: /Add folder/ }));
    await user.type(screen.getByLabelText('Project folder'), '/home/bob/repo{Enter}');
    expect(onChange).toHaveBeenCalledWith(['/home/bob/repo']);
  });

  it('waits to open a project while a tick is not saved', () => {
    render(
      <I18nProvider>
        <MCPProjectPicker
          roots={['/work/api', '/work/web']}
          saved={['/work/api']}
          picked={['/work/api', '/work/web']}
          onChange={vi.fn()}
          onOpen={vi.fn()}
        />
      </I18nProvider>,
    );
    for (const button of screen.getAllByRole('button', { name: /MCP tab$/ }))
      expect(button).toBeDisabled();
  });
});
