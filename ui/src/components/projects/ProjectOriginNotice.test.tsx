import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { I18nProvider } from '../../i18n';
import ProjectOriginNotice from './ProjectOriginNotice';

describe('ProjectOriginNotice', () => {
  it('warns that editing a shared registration creates a local override', () => {
    render(<I18nProvider><ProjectOriginNotice origin="shared" originPath="/tmp/config.yaml" /></I18nProvider>);
    expect(screen.getByText(/creates a local override/i)).toBeTruthy();
    expect(screen.getByText('/tmp/config.yaml')).toBeTruthy();
  });
});
