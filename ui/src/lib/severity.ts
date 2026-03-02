// Shared severity color/badge helpers used by AuditPage and AuditRulesPage.

export function severityColor(sev: string): string {
  switch (sev) {
    case 'CRITICAL': return 'var(--color-danger)';
    case 'HIGH': return 'var(--color-warning)';
    case 'MEDIUM': return 'var(--color-blue)';
    case 'LOW': return 'var(--color-success)';
    case 'INFO': return 'var(--color-pencil-light)';
    default: return 'var(--color-pencil-light)';
  }
}

export function severityBgColor(sev: string): string {
  switch (sev) {
    case 'CRITICAL': return 'var(--color-danger-light)';
    case 'HIGH': return 'var(--color-warning-light)';
    case 'MEDIUM': return 'var(--color-info-light)';
    case 'LOW': return 'var(--color-success-light)';
    case 'INFO': return 'transparent';
    default: return 'transparent';
  }
}

export function severityBadgeVariant(sev: string): 'danger' | 'warning' | 'info' | 'default' {
  switch (sev) {
    case 'CRITICAL': return 'danger';
    case 'HIGH': return 'warning';
    case 'MEDIUM': return 'info';
    case 'LOW': return 'info';
    case 'INFO': return 'default';
    default: return 'default';
  }
}
