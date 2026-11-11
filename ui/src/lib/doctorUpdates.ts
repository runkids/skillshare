import type { DoctorResponse } from '../api/client';

/** Remove cached update notices without hiding unrelated diagnostic warnings. */
export function doctorWithUpdatePolicy(data: DoctorResponse, enabled: boolean): DoctorResponse {
  if (enabled) return data;
  const checks = data.checks.filter((check) => !check.update_available);
  return {
    ...data,
    checks,
    summary: {
      total: checks.length,
      pass: checks.filter((check) => check.status === 'pass').length,
      warnings: checks.filter((check) => check.status === 'warning').length,
      errors: checks.filter((check) => check.status === 'error').length,
      info: checks.filter((check) => check.status === 'info').length,
    },
    version: data.version ? { current: data.version.current, dev_mode: data.version.dev_mode, update_available: false } : undefined,
  };
}
