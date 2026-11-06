import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import ProjectTools from './ProjectTools';

const tools = ['claude', 'codex', 'cursor'].map((name) => ({ name, skillsPath: `.${name}/skills`, agentsPath: '' }));

beforeEach(() => {
  Element.prototype.scrollIntoView = vi.fn();
});

describe('Project targets picker', () => {
  it('picks the target typed into the search box', async () => {
    const onChange = vi.fn();
    render(<I18nProvider><ProjectTools tools={tools} common={[]} selected={[]} onChange={onChange} /></I18nProvider>);
    await userEvent.click(screen.getByRole('combobox', { name: 'Targets' }));
    await userEvent.type(screen.getByRole('searchbox', { name: 'Search targets' }), 'cod{Enter}');
    expect(onChange).toHaveBeenCalledWith(['codex']);
  });
});
