import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitForElementToBeRemoved } from '@testing-library/react';
import { ToastProvider } from './Toast';
import CopyButton from './CopyButton';

describe('CopyButton', () => {
  const writeText = vi.fn();

  beforeEach(() => {
    writeText.mockReset();
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function renderButton() {
    return render(
      <ToastProvider>
        <CopyButton value="/tmp/skills/example/SKILL.md" />
      </ToastProvider>,
    );
  }

  it('shows copied feedback after a successful clipboard write', async () => {
    writeText.mockResolvedValue(undefined);

    renderButton();
    fireEvent.click(screen.getByRole('button', { name: /copy to clipboard/i }));

    expect(writeText).toHaveBeenCalledWith('/tmp/skills/example/SKILL.md');
    expect(await screen.findByText('Copied!')).toBeInTheDocument();

    await waitForElementToBeRemoved(() => screen.queryByText('Copied!'), { timeout: 2500 });
  });

  it('shows an error toast when clipboard access is denied', async () => {
    writeText.mockRejectedValue(new Error('denied'));

    renderButton();
    fireEvent.click(screen.getByRole('button', { name: /copy to clipboard/i }));

    expect(await screen.findByText('Failed to copy to clipboard.')).toBeInTheDocument();
  });

  it('clears stale copied feedback when a later copy attempt fails', async () => {
    writeText
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error('denied'));

    renderButton();

    const button = screen.getByRole('button', { name: /copy to clipboard/i });
    fireEvent.click(button);
    expect(await screen.findByText('Copied!')).toBeInTheDocument();

    fireEvent.click(button);

    expect(await screen.findByText('Failed to copy to clipboard.')).toBeInTheDocument();
    expect(screen.queryByText('Copied!')).not.toBeInTheDocument();
  });
});
