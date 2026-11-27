import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import DialogHeader from './DialogHeader';

describe('DialogHeader', () => {
  afterEach(cleanup);

  it('cannot be closed while closeDisabled', async () => {
    const onClose = vi.fn();
    render(
      <I18nProvider>
        <DialogHeader title="Restore" onClose={onClose} closeDisabled />
      </I18nProvider>,
    );

    await userEvent.click(screen.getByRole('button', { name: 'Close' }));

    expect(onClose).not.toHaveBeenCalled();
  });
});
