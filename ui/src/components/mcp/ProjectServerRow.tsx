import type { ReactNode } from 'react';
import type { MCPServer } from '../../api/mcp';

import { describeEndpoint } from './mcpView';
import { AgentStack } from './TargetPicker';
import Switch from '../Switch';

interface Props {
  name: string;
  server: MCPServer;
  /** Tags after the name, such as off or pending. */
  tags?: ReactNode;
  /** Short text before the logos, such as the other projects that use it. */
  aside?: string;
  /** Agent logos that say where the switch applies, and their label. */
  icons: string[];
  iconsLabel: string;
  checked: boolean;
  disabled: boolean;
  onToggle: () => void;
  /** A button before the switch, such as one that edits what this project changes. */
  action?: ReactNode;
  /** Notes under the row; the switch is described by them. */
  children?: ReactNode;
}

// Logos shown before the rest fold into a count, so ten Agents keep the row on one line.
const STACK = 3;

/** A server defined on the MCP page, with a switch for one project. */
export default function ProjectServerRow({
  name,
  server,
  tags,
  aside,
  icons,
  iconsLabel,
  checked,
  disabled,
  onToggle,
  action,
  children,
}: Props) {
  const noteId = `mcp-sw-${name}`;
  return (
    <div className="ss-r !items-stretch !flex-col !gap-2 !py-3">
      <span className="flex min-w-0 items-center gap-2">
        <span title={name} className="max-w-[45%] shrink-0 truncate font-mono font-semibold">
          {name}
        </span>
        {tags}
        <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3">
          {describeEndpoint(server)}
        </span>
        {aside && (
          <span title={aside} className="min-w-0 max-w-[30%] truncate text-xs text-ink-3">
            {aside}
          </span>
        )}
        {icons.length > 0 && (
          <span className="ss-stack" role="img" aria-label={iconsLabel} title={iconsLabel}>
            <AgentStack targets={icons} max={STACK} />
          </span>
        )}
        {action}
        <Switch
          checked={checked}
          aria-label={name}
          aria-describedby={noteId}
          disabled={disabled}
          onChange={onToggle}
        />
      </span>
      <span id={noteId} className="flex flex-col text-xs empty:hidden">
        {children}
      </span>
    </div>
  );
}
