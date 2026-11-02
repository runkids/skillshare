import type { AuditFinding } from '../api/client';

export const AUDIT_SEVERITIES: AuditFinding['severity'][] = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'];
export const auditSeverityRank = (severity: AuditFinding['severity']) => AUDIT_SEVERITIES.indexOf(severity);

// Selection always reveals matching findings; with no selection, keep the minor expander.
export function displayAuditFindings(findings: AuditFinding[], severity: AuditFinding['severity'] | null, minimum: boolean, expanded = true): AuditFinding[] {
  return findings.filter((f) => severity
   ? minimum ? auditSeverityRank(f.severity) <= auditSeverityRank(severity) : f.severity === severity
   : expanded || auditSeverityRank(f.severity) <= auditSeverityRank('MEDIUM'));
}
