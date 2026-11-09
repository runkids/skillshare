import { Fragment, useEffect, useRef, useLayoutEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { Check, ChevronRight, Target } from 'lucide-react';
import { useT } from '../i18n';
import { useAvailableTargetsQuery } from '../hooks/useSharedQueries';
import { useHeldWhileClosing, useMenuPresence } from '../hooks/usePresence';

/* ------------------------------------------------------------------ */
/*  Context menu item types                                           */
/* ------------------------------------------------------------------ */

/** A sub-item inside a submenu. */
export interface ContextMenuSubItem {
  key: string;
  label: string;
  selected?: boolean;
  onSelect: () => void;
}

/** A top-level menu item. Can be a direct action or a submenu trigger. */
export interface ContextMenuItem {
  key: string;
  label: string;
  /** What picking this does, shown under the label. */
  description?: string;
  /** Show the label as code, for field names and paths. */
  mono?: boolean;
  icon?: React.ReactNode;
  /** Direct action — mutually exclusive with `items` */
  onSelect?: () => void;
  /** Submenu items — hover/click to expand. Mutually exclusive with `onSelect` */
  items?: ContextMenuSubItem[];
  /** Destructive action: red text, separated from the items above. */
  danger?: boolean;
}

/* ------------------------------------------------------------------ */
/*  SkillContextMenu — extensible right-click / action menu           */
/*                                                                    */
/* ------------------------------------------------------------------ */

interface SkillContextMenuProps {
  items: ContextMenuItem[];
  anchorPoint?: { x: number; y: number };
  open: boolean;
  onClose: () => void;
  /** `end` puts the menu's right edge at the anchor, under a button on the right. */
  align?: 'start' | 'end';
}

export function SkillContextMenu({
  items,
  anchorPoint,
  align = 'start',
  open,
  onClose,
}: SkillContextMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState({ top: 0, left: 0, origin: 'top left' });
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  const menu = useMenuPresence(open);
  // Callers clear items as they close; the fading menu keeps showing the last open list.
  const shownItems = useHeldWhileClosing(items, open);

  // Collapse the submenu once the menu is gone, so a closing menu fades out as it was
  useEffect(() => {
    if (!menu.shown) setExpandedKey(null);
  }, [menu.shown]);

  // Viewport collision detection
  useLayoutEffect(() => {
    if (!open || !anchorPoint || !menuRef.current) return;
    const menu = menuRef.current;
    // offsetWidth/Height, not getBoundingClientRect: the menu is still at its scaled-down first animation frame.
    const rect = { width: menu.offsetWidth, height: menu.offsetHeight };
    let top = anchorPoint.y;
    let left = align === 'end' ? Math.max(8, anchorPoint.x - rect.width) : anchorPoint.x;
    if (top + rect.height > window.innerHeight - 8) {
      top = Math.max(8, anchorPoint.y - rect.height);
    }
    if (left + rect.width > window.innerWidth - 8) {
      left = Math.max(8, anchorPoint.x - rect.width);
    }
    // Grow out of the corner that touches the anchor, whichever way the menu flipped.
    setPosition({ top, left, origin: `${top < anchorPoint.y ? 'bottom' : 'top'} ${left < anchorPoint.x ? 'right' : 'left'}` });
  }, [open, anchorPoint, align]);

  // Close on Escape (submenu first, then parent)
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (expandedKey) setExpandedKey(null);
        else onClose();
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [open, onClose, expandedKey]);

  // Close on scroll
  useEffect(() => {
    if (!open) return;
    const handler = () => onClose();
    window.addEventListener('scroll', handler, { passive: true, capture: true });
    return () => window.removeEventListener('scroll', handler, true);
  }, [open, onClose]);

  if (!menu.shown) return null;

  // Portal to body to escape any ancestor transform/filter that breaks position:fixed.
  // A transparent overlay behind the menu intercepts clicks so they don't
  // propagate to underlying Links — first click only dismisses the menu.
  return createPortal(
    <>
      {open && <div
        className="fixed inset-0 z-[99]"
        onMouseDown={(e) => { e.preventDefault(); e.stopPropagation(); onClose(); }}
        onContextMenu={(e) => { e.preventDefault(); onClose(); }}
      />}
      <div
        ref={menuRef}
        {...menu.exitProps}
        className={`ss-menu fixed z-[100] !w-auto min-w-[190px] max-h-[70vh] overflow-y-auto ${menu.motion}`}
        style={{ top: position.top, left: position.left, transformOrigin: position.origin }}
        role="menu"
      >
        {shownItems.map((item, i) => (
          <Fragment key={item.key}>
            {item.danger && !shownItems[i - 1]?.danger && i > 0 && <hr />}
            {item.items ? (
              <SubmenuTrigger
                item={item}
                expanded={expandedKey === item.key}
                onExpand={() => setExpandedKey(item.key)}
                // A late timer from a submenu left earlier must not close the one open now
                onCollapse={() => setExpandedKey((k) => (k === item.key ? null : k))}
                onClose={onClose}
                parentMenuRef={menuRef}
                closing={menu.closing}
              />
            ) : (
              <button
                type="button"
                className={`${item.danger ? 'dng' : ''} ${item.description ? '!h-auto max-w-[340px] py-1.5' : ''}`}
                role="menuitem"
                onMouseDown={(e) => { e.preventDefault(); item.onSelect?.(); onClose(); }}
                onMouseEnter={() => setExpandedKey(null)}
              >
                {item.icon && <span className="w-4 shrink-0 flex items-center justify-center">{item.icon}</span>}
                {item.description ? (
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span className={item.mono ? 'font-mono' : ''}>{item.label}</span>
                    <span className="text-xs font-normal leading-snug text-ink-3">{item.description}</span>
                  </span>
                ) : item.label}
              </button>
            )}
          </Fragment>
        ))}
      </div>
    </>,
    document.body,
  );
}

/* ------------------------------------------------------------------ */
/*  SubmenuTrigger — top-level item that expands a submenu on hover    */
/* ------------------------------------------------------------------ */

function SubmenuTrigger({
  item,
  expanded,
  onExpand,
  onCollapse,
  onClose,
  parentMenuRef,
  closing,
}: {
  item: ContextMenuItem;
  expanded: boolean;
  onExpand: () => void;
  onCollapse: () => void;
  onClose: () => void;
  parentMenuRef: React.RefObject<HTMLDivElement | null>;
  closing: boolean;
}) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [subPos, setSubPos] = useState<{ top: number; left: number } | null>(null);

  // Position submenu relative to trigger + parent
  useLayoutEffect(() => {
    if (!expanded || !triggerRef.current || !parentMenuRef.current) return;
    const triggerRect = triggerRef.current.getBoundingClientRect();
    const parentRect = parentMenuRef.current.getBoundingClientRect();

    let top = triggerRect.top - 4; // slight upward offset for visual alignment
    let left = parentRect.right + 4;

    // Flip left if overflows right
    if (left + 176 > window.innerWidth - 8) {
      left = parentRect.left - 176 - 4;
    }
    // Adjust top if submenu overflows bottom
    const subHeight = (item.items?.length ?? 0) * 32 + 12;
    if (top + subHeight > window.innerHeight - 8) {
      top = Math.max(8, window.innerHeight - subHeight - 8);
    }

    setSubPos({ top, left });
  }, [expanded, item.items?.length, parentMenuRef]);

  // Hover intent: collapse after delay when mouse leaves
  const collapseTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(collapseTimer.current), []);
  const handleEnter = () => { clearTimeout(collapseTimer.current); onExpand(); };
  const handleLeave = () => { collapseTimer.current = setTimeout(onCollapse, 180); };
  const handleSubEnter = () => { clearTimeout(collapseTimer.current); };
  const handleSubLeave = () => { collapseTimer.current = setTimeout(onCollapse, 180); };

  return (
    <>
      <button
        type="button"
        ref={triggerRef}
        className={expanded ? 'hv' : ''}
        role="menuitem"
        aria-haspopup="true"
        aria-expanded={expanded}
        onMouseEnter={handleEnter}
        onMouseLeave={handleLeave}
        onClick={handleEnter}
      >
        {item.icon && <span className="w-4 shrink-0 flex items-center justify-center">{item.icon}</span>}
        <span className="flex-1">{item.label}</span>
        <ChevronRight size={14} className="shrink-0 text-ink-3" />
      </button>

      {/* Submenu panel — portaled to body to escape parent's transform containing block */}
      {expanded && subPos && createPortal(
        <div
          {...(closing ? { inert: true, 'aria-hidden': true } : {})}
          className={`ss-menu fixed z-[101] !w-auto min-w-[176px] max-h-64 overflow-y-auto ${closing ? 'animate-dropdown-out' : ''}`}
          style={{ top: subPos.top, left: subPos.left }}
          role="menu"
          onMouseEnter={handleSubEnter}
          onMouseLeave={handleSubLeave}
        >
          {item.items?.map((sub) => (
            <button
              type="button"
              key={sub.key}
              role="menuitemradio"
              aria-checked={!!sub.selected}
              onMouseDown={(e) => { e.preventDefault(); sub.onSelect(); onClose(); }}
            >
              <span className="w-4 shrink-0 flex items-center justify-center">{sub.selected && <Check size={14} />}</span>
              <span className={sub.selected ? 'font-semibold' : ''}>{sub.label}</span>
            </button>
          ))}
        </div>,
        document.body,
      )}
    </>
  );
}

/* ------------------------------------------------------------------ */
/*  TargetMenu — convenience wrapper for "Set Target" action          */
/* ------------------------------------------------------------------ */

interface TargetMenuProps {
  currentTargets: string[] | null;  // null = All
  isUniform?: boolean;              // for folders
  showTargets?: boolean;
  /** Menu item label. Defaults to "Available in...". */
  label?: string;
  /** Additional flat action items appended after the target submenu (e.g. Uninstall). */
  extraItems?: ContextMenuItem[];
  /** List the targets directly instead of behind an "Available in" submenu. */
  flat?: boolean;
  onSelect: (target: string | null) => void;
  anchorPoint?: { x: number; y: number };
  open: boolean;
  onClose: () => void;
}

export default function TargetMenu({
  currentTargets,
  isUniform = true,
  showTargets = true,
  label = 'Available in...',
  extraItems,
  flat = false,
  onSelect,
  anchorPoint,
  open,
  onClose,
}: TargetMenuProps) {
  const t = useT();
  const { data: availableData } = useAvailableTargetsQuery({ enabled: open });

  const targets = (availableData?.targets ?? []).filter((t) => t.installed);
  const isAllSelected = isUniform && (!currentTargets || currentTargets.length === 0);

  const targetItems: ContextMenuSubItem[] = [
    {
      key: '__all__',
      label: t('targetMenu.allTargets'),
      selected: isAllSelected,
      onSelect: () => onSelect(null),
    },
    ...targets.map((t) => ({
      key: t.name,
      label: t.name,
      selected: isUniform && currentTargets?.length === 1 && currentTargets[0] === t.name,
      onSelect: () => onSelect(t.name),
    })),
  ];

  const items: ContextMenuItem[] = flat
    ? targetItems.map((sub) => ({ key: sub.key, label: sub.label, icon: sub.selected ? <Check size={14} /> : <span />, onSelect: sub.onSelect }))
    : showTargets ? [{ key: 'set-target', label, icon: <Target size={14} />, items: targetItems }] : [];
  items.push(...(extraItems ?? []));

  if (items.length === 0) return null;

  return (
    <SkillContextMenu
      items={items}
      anchorPoint={anchorPoint}
      open={open}
      onClose={onClose}
    />
  );
}
