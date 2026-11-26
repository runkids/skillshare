import { scalarText } from '../../lib/scalarText';
import { Fragment, useCallback, useEffect, useState, type ReactNode } from 'react';
import { Ellipsis, Pencil } from 'lucide-react';
import { useT, plural } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import Reveal from '../Reveal';
import { mcpOffTargets } from '../../api/mcp';
import {
  describeEndpoint,
  hasToolPolicy,
  overrideParts,
  toolSummary,
  usingChanges,
  writes,
  type MatrixRow,
} from './mcpView';
import { TargetPill, TargetToggles } from './TargetPicker';
import type { MCPCheckFinding } from '../../api/mcpCheck';
import MCPCheckFindings, { MCPCheckTag } from './MCPCheckFindings';
import MCPProjectPicker from './MCPProjectPicker';
import Button from '../Button';
import SharedStatus, { type SharedLookup } from './SharedStatus';

interface Props {
  rows: MatrixRow[];
  targets: string[];
  targetsOf: (name: string) => string[];
  onToggle: (name: string, target: string, on: boolean) => void;
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string) => void;
  /** Agents a switch-only entry can go to. mcp.projects cannot reach Claude's off list. */
  offTargets?: readonly string[];
  /** Holds the toggles while a save is on its way: each sends the revision it previewed. */
  disabled?: boolean;
  /** The last check's errors and warnings, by server name. */
  problems?: Record<string, MCPCheckFinding[]>;
  /** The projects that use each server that loads only in those (`global: false`). */
  users?: Record<string, string[]>;
  /** The mcp.projects roots such a server can be ticked for. */
  roots?: string[];
  /** Saves the projects ticked for such a server; left out, they are not offered. */
  onProjects?: (name: string, picked: string[]) => void;
  /** How each project that uses such a server loads it, and where to change that. */
  shared?: SharedLookup & { onOverride: (name: string, root: string) => void };
  /** Opens a project's MCP tab from the projects listed under a shared server. */
  onOpenProject?: (root: string) => void;
  /** Hears whether any row has project ticks not saved yet, which leaving the page would drop. */
  onUnsavedChange?: (unsaved: boolean) => void;
}

/**
 * One row per server. The agents it writes to are logos on its pill and toggles under it, not columns:
 * the list grows downwards as more CLIs gain MCP support, so it never scrolls sideways.
 */
export default function MCPServerList({
  rows,
  targets,
  targetsOf,
  onToggle,
  onMenu,
  offTargets = mcpOffTargets,
  disabled = false,
  problems = {},
  users = {},
  roots = [],
  onProjects,
  shared,
  onOpenProject,
  onUnsavedChange,
}: Props) {
  const t = useT();
  const [open, setOpen] = useState<string[]>([]);
  // Rows whose project ticks are not saved; any of them keeps every project link waiting.
  const [unsavedRows, setUnsavedRows] = useState<string[]>([]);
  const markUnsaved = useCallback(
    (name: string, unsaved: boolean) =>
      setUnsavedRows((prev) => {
        const rest = prev.filter((x) => x !== name);
        return unsaved ? [...rest, name] : rest;
      }),
    [],
  );
  const anyUnsaved = unsavedRows.length > 0;
  useEffect(() => onUnsavedChange?.(anyUnsaved), [anyUnsaved, onUnsavedChange]);
  const overridden = (name: string) =>
    (users[name] ?? []).filter(
      (root) =>
        shared &&
        !shared.replaced(name, root) &&
        overrideParts(t, shared.overrideOf(name, root)).length > 0,
    ).length;
  // A project's own server, or what it changes in this one or inherits, and the way to change that.
  const detail = (name: string, root: string, using: boolean) => {
    if (!shared) return undefined;
    const own = shared.replaced(name, root);
    if (!using && !own) return undefined;
    const parts = own ? [] : overrideParts(t, shared.overrideOf(name, root));
    return (
      <span className="flex shrink-0 items-center gap-2">
        <SharedStatus name={name} root={root} using={using} shared={shared} />
        {using && !own && (
          <Button
            size="sm"
            variant="ghost"
            disabled={disabled}
            onClick={() => shared.onOverride(name, root)}
          >
            <Pencil size={13} />
            {t(parts.length > 0 ? 'mcp.override.edit' : 'mcp.override.add')}
          </Button>
        )}
      </span>
    );
  };

  return (
    <div className="ss-list">
      {rows.map((row) => {
        // A switch-only entry works in a few Agents, so it offers and counts only those.
        const offered = row.server?.disabled
          ? targets.filter((x) => offTargets.includes(x))
          : targets;
        const overrides = shared ? overridden(row.name) : 0;
        const selected = row.server ? targetsOf(row.name).filter((x) => offered.includes(x)) : [];
        const expanded = open.includes(row.name);
        const http = Boolean(row.server?.url);
        const exposure = scalarText(row.server?.piOptions?.exposure ?? 'codemode');
        // Pi's exposure mode in a few words; the full wording is in the title. A value Pi adds later shows as written.
        const pi = {
          text: t(`mcp.piExposureShort.${exposure}`, undefined, exposure),
          title: t(`mcp.piExposure.${exposure}`, undefined, exposure),
        };
        const targetPill = (
          <TargetPill
            selected={selected}
            text={`${selected.length}/${offered.length}`}
            expanded={expanded}
            label={t('mcp.chooseAgents', { name: row.name })}
            onClick={() =>
              setOpen((prev) =>
                expanded ? prev.filter((x) => x !== row.name) : [...prev, row.name],
              )
            }
          />
        );
        return (
          <Fragment key={row.name}>
            <div className="ss-r !items-stretch !flex-col !gap-2 !py-3">
              <span className="flex min-w-0 items-center gap-2">
                <span
                  title={row.name}
                  className={`max-w-[45%] shrink-0 truncate font-mono font-semibold ${row.server ? '' : 'text-ink-3 line-through'}`}
                >
                  {row.name}
                </span>
                {row.server && Object.values(row.cells).some(writes) && (
                  <span className="ss-tag warn">{t('plugins.pending')}</span>
                )}
                {row.server && !row.server.disabled && row.server.targets?.length === 0 && (
                  <span className="ss-tag">{t('plugins.noAgentsYet')}</span>
                )}
                {row.server && users[row.name] && (
                  <span className={`ss-tag ${users[row.name].length === 0 ? 'warn' : ''}`}>
                    {users[row.name].length === 0
                      ? t('mcp.shared.unused')
                      : t(plural('mcp.shared.badge', users[row.name].length), {
                          count: users[row.name].length,
                        })}
                  </span>
                )}
                {overrides > 0 && (
                  <span className="ss-tag inf">
                    {t(plural('mcp.override.count', overrides), { count: overrides })}
                  </span>
                )}
                {problems[row.name] && <MCPCheckTag findings={problems[row.name]} />}
                {row.server && !row.server.disabled && hasToolPolicy(row.server.tools) && (
                  <span className="ss-tag">
                    {t('mcp.tools.chip', { summary: toolSummary(t, row.server.tools) })}
                  </span>
                )}
                {!row.server && <span className="ss-st bad">{t('mcp.removedFromSource')}</span>}
                <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3">
                  {row.server && !row.server.disabled && describeEndpoint(row.server)}
                </span>
                {row.server && targetPill}
                {row.server && (
                  <button
                    type="button"
                    className="ss-ib -my-1.5 -mr-2"
                    aria-label={t('mcp.moreActions', { name: row.name })}
                    onClick={(e) => onMenu(e, row.name)}
                  >
                    <Ellipsis size={16} />
                  </button>
                )}
              </span>
              {row.server && users[row.name] && users[row.name].length > 0 && (
                <span className="truncate font-mono text-xs text-ink-3">
                  {users[row.name].map(shortenHome).join(', ')}
                </span>
              )}
              {(!row.server || row.server.disabled) && (
                <span className="flex min-w-0 items-center gap-2 text-xs text-ink-3">
                  <span className="truncate">
                    {row.server ? t('mcp.offHere') : t('mcp.removedHint')}
                  </span>
                </span>
              )}
            </div>
            {problems[row.name] && <MCPCheckFindings findings={problems[row.name]} />}
            <Reveal open={expanded && Boolean(row.server)} className="ss-rv">
              <div className="ss-r fold !min-h-0 flex-wrap gap-x-6 gap-y-3.5 !py-3.5">
                <TargetToggles
                  offered={offered}
                  selected={selected}
                  http={http}
                  disabled={disabled}
                  tags={selected.includes('pi') && !row.server?.disabled ? { pi } : undefined}
                  onToggle={(target, on) => onToggle(row.name, target, on)}
                />
                {users[row.name] && onProjects && (
                  <RowProjects
                    // A save changes who uses it, which starts the ticks again from there.
                    key={users[row.name].join('\n')}
                    name={row.name}
                    roots={roots}
                    saved={users[row.name]}
                    disabled={disabled}
                    onSave={(picked) => onProjects(row.name, picked)}
                    detail={shared && ((root, using) => detail(row.name, root, using))}
                    onOpen={onOpenProject}
                    locked={anyUnsaved}
                    onUnsaved={markUnsaved}
                  />
                )}
              </div>
            </Reveal>
          </Fragment>
        );
      })}
    </div>
  );
}

/** The projects ticked for one server in its row, saved with one button rather than per tick. */
function RowProjects({
  name,
  roots,
  saved,
  disabled,
  onSave,
  detail,
  onOpen,
  locked,
  onUnsaved,
}: {
  name: string;
  roots: string[];
  saved: string[];
  disabled: boolean;
  onSave: (picked: string[]) => void;
  detail?: (root: string, using: boolean) => ReactNode;
  onOpen?: (root: string) => void;
  locked: boolean;
  onUnsaved: (name: string, unsaved: boolean) => void;
}) {
  const t = useT();
  const [picked, setPicked] = useState(saved);
  const changed = usingChanges(name, saved, picked).length > 0;
  useEffect(() => {
    if (!changed) return;
    onUnsaved(name, true);
    return () => onUnsaved(name, false);
  }, [changed, name, onUnsaved]);
  return (
    <div className="basis-full pt-3.5 [border-top:var(--sep)]">
      <MCPProjectPicker
        roots={roots}
        saved={saved}
        picked={picked}
        onChange={setPicked}
        disabled={disabled}
        hint={t('mcp.projectsPick.hint')}
        detail={detail}
        onOpen={onOpen}
        locked={locked}
      >
        <Button
          size="sm"
          variant="secondary"
          disabled={!changed || disabled}
          onClick={() => setPicked(saved)}
        >
          {t('mcp.projectsPick.revert')}
        </Button>
        <Button
          size="sm"
          variant="primary"
          disabled={!changed || disabled}
          onClick={() => onSave(picked)}
        >
          {t('common.save')}
        </Button>
      </MCPProjectPicker>
    </div>
  );
}
