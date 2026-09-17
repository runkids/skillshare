import { useMemo } from 'react';
import { fieldDocs } from '../../lib/fieldDocs';
import { useT } from '../../i18n';

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
    let key: string | null = null;
    let rest = '';
    const indent = rawLine.length - rawLine.trimStart().length;

    // Try list item key first: "- key: value"
    const listMatch = trimmed.match(/^-\s+([a-zA-Z_][\w.-]*)\s*:(.*)/);
    if (listMatch) {
      key = listMatch[1];
      rest = listMatch[2].trim();
    } else {
      // Plain key: "key: value"
      const plainMatch = trimmed.match(/^([a-zA-Z_][\w.-]*)\s*:(.*)/);
      if (plainMatch) {
        key = plainMatch[1];
        rest = plainMatch[2].trim();
      } else {
        // Bare list value: "- agents" (short-form target name, no colon)
        const bareMatch = trimmed.match(/^-\s+([a-zA-Z_][\w.-]+)$/);
        if (bareMatch) {
          key = bareMatch[1];
        }
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
  const t = useT();
  // useMemo must be called before any early return (rules of hooks)
  const nodes = useMemo(() => parseYamlTree(source), [source]);

  if (parseError) return <p className="text-[13px] text-ink-3">{t('config.panel.parseError')}</p>;

  // An empty file still gets a starting point: the keys it could have.
  if (nodes.length === 0) {
    return (
      <div className="flex flex-col gap-2">
        <span className="text-[12px] font-semibold text-ink-3">{t('config.panel.availableFields')}</span>
        <div className="flex flex-col gap-1">
          {Object.entries(fieldDocs).filter(([key]) => !key.includes('.')).map(([key, doc]) => (
            <div key={key} className="flex items-baseline gap-2 text-xs">
              <code className="font-mono font-semibold">{key}</code>
              <span className="truncate text-ink-3">{doc.description.split('.')[0]}</span>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div role="tree" aria-label="YAML structure" className="flex flex-col">
      {nodes.map((node, i) => (
        <TreeNodeItem key={i} node={node} cursorLine={cursorLine} onClickNode={onClickNode} />
      ))}
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
  const active = node.line === cursorLine;
  return (
    <button
      role="treeitem"
      type="button"
      aria-selected={active}
      onClick={() => onClickNode(node.line)}
      style={{ paddingLeft: `${node.depth * 14 + 8}px` }}
      className={`ss-nv !mb-0 !h-[30px] !gap-2 !pr-2 ${active ? 'on' : ''}`}
    >
      {node.depth > 0 && <span className={`h-1 w-1 shrink-0 rounded-full ${active ? 'bg-current' : 'bg-ink-3'}`} />}
      <span className="shrink-0 font-mono text-[13px] font-semibold">{node.key}</span>
      {node.value && <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3">: {node.value}</span>}
      <span className="ml-auto shrink-0 font-mono text-xs text-ink-3">L:{node.line}</span>
    </button>
  );
}
