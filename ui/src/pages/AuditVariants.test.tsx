import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import type { AuditAllResponse, Overview } from '../api/client';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import { queryKeys } from '../lib/queryKeys';
import AuditPage from './AuditPage';
import AuditRulesPage from './AuditRulesPage';

vi.mock('../api/client', async (load) => ({
  ...await load<typeof import('../api/client')>(),
  api: { getOverview: vi.fn(), getCompiledRules: vi.fn() },
}));

const scan: AuditAllResponse = {
  results: [{ skillName: '_repo/skills/codex/fix', findings: [{ severity: 'HIGH', pattern: 'credential-access', message: 'Variant finding', file: 'SKILL.md', line: 1, snippet: '', ruleId: 'test-rule' }], riskScore: 30, riskLabel: 'high', threshold: 'HIGH', isBlocked: true }],
  summary: { total: 2, passed: 1, warning: 0, failed: 1, critical: 0, high: 1, medium: 0, low: 0, info: 0, threshold: 'HIGH', riskScore: 30, riskLabel: 'high' },
};

function renderPage(page: React.ReactNode, variants: number) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  client.setQueryData(queryKeys.overview, { skillCount: 1, variantCount: variants, agentCount: 0 } as unknown as Overview);
  client.setQueryData(queryKeys.audit.all('skills'), scan);
  client.setQueryData(queryKeys.audit.compiled, {
    rules: [{ id: 'test-rule', severity: 'HIGH', pattern: 'credential-access', message: 'Test rule', regex: 'secret', enabled: true, source: 'builtin' }], patterns: [], profile: 'default',
  });
  render(<MemoryRouter><QueryClientProvider client={client}><I18nProvider><ToastProvider>{page}</ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
}

describe('audit cache with logical skills and physical variants', () => {
  it('keeps the scan visible on AuditPage', () => {
    renderPage(<AuditPage />, 2);
    expect(screen.getByText('_repo/skills/codex/fix')).toBeInTheDocument();
  });
  it('keeps cached rule matches on AuditRulesPage', async () => {
    const user = userEvent.setup();
    renderPage(<AuditRulesPage />, 2);
    await user.click(screen.getByRole('button', { name: 'Credential access' }));
    expect(screen.getByText('1 now')).toBeInTheDocument();
  });
  it('discards the scan when the physical inventory changes', () => {
    renderPage(<AuditPage />, 3);
    expect(screen.queryByText('_repo/skills/codex/fix')).toBeNull();
  });
});
