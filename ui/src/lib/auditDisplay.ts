import type { AuditAllResponse, AuditFinding } from '../api/client';
import { plural } from '../i18n/locales';

export const AUDIT_SEVERITIES: AuditFinding['severity'][] = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'];
export const auditSeverityRank = (severity: AuditFinding['severity']) => AUDIT_SEVERITIES.indexOf(severity);

// Selection always reveals matching findings; with no selection, keep the minor expander.
export function displayAuditFindings(findings: AuditFinding[], severity: AuditFinding['severity'] | null, minimum: boolean, expanded = true): AuditFinding[] {
  return findings.filter((f) => severity
   ? minimum ? auditSeverityRank(f.severity) <= auditSeverityRank(severity) : f.severity === severity
   : expanded || auditSeverityRank(f.severity) <= auditSeverityRank('MEDIUM'));
}

// The source-link note for a scan, so every view words a partial scan the same way.
export function auditSourceLinkNote(res: Pick<AuditAllResponse, 'warnings' | 'incomplete'>): { key: string; count: number } | null {
  const count = res.warnings?.length ?? 0;
  if (count === 0) return null;
  return { key: plural(res.incomplete ? 'audit.sourceLinksIncomplete' : 'audit.sourceLinks', count), count };
}
