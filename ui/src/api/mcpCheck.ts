import { apiFetch } from './client';
import { ApiError, BASE, parseApiErrorPayload } from './http';
import type { MCPMutation } from './mcp';

/** One result of `skillshare mcp check`. `subject` names the variable, command, package, image, or host; never a value. */
export interface MCPCheckFinding {
  level: 'error' | 'warning' | 'info';
  check: string;
  target: string;
  message: string;
  subject?: string;
}
/** `project` is the mcp.projects root a server belongs to; absent for a global server. */
/** `live`: what the server reported when probed, only with `?live=1`. */
export interface MCPToolContract {
  hash: string; inputHash: string; proseHash: string; outputHash: string; metadataHash: string;
  /** Complete bounded canonical JSON descriptor; render as inert text only. */
  definitionText: string;
}
export interface MCPContractReview {
  status: 'untrusted' | 'legacy' | 'unchanged' | 'drifted' | 'accepted' | 'error';
  revision?: string; policy?: string; identityChanged?: boolean;
  tools?: Record<string, MCPToolContract> | null;
  changes?: { tool: string; area: string; path: string; grade: string; message: string }[];
  audit?: { tool: string; path: string; ruleId: string; severity: string; message: string }[];
  omitted?: number;
}
export interface MCPCheckServer { name: string; project?: string; ok: boolean; findings: MCPCheckFinding[]; live?: { tools: number; toolNames?: string[] }; contract?: MCPContractReview }
export interface MCPCheckReport { toolsAcceptance?: 'accepted' | 'stale' | 'error'; servers: MCPCheckServer[]; summary: { errors: number; warnings: number } }
/** Why a probe failed, for the dashboard to phrase; `error` is the raw detail. */
export type MCPProbeErrorKind = 'connect' | 'timeout' | 'auth' | 'protocol' | 'command' | 'unknown';
export interface MCPProbe { live?: { tools: number; toolNames?: string[] }; errorKind?: MCPProbeErrorKind; error?: string }

export interface MCPToolsAcceptance { accepted: boolean; report: MCPCheckReport | null; error?: string }
export interface MCPToolsApproval { name: string; root: string; revision: string }

/** A refused acceptance remains an HTTP error while retaining its current report. */
export class MCPToolsAcceptanceError extends ApiError {
  report: MCPCheckReport | null;
  constructor(status: number, message: string, report: MCPCheckReport | null, code?: string) {
    super(status, message, { code });
    this.report = report;
  }
}

function acceptanceReport(value: unknown): MCPCheckReport | null {
  if (!value || typeof value !== 'object') return null;
  const report = value as Partial<MCPCheckReport>;
  return Array.isArray(report.servers) && typeof report.summary?.errors === 'number' && typeof report.summary?.warnings === 'number'
    ? report as MCPCheckReport : null;
}

// apiFetch deliberately discards non-success bodies. This endpoint needs its
// typed refusal report, using the shared URL and error-envelope parser.
async function acceptTools(approval: MCPToolsApproval): Promise<MCPToolsAcceptance> {
  let response: Response;
  let raw: string;
  try {
    response = await fetch(BASE + '/mcp/check', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(approval) });
    raw = await response.text();
  } catch {
    throw new MCPToolsAcceptanceError(0, 'MCP acceptance outcome is unknown; probe again before accepting', null, 'connection_lost');
  }
  let data: Partial<MCPToolsAcceptance> | null;
  try { data = JSON.parse(raw); } catch {
    throw new MCPToolsAcceptanceError(response.status, 'invalid MCP acceptance response', null, 'invalid_json');
  }
  const report = acceptanceReport(data?.report);
  if (!response.ok) {
    const parsed = parseApiErrorPayload(data, response.status, response.statusText);
    throw new MCPToolsAcceptanceError(response.status, parsed.message, report, parsed.code);
  }
  if (typeof data?.accepted !== 'boolean' || data.accepted && !report) {
    throw new MCPToolsAcceptanceError(response.status, 'invalid MCP acceptance response', null, 'invalid_json');
  }
  return { accepted: data.accepted, report, error: typeof data.error === 'string' ? data.error : undefined };
}

export const mcpCheckApi = {
  /** Read-only. Findings that are errors still resolve; only a check that cannot run rejects. */
  run: () => apiFetch<MCPCheckReport>('/mcp/check'),
  /** Explicitly requested live observation. An empty root selects this config's list. */
  review: (root?: string) => {
    const query = new URLSearchParams({ live: '1' });
    if (root !== undefined) query.set('root', root);
    return apiFetch<MCPCheckReport>(`/mcp/check?${query}`);
  },
  accept: acceptTools,
  /** Starts the server as the form describes it, saved or not, and asks for its tools. Saves nothing. */
  probe: (mutation: MCPMutation) => apiFetch<MCPProbe>('/mcp/probe', { method: 'POST', body: JSON.stringify({ mutation }) }),
};
