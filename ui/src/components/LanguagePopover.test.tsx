import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it } from 'vitest';
import LanguagePopover from './LanguagePopover';
import { I18nProvider, LOCALE_STORAGE_KEY } from '../i18n';

function renderLanguagePopover() {
  return render(
    <I18nProvider>
      <LanguagePopover />
    </I18nProvider>,
  );
}

describe('LanguagePopover', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('persists locale changes and updates visible copy', async () => {
    const user = userEvent.setup();
    renderLanguagePopover();

    await user.click(screen.getByRole('button', { name: 'Language' }));
    await user.click(screen.getByRole('radio', { name: /繁體中文/ }));

    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('zh-TW');
    expect(screen.getByRole('button', { name: '語言' })).toBeInTheDocument();
  });

  it('puts simplified Chinese first for zh-CN users', async () => {
    const user = userEvent.setup();
    localStorage.setItem(LOCALE_STORAGE_KEY, 'zh-CN');
    renderLanguagePopover();

    await user.click(screen.getByRole('button', { name: '语言' }));

    const options = screen.getAllByRole('radio');
    expect(options[0]).toHaveTextContent('简体中文');
  });
});
