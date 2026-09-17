import type { LogEntry } from '../api/client';

/* Shared by the Log page and the Dashboard's recent list. */

function asInt(v: unknown): number | undefined {
  if (typeof v === 'number' && Number.isFinite(v)) return Math.trunc(v);
  if (typeof v === 'string') {
    const n = Number.parseInt(v, 10);
    if (Number.isFinite(n)) return n;
  }
  return undefined;
}

function asString(v: unknown): string | undefined {
  if (typeof v === 'string') {
    const s = v.trim();
    return s.length > 0 ? s : undefined;
  }
  if (v == null) return undefined;
  return String(v);
}

function asStringArray(v: unknown): string[] {
  if (Array.isArray(v)) return v.map((it) => String(it).trim()).filter(Boolean);
  const s = asString(v);
  return s ? [s] : [];
}

function summarizeNames(names: string[], limit = 3): string {
  if (names.length <= limit) return names.join(', ');
  return `${names.slice(0, limit).join(', ')} (+${names.length - limit})`;
}

/* ── Detail formatters ─── */

function formatSyncDetail(args: Record<string, any>): string {
  const parts: string[] = [];
  const total = asInt(args.targets_total ?? args.targets);
  if (total != null) parts.push(`targets=${total}`);
  const failed = asInt(args.targets_failed);
  if (failed != null && failed > 0) parts.push(`failed=${failed}`);
  if (args.dry_run === true || args.dry_run === 'true') parts.push('dry-run');
  if (args.force === true || args.force === 'true') parts.push('force');
  const scope = asString(args.scope);
  if (scope) parts.push(`scope=${scope}`);
  return parts.join(', ');
}

function formatAuditDetail(args: Record<string, any>): string {
  const parts: string[] = [];
  const scope = asString(args.scope);
  const name = asString(args.name);
  if (scope === 'single' && name) parts.push(`skill=${name}`);
  else if (scope === 'all') parts.push('all-skills');
  else if (name) parts.push(name);
  const mode = asString(args.mode);
  if (mode) parts.push(`mode=${mode}`);
  const threshold = asString(args.threshold);
  if (threshold) parts.push(`threshold=${threshold.toUpperCase()}`);
  const scanned = asInt(args.scanned);
  if (scanned != null) parts.push(`scanned=${scanned}`);
  const passed = asInt(args.passed);
  if (passed != null) parts.push(`passed=${passed}`);
  const warning = asInt(args.warning);
  if (warning != null && warning > 0) parts.push(`warning=${warning}`);
  const failed = asInt(args.failed);
  if (failed != null && failed > 0) parts.push(`failed=${failed}`);
  const critical = asInt(args.critical) ?? 0;
  const high = asInt(args.high) ?? 0;
  const medium = asInt(args.medium) ?? 0;
  const low = asInt(args.low) ?? 0;
  const info = asInt(args.info) ?? 0;
  // Severity names stay English everywhere in the product, so this phrase needs no translation.
  const findings = ([['critical', critical], ['high', high], ['medium', medium], ['low', low], ['info', info]] as const)
    .filter(([, n]) => n > 0)
    .map(([name, n]) => `${n} ${name}`);
  if (findings.length > 0) parts.push(findings.join(', '));
  const riskScore = asInt(args.risk_score);
  const riskLabel = asString(args.risk_label);
  if (riskScore != null) {
    if (riskLabel) parts.push(`risk=${riskLabel.toUpperCase()}(${riskScore}/100)`);
    else parts.push(`risk=${riskScore}/100`);
  }
  const scanErrors = asInt(args.scan_errors);
  if (scanErrors != null && scanErrors > 0) parts.push(`scan-errors=${scanErrors}`);
  return parts.join(', ');
}

function formatUpdateDetail(args: Record<string, any>): string {
  const parts: string[] = [];
  const mode = asString(args.mode);
  if (mode) parts.push(`mode=${mode}`);
  if (args.all === true || args.all === 'true') parts.push('all');
  const name = asString(args.name);
  if (name) parts.push(name);
  const names = asStringArray(args.names);
  if (names.length > 0) parts.push(summarizeNames(names));
  const threshold = asString(args.threshold);
  if (threshold) parts.push(`threshold=${threshold.toUpperCase()}`);
  if (args.force === true || args.force === 'true') parts.push('force');
  if (args.dry_run === true || args.dry_run === 'true') parts.push('dry-run');
  if (args.skip_audit === true || args.skip_audit === 'true') parts.push('skip-audit');
  if (args.diff === true || args.diff === 'true') parts.push('diff');
  return parts.join(', ');
}

function formatGenericDetail(args: Record<string, any>): string {
  const parts: string[] = [];
  if (args.source) parts.push(String(args.source));
  if (args.name) parts.push(String(args.name));
  if (args.targets) parts.push(`${args.targets} target(s)`);
  if (args.target) parts.push(String(args.target));
  if (args.message) parts.push(String(args.message));
  if (args.summary) parts.push(String(args.summary));
  return parts.join(' ');
}

export function formatLogDetail(entry: LogEntry): string {
  const detail = entry.args
    ? entry.cmd === 'sync'
      ? formatSyncDetail(entry.args)
      : entry.cmd === 'update'
        ? formatUpdateDetail(entry.args)
        : entry.cmd === 'audit'
          ? formatAuditDetail(entry.args)
          : formatGenericDetail(entry.args)
    : '';
  if (entry.msg && detail) return `${detail} — ${entry.msg}`;
  if (entry.msg) return entry.msg;
  return detail;
}
