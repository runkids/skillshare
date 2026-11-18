import { useState } from 'react';
import { mcpCheckApi } from '../../api/mcpCheck';
import type { MCPCheckFinding, MCPCheckReport, MCPCheckServer } from '../../api/mcpCheck';

/** The servers one page lists: global ones without `project`, else one mcp.projects root's. Names repeat across scopes, so each page counts only its own. */
export function serversFor(report?: MCPCheckReport, project?: string): MCPCheckServer[] {
  return (report?.servers ?? []).filter(
    (server) => (server.project || undefined) === (project || undefined),
  );
}

/** What the page shows of a check: errors and warnings. Info findings stay in the CLI. */
export function problemsByServer(
  report?: MCPCheckReport,
  project?: string,
): Record<string, MCPCheckFinding[]> {
  const out: Record<string, MCPCheckFinding[]> = {};
  for (const server of serversFor(report, project)) {
    const problems = server.findings.filter((f) => f.level !== 'info');
    if (problems.length > 0) out[server.name] = problems;
  }
  return out;
}

/** Runs the check on demand only. The result lives in memory, so a reload clears it. */
export function useMCPCheck() {
  const [result, setResult] = useState<{
    report?: MCPCheckReport;
    checkedAt?: number;
    error?: string;
  }>({});
  const [running, setRunning] = useState(false);
  const run = async () => {
    setRunning(true);
    try {
      setResult({ report: await mcpCheckApi.run(), checkedAt: Date.now() });
    } catch (e) {
      setResult({ error: (e as Error).message });
    } finally {
      setRunning(false);
    }
  };
  return { ...result, running, run };
}
