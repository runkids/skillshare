import { Fragment, useEffect, useRef, useState } from 'react';
import type { CSSProperties, KeyboardEvent, MouseEvent, ReactNode } from 'react';
import {
  Bot,
  ChevronDown,
  ChevronRight,
  Folder,
  FolderOpen,
  GitBranch,
  Link2,
  Power,
  PowerOff,
  Puzzle,
} from 'lucide-react';
import type { Skill, SourceLink } from '../../api/client';
import { countLabel } from '../../lib/resourceGrouping';
import { useT } from '../../i18n';
import { formatTrackedRepoName } from '../../lib/resourceNames';
import { insideSelection, skillsUnder, treeLines } from './tree';
import type { TreeRow } from './tree';

export type SelectMode = 'only' | 'toggle' | 'range';

interface Props {
  rows: TreeRow[];
  selected: ReadonlySet<string>;
  kind: Skill['kind'];
  label: string;
  onSelect: (id: string, mode: SelectMode) => void;
  onToggleFolder: (path: string) => void;
  onOpen: (skill: Skill) => void;
  renderUnlink?: (link: SourceLink) => ReactNode;
  onContextMenu?: (e: MouseEvent, row: TreeRow) => void;
}

/**
 * Explorer-style tree: click selects, Cmd/Ctrl-click adds, Shift-click takes a range,
 * double-click (or Enter) opens a skill. Keyboard follows the WAI-ARIA tree pattern.
 */
export default function SkillTree({
  rows,
  selected,
  kind,
  label,
  onSelect,
  onToggleFolder,
  onOpen,
  renderUnlink,
  onContextMenu,
}: Props) {
  const t = useT();
  const refs = useRef(new Map<string, HTMLDivElement>());
  const [focusId, setFocusId] = useState<string | null>(null);
  // A collapsing folder's rows shrink away first; the folder closes once they are gone.
  const [closing, setClosing] = useState<string | null>(null);
  // Rows on screen before the first paint do not grow in; any row added later does.
  const shown = useRef<Set<string> | null>(null);
  shown.current ??= new Set(rows.map((r) => r.id));
  useEffect(() => {
    const ids = new Set(rows.map((r) => r.id));
    for (const id of shown.current!) if (!ids.has(id)) shown.current!.delete(id);
  });
  // The close runs after the rows have shrunk, so it checks the folder is still open then.
  const latest = useRef(rows);
  useEffect(() => {
    latest.current = rows;
  });
  const closeTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(closeTimer.current), []);
  const lines = treeLines(rows);
  const ItemIcon = kind === 'agent' ? Bot : Puzzle;

  const toggleFolder = (row: Extract<TreeRow, { type: 'folder' }>) => {
    if (row.collapsed) onToggleFolder(row.node.path);
    else if (!closing) {
      setClosing(row.id);
      // Matches the .ss-tn height transition in components.css.
      closeTimer.current = setTimeout(() => {
        setClosing(null);
        const now = latest.current.find((r) => r.id === row.id);
        if (now?.type === 'folder' && !now.collapsed) onToggleFolder(row.node.path);
      }, 180);
    }
  };
  // The closing folder's rows are the ones after it, up to the next row no deeper than it.
  const at = closing ? rows.findIndex((r) => r.id === closing) : -1;
  const end = at < 0 ? -1 : at + 1 + rows.slice(at + 1).findIndex((r) => r.depth <= rows[at].depth);
  const shrinking = (i: number) => at >= 0 && i > at && (end <= at || i < end);

  // Roving tabindex: the focused row, else the first selected one, else the first row.
  const tabId = rows.some((r) => r.id === focusId)
    ? focusId
    : (rows.find((r) => selected.has(r.id)) ?? rows[0])?.id;

  const focus = (id: string | undefined) => {
    if (!id) return;
    setFocusId(id);
    refs.current.get(id)?.focus();
  };

  const onKeyDown = (e: KeyboardEvent, i: number) => {
    const row = rows[i];
    const folder = row.type === 'folder' ? row : null;
    switch (e.key) {
      case 'ArrowDown':
        focus(rows[i + 1]?.id);
        break;
      case 'ArrowUp':
        focus(rows[i - 1]?.id);
        break;
      case 'Home':
        focus(rows[0]?.id);
        break;
      case 'End':
        focus(rows[rows.length - 1]?.id);
        break;
      case 'ArrowRight':
        if (folder?.collapsed) toggleFolder(folder);
        else if (folder && rows[i + 1]?.depth === row.depth + 1) focus(rows[i + 1].id);
        break;
      case 'ArrowLeft':
        if (folder && !folder.collapsed) toggleFolder(folder);
        else
          for (let j = i - 1; j >= 0; j--)
            if (rows[j].depth === row.depth - 1) {
              focus(rows[j].id);
              break;
            }
        break;
      case ' ':
        onSelect(row.id, 'toggle');
        break;
      case 'Enter':
        if (folder) toggleFolder(folder);
        else if (row.type === 'item') onOpen(row.skill);
        break;
      default:
        return;
    }
    e.preventDefault();
  };

  const onClick = (e: MouseEvent, id: string) => {
    focus(id);
    onSelect(id, e.shiftKey ? 'range' : e.metaKey || e.ctrlKey ? 'toggle' : 'only');
  };

  return (
    <div role="tree" aria-label={label} aria-multiselectable="true" className="ss-tree">
      {rows.map((row, i) => {
        const isSel = selected.has(row.id);
        const inside = !isSel && insideSelection(row, selected);
        const folder = row.type === 'folder' ? row : null;
        const link = kind === 'skill' ? folder?.node.link : undefined;
        const skills = folder ? skillsUnder(folder.node) : [];
        const off = folder
          ? skills.filter((s) => s.disabled).length
          : row.type === 'item' && row.skill.disabled
            ? 1
            : 0;
        const dim = folder ? skills.length > 0 && off === skills.length : off === 1;
        return (
          <div
            key={row.id}
            ref={(el) => {
              if (el) refs.current.set(row.id, el);
              else refs.current.delete(row.id);
            }}
            role="treeitem"
            aria-level={row.depth + 1}
            aria-expanded={folder ? !folder.collapsed : undefined}
            aria-selected={isSel}
            tabIndex={row.id === tabId ? 0 : -1}
            className={`ss-tn ${isSel ? 'sel' : inside ? 'in' : ''} ${dim ? 'off' : ''} ${shown.current!.has(row.id) ? '' : 'grow'} ${shrinking(i) ? 'shut' : ''}`}
            style={{ '--d': row.depth } as CSSProperties}
            onMouseDown={(e) => {
              if (e.shiftKey) e.preventDefault();
            }}
            onClick={(e) => onClick(e, row.id)}
            onContextMenu={link ? undefined : (e) => onContextMenu?.(e, row)}
            onDoubleClick={() => {
              if (row.type === 'item') onOpen(row.skill);
            }}
            onFocus={() => setFocusId(row.id)}
            onKeyDown={(e) => onKeyDown(e, i)}
          >
            {lines[i].through.map((g) => (
              <i key={g} className="gd" style={{ '--g': g } as CSSProperties} />
            ))}
            {row.depth > 0 && (
              <i
                className={`gd el ${lines[i].last ? '' : 'on'}`}
                style={{ '--g': row.depth - 1, '--w': folder ? '9px' : '30px' } as CSSProperties}
              />
            )}
            {folder ? (
              <button
                type="button"
                tabIndex={-1}
                className="cv"
                aria-label={t(
                  folder.collapsed ? 'resources.folder.expand' : 'resources.folder.collapse',
                  { name: folder.names.join('/') },
                )}
                onClick={(e) => {
                  e.stopPropagation();
                  toggleFolder(folder);
                }}
              >
                {folder.collapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
              </button>
            ) : (
              <span className="cv" />
            )}
            {folder ? (
              link ? (
                <Link2 size={15} className="ic" />
              ) : folder.repo ? (
                <GitBranch size={15} className="ic" />
              ) : folder.collapsed ? (
                <Folder size={15} className="ic" />
              ) : (
                <FolderOpen size={15} className="ic" />
              )
            ) : dim ? (
              // A disabled item says so with its icon; the hover text is for folder counts only.
              <PowerOff
                size={14}
                className="ic"
                role="img"
                aria-label={t('resources.status.disabled')}
              >
                <title>{t('resources.status.disabled')}</title>
              </PowerOff>
            ) : (
              <ItemIcon size={14} className="ic" />
            )}
            <span className={`nm ${folder ? 'f' : ''}`}>
              {folder
                ? folder.names.map((n, j) => (
                    <Fragment key={j}>
                      {j > 0 && <span className="sl">/</span>}
                      {folder.repo && !link ? formatTrackedRepoName(n) : n}
                    </Fragment>
                  ))
                : row.type === 'item' && row.skill.name}
            </span>
            {folder?.repo && !link && <span className="ss-tag shrink-0">tracked</span>}
            {link && skills.length === 0 && (
              <>
                <span className="min-w-0 truncate font-mono text-xs text-ink-3" title={link.target}>
                  {link.target}
                </span>
                <span className="text-xs text-ink-3">{link.warning ?? countLabel(t, kind, 0)}</span>
                <span className="ml-auto">{renderUnlink?.(link)}</span>
              </>
            )}
            <span className={`hv ${link ? '!ml-auto' : ''}`}>
              {folder && !(link && skills.length === 0) && <span>{skills.length}</span>}
              {folder && off > 0 && (
                <span className="inline-flex items-center gap-1">
                  <Power size={12} />
                  {off === skills.length
                    ? t('resources.tree.allDisabled')
                    : t('resources.tree.someDisabled', { count: off })}
                </span>
              )}
            </span>
          </div>
        );
      })}
    </div>
  );
}
