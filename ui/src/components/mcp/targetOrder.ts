import { createContext } from 'react';
import { mcpTargets } from '../../api/mcp';

/**
 * Every MCP target in display order. The MCP page appends the accounts the config declares
 * (a target that is another config folder of an Agent); a project has none, since every
 * account of an Agent reads the same project files.
 */
export const MCPTargetOrder = createContext<readonly string[]>(mcpTargets);

/** The Agent each account target writes to, by account name; empty in a project. */
export const MCPAccountAgents = createContext<Readonly<Record<string, string>>>({});
