import type { DoctorCheck, DoctorResponse } from '../api/client';

/** Remove cached update notices without hiding unrelated diagnostic warnings. */
export function doctorWithUpdatePolicy(data: DoctorResponse, enabled: boolean): DoctorResponse {
  if (enabled) return data;
  const checks = data.checks.filter((check) => !check.update_available);
  return {
    ...data,
    ...doctorFromChecks(checks),
    version: data.version ? { current: data.version.current, dev_mode: data.version.dev_mode, update_available: false } : undefined,
  };
}

/** A result for the checks found so far, counted the way doctor counts them. */
export function doctorFromChecks(checks: DoctorCheck[]): DoctorResponse {
  return {
    checks,
    summary: {
      total: checks.length,
      pass: checks.filter((check) => check.status === 'pass').length,
      warnings: checks.filter((check) => check.status === 'warning').length,
      errors: checks.filter((check) => check.status === 'error').length,
      info: checks.filter((check) => check.status === 'info').length,
    },
  };
}

/** Adds a streamed check; a slow check streams as pending first and its result replaces that row in place. */
export function withStreamedCheck(checks: DoctorCheck[], check: DoctorCheck): DoctorCheck[] {
  const i = checks.findIndex((c) => c.status === 'pending' && c.name === check.name);
  return i === -1 ? [...checks, check] : checks.map((c, j) => (j === i ? check : c));
}
