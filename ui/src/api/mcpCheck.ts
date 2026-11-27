import { http } from './http';
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
export interface MCPCheckServer {
  name: string;
  project?: string;
  ok: boolean;
  findings: MCPCheckFinding[];
  live?: { tools: number; toolNames?: string[]; toolTokens?: number };
}
export interface MCPCheckReport {
  servers: MCPCheckServer[];
  summary: { errors: number; warnings: number };
}
/** Why a probe failed, for the dashboard to phrase; `error` is the raw detail. */
export type MCPProbeErrorKind = 'connect' | 'timeout' | 'auth' | 'protocol' | 'command' | 'unknown';
export interface MCPProbe {
  live?: { tools: number; toolNames?: string[] };
  errorKind?: MCPProbeErrorKind;
  error?: string;
}

/** One server a target loads. `checked` is when a live check last counted it; absent means never, and `tokens` is then 0 only for lack of a count. */
export interface MCPToolCostServer {
  name: string;
  project?: string;
  tools: number;
  tokens: number;
  checked?: string;
}
/** What one MCP target loads in one place: the global list, or the mcp.projects `root`. `tokens` adds up only the `measured` servers. */
export interface MCPToolCostGroup {
  root?: string;
  target: string;
  agent: string;
  tokens: number;
  measured: number;
  servers: MCPToolCostServer[];
}
export interface MCPToolCosts {
  budget: number;
  groups: MCPToolCostGroup[];
}

export const mcpCheckApi = {
  /** Read-only. Findings that are errors still resolve; only a check that cannot run rejects. */
  run: () => http.get<MCPCheckReport>('/mcp/check'),
  /** Starts every server and counts its tools; records the counts `costs` reads. */
  runLive: () => http.get<MCPCheckReport>('/mcp/check?live=1'),
  /** Per MCP target, the tool definitions the synced servers cost, as the last live check counted them. */
  costs: () => http.get<MCPToolCosts>('/mcp/costs'),
  /** Starts the server as the form describes it, saved or not, and asks for its tools. Saves nothing. */
  probe: (mutation: MCPMutation) => http.post<MCPProbe>('/mcp/probe', { mutation }),
};
