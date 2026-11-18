export interface UpdateResultItem {
  name: string;
  kind?: 'skill' | 'agent';
  action: string; // "updated", "up-to-date", "skipped", "error", "blocked"
  message?: string;
  isRepo: boolean;
  auditRiskScore?: number;
  auditRiskLabel?: string;
}

export interface UpdateStreamSummary {
  updated: number;
  upToDate: number;
  blocked: number;
  errors: number;
  skipped: number;
}
