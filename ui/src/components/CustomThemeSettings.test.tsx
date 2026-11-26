import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import { I18nProvider } from '../i18n';
import { ThemeProvider } from '../context/ThemeContext';
import { ToastProvider } from './Toast';
import CustomThemeSettings from './CustomThemeSettings';
import { PRESETS, type CustomTheme } from '../lib/customTheme';

function mount(theme: CustomTheme) {
  localStorage.setItem('skillshare-style', 'custom');
  localStorage.setItem('skillshare-custom-theme', JSON.stringify(theme));
  render(
    <I18nProvider>
      <ToastProvider>
        <MemoryRouter>
          <ThemeProvider>
            <CustomThemeSettings />
          </ThemeProvider>
        </MemoryRouter>
      </ToastProvider>
    </I18nProvider>,
  );
}
const saved = () => JSON.parse(localStorage.getItem('skillshare-custom-theme') ?? 'null');

async function typeAccent(text: string) {
  const field = screen.getByLabelText('Accent hex color');
  await userEvent.clear(field);
  await userEvent.type(field, text);
}

describe('CustomThemeSettings', () => {
  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it('restores the preset when it is picked again', async () => {
    mount({ ...PRESETS.graphite, from: 'graphite', corners: 'round' });
    await userEvent.click(screen.getByRole('radio', { name: /^Graphite/ }));
    expect(saved()).toEqual({ ...PRESETS.graphite, from: 'graphite' });
  });

  it('takes a six-digit accent typed one key at a time', async () => {
    mount(PRESETS.pine);
    await typeAccent('#4f46e5');
    expect(saved().accent).toBe('#4f46e5');
  });

  it('expands a short accent when the field is left', async () => {
    mount(PRESETS.pine);
    await typeAccent('#4f4');
    await userEvent.tab();
    expect(saved().accent).toBe('#44ff44');
  });

  it("drops a preset's dark accent when the accent changes after another edit", async () => {
    mount({ ...PRESETS.indigo, corners: 'round' });
    await typeAccent('#be123c');
    expect(saved().accentDark).toBeUndefined();
  });

  it('keeps the dark accent the user set when the accent changes', async () => {
    mount({ ...PRESETS.pine, corners: 'round', accentDark: '#123456' });
    await typeAccent('#4f46e5');
    expect(saved().accentDark).toBe('#123456');
  });
});
