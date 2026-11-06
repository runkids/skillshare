import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { expect, it, vi } from 'vitest';
import AuditPage from './AuditPage';
import { ToastProvider } from '../components/Toast';

vi.mock('../i18n', async (original) => ({ ...await original<typeof import('../i18n')>(), useT: () => (key: string, values?: Record<string, unknown>) => `${key} ${values?.skipped ?? ''}/${values?.binary ?? ''}/${values?.oversized ?? ''}`, useI18n: () => ({ locale: 'en' }) }));
vi.mock('../hooks/useSharedQueries', () => ({ useOverviewQuery: () => ({ data: { skillCount: 1, agentCount: 0 } }) }));
vi.mock('../lib/auditCache', () => ({ getCachedAuditResult: () => ({ results: [{ skillName: 'partial', findings: [], skippedFiles: 2, binaryFiles: 1, oversizedFiles: 1, riskScore: 0, riskLabel: 'clean', threshold: 'HIGH', isBlocked: false }], summary: { total: 1, passed: 1, warning: 0, failed: 0, critical: 0, high: 0, medium: 0, low: 0, info: 0, threshold: 'HIGH', riskScore: 0, riskLabel: 'clean', skippedFiles: 2, binaryFiles: 1, oversizedFiles: 1 } }) }));

it('shows file coverage for a clean resource as well as the summary', () => {
 const client = new QueryClient();
 render(<MemoryRouter><QueryClientProvider client={client}><ToastProvider><AuditPage /></ToastProvider></QueryClientProvider></MemoryRouter>);
 expect(screen.getByText('partial')).toBeInTheDocument();
 expect(screen.getAllByText(/audit\.coverage.*2\/1\/1/)).toHaveLength(2);
});
