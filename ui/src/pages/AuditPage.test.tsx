import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { expect, it, vi } from 'vitest';
import userEvent from '@testing-library/user-event';
import type { AuditAllResponse } from '../api/client';
import { I18nProvider } from '../i18n';
import { queryKeys } from '../lib/queryKeys';
import { ToastProvider } from '../components/Toast';
import AuditPage from './AuditPage';

vi.mock('../hooks/useSharedQueries', () => ({ useOverviewQuery: () => ({ data: {} }) }));

it('keeps the severity count in the selector accessible name', () => {
  const queryClient = new QueryClient();
  const result: AuditAllResponse = {
    results: [{
      skillName: 'demo',
      findings: [{ severity: 'CRITICAL', pattern: 'test', message: 'test', file: 'SKILL.md', line: 1, snippet: '' }],
      riskScore: 0,
      riskLabel: 'low',
      threshold: 'HIGH',
      isBlocked: false,
    }],
    summary: {
      total: 1,
      passed: 0,
      warning: 1,
      failed: 0,
      critical: 1,
      high: 0,
      medium: 0,
      low: 0,
      info: 0,
      threshold: 'HIGH',
      riskScore: 0,
      riskLabel: 'low',
    },
  };
  queryClient.setQueryData(queryKeys.audit.all('skills'), result);

  const { unmount } = render(
    <MemoryRouter>
      <QueryClientProvider client={queryClient}>
        <I18nProvider>
          <ToastProvider><AuditPage /></ToastProvider>
        </I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );

  expect(screen.getByRole('button', { name: '1 CRITICAL' })).toBeInTheDocument();
  unmount();
  localStorage.setItem('skillshare:locale', 'de');
  render(<MemoryRouter><QueryClientProvider client={queryClient}><I18nProvider><ToastProvider><AuditPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
  expect(screen.getByRole('button', { name: '1 Kritisch' })).toBeInTheDocument();
  localStorage.removeItem('skillshare:locale');
});


it('retains a blocked resource when the selected floor hides its findings', async () => {
  Element.prototype.scrollIntoView = vi.fn();
  const queryClient = new QueryClient();
  const result: AuditAllResponse = {
    results: [{ skillName: 'blocked-low', findings: [{ severity: 'LOW', pattern: 'low', message: 'hidden low', file: 'SKILL.md', line: 1, snippet: '' }], riskScore: 1, riskLabel: 'low', threshold: 'LOW', isBlocked: true },
      { skillName: 'critical', findings: [{ severity: 'CRITICAL', pattern: 'critical', message: 'visible critical', file: 'SKILL.md', line: 1, snippet: '' }], riskScore: 90, riskLabel: 'critical', threshold: 'LOW', isBlocked: true }],
    summary: { total: 2, passed: 0, warning: 0, failed: 2, critical: 1, high: 0, medium: 0, low: 1, info: 0, threshold: 'LOW', riskScore: 45, riskLabel: 'medium' },
  };
  queryClient.setQueryData(queryKeys.audit.all('skills'), result);
  render(<MemoryRouter><QueryClientProvider client={queryClient}><I18nProvider><ToastProvider><AuditPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
  await userEvent.click(screen.getByRole('combobox', { name: 'Severity filter' }));
  await userEvent.click(screen.getByRole('option', { name: 'Minimum severity' }));
  await userEvent.click(screen.getByRole('button', { name: '1 CRITICAL' }));
  expect(screen.getByText('blocked-low')).toBeInTheDocument();
  expect(screen.queryByText('hidden low')).not.toBeInTheDocument();

});

it('shows file coverage for a clean resource as well as the summary', () => {
  const queryClient = new QueryClient();
  queryClient.setQueryData(queryKeys.audit.all('skills'), {
    results: [{ skillName: 'partial', findings: [], skippedFiles: 2, binaryFiles: 1, oversizedFiles: 1, riskScore: 0, riskLabel: 'clean', threshold: 'HIGH', isBlocked: false }],
    summary: { total: 1, passed: 1, warning: 0, failed: 0, critical: 0, high: 0, medium: 0, low: 0, info: 0, threshold: 'HIGH', riskScore: 0, riskLabel: 'clean', skippedFiles: 2, binaryFiles: 1, oversizedFiles: 1 },
  });
  render(<MemoryRouter><QueryClientProvider client={queryClient}><I18nProvider><ToastProvider><AuditPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
  expect(screen.getByText('partial')).toBeInTheDocument();
  expect(screen.getAllByText(/2.*1.*1/).length).toBeGreaterThanOrEqual(2);
});
