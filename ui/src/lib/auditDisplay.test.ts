import { expect, it } from 'vitest';
import type { AuditFinding } from '../api/client';
import { auditSourceLinkNote, displayAuditFindings } from './auditDisplay';

const levels: AuditFinding['severity'][] = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'];
const findings = levels.map((severity) => ({ severity, pattern: severity, message: severity, file: 'SKILL.md', line: 1, snippet: '' }));

it('selects inclusive floors and exact severities without changing full findings', () => {
  const original = structuredClone(findings);
  for (const [i, severity] of levels.entries()) {
   expect(displayAuditFindings(findings, severity, true)).toEqual(findings.slice(0, i + 1));
   expect(displayAuditFindings(findings, severity, false)).toEqual([findings[i]]);
  }
  expect(findings).toEqual(original);
});

it('preserves minor collapsing unless expanded or a severity is selected', () => {
  expect(displayAuditFindings(findings, null, false, false)).toEqual(findings.slice(0, 3));
  expect(displayAuditFindings(findings, null, false, true)).toEqual(findings);
  expect(displayAuditFindings(findings, 'LOW', true, false)).toEqual(findings.slice(0, 4));
  expect(displayAuditFindings(findings, 'INFO', false, false)).toEqual([findings[4]]);
});

it('handles all-hidden and empty findings', () => {
  expect(displayAuditFindings(findings.slice(3), 'HIGH', true)).toEqual([]);
  expect(displayAuditFindings([], 'HIGH', true)).toEqual([]);
});

it('words a scan by whether a source link could not be read', () => {
  expect(auditSourceLinkNote({ warnings: ['a', 'b'], incomplete: true })).toEqual({ key: 'audit.sourceLinksIncomplete.other', count: 2 });
  expect(auditSourceLinkNote({ warnings: ['a'] })).toEqual({ key: 'audit.sourceLinks.one', count: 1 });
  expect(auditSourceLinkNote({})).toBeNull();
});
