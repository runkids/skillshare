import { useMemo } from 'react';
import { AlertTriangle, Lightbulb } from 'lucide-react';
import { fieldDocs } from '../../lib/fieldDocs';
import { isMacOS } from '../../hooks/useGlobalShortcuts';
import Badge from '../Badge';

/** Show idle hints when tree has this few nodes or fewer */
const IDLE_HINT_THRESHOLD = 3;

interface TreeNode {
  key: string;
  value: string;
  line: number;
  depth: number;
  children: TreeNode[];
}

interface StructureTreeProps {
  source: string;
  cursorLine: number;
  parseError: boolean;
  onClickNode: (line: number) => void;
}

/** Parse YAML source by indentation into a flat list of nodes.
 *  Handles both plain keys ("key: value") and list item keys ("- key: value"). */
function parseYamlTree(source: string): TreeNode[] {
  const lines = source.split('\n');
  const nodes: TreeNode[] = [];

  for (let i = 0; i < lines.length; i++) {
    const rawLine = lines[i];
    const trimmed = rawLine.trim();
    // Skip blank lines and comment lines
    if (!trimmed || trimmed.startsWith('#')) continue;
    // Skip bare list values (- value without colon)
    if ((trimmed.startsWith('- ') || trimmed === '-') && !trimmed.includes(':')) continue;

    let key: string | null = null;
    let rest = '';
    let indent = rawLine.length - rawLine.trimStart().length;

    // Try list item key first: "- key: value"
    const listMatch = trimmed.match(/^-\s+([a-zA-Z_][\w.-]*)\s*:(.*)/);
    if (listMatch) {
      key = listMatch[1];
      rest = listMatch[2].trim();
      // For depth calc, use indent of the dash
    } else {
      // Plain key: "key: value"
      const plainMatch = trimmed.match(/^([a-zA-Z_][\w.-]*)\s*:(.*)/);
      if (plainMatch) {
        key = plainMatch[1];
        rest = plainMatch[2].trim();
      }
    }

    if (!key) continue;

    const depth = Math.floor(indent / 2);
    nodes.push({ key, value: rest, line: i + 1, depth, children: [] });
  }

  return nodes;
}

export default function StructureTree({
  source,
  cursorLine,
  parseError,
  onClickNode,
}: StructureTreeProps) {
  // useMemo must be called before any early return (rules of hooks)
  const nodes = useMemo(() => parseYamlTree(source), [source]);
  const modKey = isMacOS() ? '⌘' : 'Ctrl+';

  if (parseError) {
    return (
      <div className="flex items-center gap-2 px-3 py-4 text-sm text-warning">
        <AlertTriangle size={14} strokeWidth={2} />
        <span>Fix syntax errors to see structure</span>
      </div>
    );
  }

  if (nodes.length === 0) {
    // Show available top-level fields as a quick reference
    const topLevelFields = Object.entries(fieldDocs).filter(([key]) => !key.includes('.'));
    return (
      <div className="px-3 py-3 space-y-3 animate-fade-in">
        <div className="flex items-center gap-1.5 text-pencil-light">
          <Lightbulb size={13} strokeWidth={2} />
          <span className="text-xs font-medium uppercase tracking-wider">Available fields</span>
        </div>
        <div className="space-y-0.5">
          {topLevelFields.map(([key, doc]) => (
            <div key={key} className="flex items-start gap-2 px-2 py-1.5 rounded text-xs hover:bg-muted/30 transition-colors">
              <code className="font-mono font-semibold text-pencil shrink-0">{key}</code>
              <span className="text-pencil-light truncate">{doc.description.split('.')[0]}</span>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      <div role="tree" aria-label="YAML structure" className="py-1 overflow-x-auto">
        {nodes.map((node, i) => (
          <TreeNodeItem
            key={i}
            node={node}
            cursorLine={cursorLine}
            onClickNode={onClickNode}
          />
        ))}
      </div>

      {nodes.length <= IDLE_HINT_THRESHOLD && (
        <div className="flex-1 flex flex-col items-center justify-center p-4 select-none animate-fade-in">
          <div className="ss-panel-idle text-center space-y-2.5 max-w-[180px]">
            <Lightbulb size={28} strokeWidth={1.2} className="text-muted-dark/30 mx-auto" />
            <p className="text-xs text-pencil-light leading-relaxed">
              Click fields in the editor for contextual docs
            </p>
            <div className="flex items-center justify-center gap-1.5 text-[10px] text-muted-dark">
              <kbd className="font-mono bg-muted px-1.5 py-0.5 rounded text-pencil-light border border-muted-dark/30">{modKey}B</kbd>
              <span>toggle panel</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function TreeNodeItem({
  node,
  cursorLine,
  onClickNode,
}: {
  node: TreeNode;
  cursorLine: number;
  onClickNode: (line: number) => void;
}) {
  const isActive = node.line === cursorLine;
  const paddingLeft = node.depth * 16;

  return (
    <button
      role="treeitem"
      type="button"
      aria-selected={isActive}
      onClick={() => onClickNode(node.line)}
      style={{ paddingLeft: `${paddingLeft + 12}px` }}
      className={`ss-tree-node w-full text-left flex items-center gap-2 pr-3 py-1 text-sm transition-colors duration-150 cursor-pointer ${
        isActive
          ? 'bg-info-light border-l-2 border-blue text-blue font-medium'
          : 'text-pencil hover:bg-muted/30 border-l-2 border-transparent'
      }`}
    >
      {/* Depth dot indicator */}
      {node.depth > 0 && (
        <span className={`w-1 h-1 rounded-full flex-shrink-0 ${isActive ? 'bg-blue' : 'bg-muted-dark/40'}`} />
      )}
      <span className="font-mono font-semibold text-sm truncate max-w-[120px]">{node.key}</span>
      {node.value && (
        <>
          <span className={`text-muted-dark ${isActive ? 'text-blue/60' : ''}`}>:</span>
          <span
            className={`font-mono text-xs truncate flex-1 ${
              isActive ? 'text-blue/80' : 'text-pencil-light'
            }`}
          >
            {node.value}
          </span>
        </>
      )}
      <span className="flex-shrink-0 ml-auto">
        <Badge variant="default" size="sm">
          L:{node.line}
        </Badge>
      </span>
    </button>
  );
}
