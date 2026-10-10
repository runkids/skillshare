import type { MouseEvent, ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { ArrowUpRight, Bot, FolderInput, Puzzle, Target, Trash2 } from 'lucide-react';
import type { Skill } from '../../api/client';
import { useT } from '../../i18n';
import { formatTrackedRepoName, resourceHref } from '../../lib/resourceNames';
import AgentIcon from '../AgentIcon';
import Tooltip from '../Tooltip';
import Button from '../Button';
import { isRepoRoot, summarize } from './tree';
import type { FolderNode } from './tree';

export type PaneSubject =
  | { type: 'none' }
  | { type: 'folder'; node: FolderNode; skills: Skill[] }
  | { type: 'skill'; skill: Skill }
  | { type: 'multi'; skills: Skill[] };

interface Props {
  kind: Skill['kind'];
  subject: PaneSubject;
  busy: boolean;
  /** Turn every listed skill on or off. */
  onToggleAll: (skills: Skill[], enable: boolean) => void;
  onToggleOne: (skill: Skill) => void;
  onSetTargets: (e: MouseEvent) => void;
  onUninstall: () => void;
  /** Opens the move dialog for what is selected; leave out when it cannot move. */
  move?: { label: string; run: () => void };
  /** Update / Uninstall or Unlink for a source group root. */
  repoActions?: ReactNode;
  /** Where a single selected item actually syncs to, as the list's Targets column shows it; replaces its `targets:` setting. */
  syncedTo?: ReactNode;
}

/** "_team-repo/frontend" → "team-repo / frontend": the repo's leading "_" is how it is stored, not its name. */
function crumb(path: string): string {
  return path.split('/').filter(Boolean).map((seg, i) => (i === 0 && seg.startsWith('_') ? formatTrackedRepoName(seg) : seg)).join(' / ');
}

function Switch({ on, mixed, label, disabled, onClick }: { on: boolean; mixed?: boolean; label: string; disabled: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={mixed ? 'mixed' : on}
      aria-label={label}
      disabled={disabled}
      className={`ss-sw ${on ? 'on' : mixed ? 'mix' : ''}`}
      onClick={onClick}
    >
      <i />
    </button>
  );
}

/** Right side of the tree view: what is selected, whether it is on, and where it goes. */
export default function TreeDetailPane({ kind, subject, busy, onToggleAll, onToggleOne, onSetTargets, onUninstall, move, repoActions, syncedTo }: Props) {
  const t = useT();
  const isAgent = kind === 'agent';
  const ItemIcon = isAgent ? Bot : Puzzle;

  if (subject.type === 'none') {
    return <div className="grid h-full place-items-center p-6 text-[13px] text-ink-3">{t(isAgent ? 'resources.tree.pane.emptyAgent' : 'resources.tree.pane.empty')}</div>;
  }

  const skills = subject.type === 'skill' ? [subject.skill] : subject.skills;
  const count = (n: number) => t(`resources.count.${kind}${n === 1 ? '' : 's'}`, { count: n });
  let path = '';
  let title = '';
  let meta = '';
  let tracked = false;
  const link = subject.type === 'folder' && !isAgent ? subject.node.link : undefined;
  if (subject.type === 'folder') {
    const { node } = subject;
    const repo = !link && isRepoRoot(node);
    path = node.path.slice(0, node.path.length - node.name.length);
    title = link ? link.name : repo ? formatTrackedRepoName(node.name) : node.name;
    tracked = !link && node.path.startsWith('_');
    meta = [link?.warning ?? count(skills.length), repo ? skills[0]?.branch : ''].filter(Boolean).join(' · ');
  } else if (subject.type === 'skill') {
    const { skill } = subject;
    path = skill.relPath.slice(0, Math.max(0, skill.relPath.lastIndexOf('/')));
    title = skill.name;
    tracked = skill.isInRepo;
  } else {
    title = t('resources.tree.pane.selected', { what: count(skills.length) });
  }

  const enabled = skills.filter((s) => !s.disabled).length;
  const allOn = enabled === skills.length;
  const noneOn = enabled === 0;
  const state = allOn ? t('resources.status.enabled') : noneOn ? t('resources.status.disabled') : t('resources.tree.pane.partial', { count: enabled, total: skills.length });
  const summary = summarize(skills);

  return (
    <div className="flex flex-col gap-5 p-6">
      <div className="flex flex-col gap-1.5">
        {path && <div className="truncate font-mono text-xs text-ink-3">{crumb(path)} /</div>}
        <div className="flex min-w-0 items-center gap-2.5">
          <h2 className={`min-w-0 truncate text-lg font-bold tracking-tight ${subject.type === 'multi' ? '' : 'font-mono'}`}>{title}</h2>
          {tracked && <span className="ss-tag shrink-0">tracked</span>}
          <span className="flex-1" />
          {move && (
            <Button variant="secondary" size="sm" className="shrink-0" onClick={move.run}>
              <FolderInput size={14} />
              {move.label}
            </Button>
          )}
          {subject.type === 'skill' && (
            <Link to={resourceHref(subject.skill)} className="ss-btn sm shrink-0">
              {t(isAgent ? 'resources.tree.pane.openAgent' : 'resources.tree.pane.openSkill')}
              <ArrowUpRight size={14} />
            </Link>
          )}
          {subject.type === 'multi' && (
            <Button variant="secondary" size="sm" className="shrink-0" onClick={onUninstall}>
              <Trash2 size={14} />
              {t('resources.contextMenu.uninstall')}
            </Button>
          )}
        </div>
        {meta && <div className="text-[13px] text-ink-2">{meta}</div>}
        {link && <p className="break-all font-mono text-xs text-ink-3">{link.target}</p>}
      </div>

      <div className="ss-tree-props">
        {skills.length > 0 && <div>
          <span className="lb">{t('resources.col.status')}</span>
          <Switch
            on={allOn}
            mixed={!allOn && !noneOn}
            label={title}
            disabled={busy}
            onClick={() => onToggleAll(skills, !allOn)}
          />
          <span className="text-[13px] font-semibold">{state}</span>
        </div>}
        {skills.length > 0 && (!isAgent || syncedTo) && (
          <div>
            <span className="lb">{t('resources.col.targets')}</span>
            {syncedTo ? (
              // One item: where it actually goes. Its own `targets:` only shows when it narrows that.
              <span className="flex min-w-0 items-center gap-2">
                {syncedTo}
                {summary.targets.length > 0 && (
                  <Tooltip content={summary.targets.join(', ')}>
                    <span className="ss-tag">{t('resources.tree.pane.limited')}</span>
                  </Tooltip>
                )}
              </span>
            ) : !summary.isUniform ? (
              <span className="text-[13px]">{t('resources.tree.mixed')}</span>
            ) : summary.targets.length === 0 ? (
              <span className="text-[13px]">{t('targetMenu.allTargets')}</span>
            ) : (
              <span className="flex min-w-0 items-center gap-2">
                <span className="ss-stack shrink-0">
                  {summary.targets.slice(0, 4).map((n) => <span key={n} className="ss-at"><AgentIcon target={n} size={14} /></span>)}
                </span>
                <span className="truncate text-[13px]">{summary.display}</span>
              </span>
            )}
            <span className="flex-1" />
            {!isAgent && (
              <Button variant="secondary" size="sm" className="shrink-0" onClick={onSetTargets}>
                <Target size={14} />
                {t('resources.setTargets')}
              </Button>
            )}
          </div>
        )}
        {repoActions && (
          <div>
            <span className="lb">{t('resources.tree.pane.repo')}</span>
            <span className="flex flex-wrap items-center gap-2">{repoActions}</span>
          </div>
        )}
      </div>

      {subject.type !== 'skill' && (
        <div className="flex flex-col gap-1">
          <div className="mb-1 text-xs font-semibold text-ink-3">{t(isAgent ? 'layout.nav.agents' : 'layout.nav.skills')}</div>
          {skills.map((s) => (
            <div key={s.flatName} className="ss-tree-item">
              <ItemIcon size={14} className="shrink-0 text-ink-3" />
              <Link to={resourceHref(s)} className={`min-w-0 flex-1 truncate font-mono text-[13px] font-medium hover:underline ${s.disabled ? 'text-ink-3' : ''}`}>
                {s.name}
              </Link>
              <Switch on={!s.disabled} label={s.name} disabled={busy} onClick={() => onToggleOne(s)} />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
