import { useState } from 'react';
import { Bot, ChevronRight, Puzzle, RefreshCw } from 'lucide-react';
import type { ProjectList, ProjectResource, SyncMatrixEntry } from '../../api/client';
import { plural, useT } from '../../i18n';
import Button from '../Button';
import { RailLayout, RailSection, SyncBox } from '../StatusRail';
import ProjectSectionHead, { InfoTip } from '../mcp/ProjectSectionHead';
import { AgentStack } from '../mcp/TargetPicker';
import { targetLabel } from '../mcp/mcpView';
import FilterCard from '../targets/FilterCard';
import ModeField from '../targets/ModeField';
import PreviewList from '../targets/PreviewList';
import { patternName } from '../targets/targetView';
import { useFilterFeedback } from '../targets/useFilterFeedback';
import ProjectTools from './ProjectTools';
import { syncBoxState, writeFolders, type ProjectState } from './projectView';

const FOLDERS_SHOWN = 5;

interface Props {
  kind: 'skill' | 'agent';
  /** The draft; null while this kind is off. */
  resource: ProjectResource | null;
  onToggle: () => void;
  onChange: (next: ProjectResource) => void;
  /** Preview rows of this kind for the draft filters */
  entries: SyncMatrixEntry[];
  preview: { loaded: boolean; loading: boolean; error: Error | null };
  /** The filters the shown preview rows were computed for, null while they are stale */
  previewKey: string | null;
  tools: ProjectList['tools'];
  /** Tools found on this machine, listed first */
  common: string[];
  targets: string[];
  onTargets: (next: string[]) => void;
  /** The whole project's sync state, as its header shows it */
  health: { state: ProjectState; count: number };
  /** The draft differs from what is saved, which sync does not use yet */
  dirty: boolean;
  onSync: () => void;
  syncDisabled: boolean;
  disabled: boolean;
}

/** A project's Skills or Agents tab: its targets, whether this kind syncs, the filters with a preview, and where it is written. */
export default function ProjectResourceTab({
  kind,
  resource,
  onToggle,
  onChange,
  entries,
  preview,
  previewKey,
  tools,
  common,
  targets,
  onTargets,
  health,
  dirty,
  onSync,
  syncDisabled,
  disabled,
}: Props) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [allFolders, setAllFolders] = useState(false);
  const agent = kind === 'agent';
  const part = agent ? 'agents' : 'skills';
  const title = t(`projects.part.${part}`);
  const hint = t(`projects.part.${part}Hint`);
  const include = resource?.include ?? [];
  const exclude = resource?.exclude ?? [];
  const mode = resource?.mode || 'merge';
  const symlink = mode === 'symlink';
  const synced = entries.filter((e) => e.status === 'synced');
  const counts = { all: entries.length, sync: synced.length, skip: entries.length - synced.length };
  const { folders, none } = writeFolders(tools, targets, kind);
  const box = syncBoxState(health.state);
  const feedback = useFilterFeedback(entries, previewKey);
  const setFilters = (next: { include: string[]; exclude: string[] }) =>
    resource && onChange({ ...resource, ...next });

  return (
    <RailLayout
      rail={
        resource && (
          <>
            <SyncBox
              tone={box.tone}
              state={t(health.count > 0 ? plural(box.key, health.count) : box.key, {
                count: health.count,
              })}
            >
              {preview.loaded && !symlink && (
                <div className="flex flex-col gap-1.5">
                  <button
                    type="button"
                    aria-expanded={open}
                    className="flex items-baseline gap-1.5 text-left text-[13px] text-ink-2"
                    onClick={() => setOpen(!open)}
                  >
                    <span className="shrink-0 font-semibold text-ink">
                      {t(plural(`projects.box.${part}`, synced.length), { count: synced.length })}
                    </span>
                    <span className="min-w-0 flex-1 truncate">
                      →{' '}
                      {t(plural('projects.box.folders', folders.length), {
                        count: folders.length,
                      })}
                    </span>
                    <ChevronRight
                      size={14}
                      className={`shrink-0 self-center transition-transform duration-150 ease-[cubic-bezier(.23,1,.32,1)] motion-reduce:transition-none ${open ? 'rotate-90' : ''}`}
                    />
                  </button>
                  {open && (
                    <div className="flex max-h-40 flex-wrap gap-1.5 overflow-y-auto pb-1">
                      {synced.map((e) => (
                        <span key={e.skill} className="ss-tag font-mono">
                          {patternName(e)}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              )}
              {dirty && <p className="text-xs text-ink-2">{t('projects.box.unsaved')}</p>}
              <Button className="w-full justify-center" onClick={onSync} disabled={syncDisabled}>
                <RefreshCw size={15} />
                {t('projects.sync.button')}
              </Button>
              <p className="text-xs leading-normal text-ink-2">{t('projects.box.caption')}</p>
            </SyncBox>

            <FilterCard
              kind={kind}
              include={include}
              exclude={exclude}
              onChange={setFilters}
              entries={preview.loaded ? entries : undefined}
              symlink={symlink}
              disabled={disabled}
              feedback={feedback}
            />

            {folders.length + none.length > 0 && (
              <RailSection
                title={t('projects.writes')}
                count={folders.length}
                action={<InfoTip hint={t('projects.writes.hint')} />}
              >
                <div className="flex flex-col gap-1">
                  {(allFolders ? folders : folders.slice(0, FOLDERS_SHOWN)).map((f) => (
                    <div
                      key={f.path}
                      className="flex min-h-8 items-center gap-2.5"
                      title={f.tools.map(targetLabel).join(', ')}
                    >
                      <span className="ss-stack shrink-0">
                        <AgentStack targets={f.tools} max={3} />
                      </span>
                      <span className="min-w-0 flex-1 truncate font-mono text-[12.5px]">
                        {f.path}
                      </span>
                      {f.tools.length > 1 && (
                        <span className="ss-tag shrink-0">
                          {t(plural('projects.writes.sharedBy', f.tools.length), {
                            count: f.tools.length,
                          })}
                        </span>
                      )}
                    </div>
                  ))}
                  {none.length > 0 && (
                    <div
                      className="flex min-h-8 items-center gap-2.5 text-[13px] text-ink-3"
                      title={none.map(targetLabel).join(', ')}
                    >
                      <span className="ss-stack shrink-0 opacity-55">
                        <AgentStack targets={none} max={3} />
                      </span>
                      <span className="min-w-0 flex-1">
                        {t(plural('projects.writes.noAgentsCount', none.length), {
                          count: none.length,
                        })}
                      </span>
                    </div>
                  )}
                </div>
                {folders.length > FOLDERS_SHOWN && (
                  <Button
                    variant="link"
                    className="self-start"
                    onClick={() => setAllFolders(!allFolders)}
                  >
                    {allFolders
                      ? t('mcp.projects.showFewer', { count: FOLDERS_SHOWN })
                      : t('mcp.projects.showRest', { count: folders.length - FOLDERS_SHOWN })}
                  </Button>
                )}
              </RailSection>
            )}

            <ModeField
              kind={kind}
              mode={mode}
              onChange={(next) => onChange({ ...resource, mode: next })}
              disabled={disabled}
            />
          </>
        )
      }
    >
      <div className="ss-box !p-0">
        <div className="ss-setrow">
          <div className="l">
            <b>{t('projects.targets')}</b>
            <span>
              {agent && none.length > 0
                ? t(plural('projects.targets.noAgents', none.length), { count: none.length })
                : hint}
            </span>
          </div>
          <ProjectTools
            tools={tools}
            common={common}
            selected={targets}
            onChange={onTargets}
            disabled={disabled}
            summary={t(plural('projects.tools.count', targets.length), { count: targets.length })}
            className="w-[268px] shrink-0"
          />
        </div>
      </div>

      <section className="mt-3 flex flex-col gap-3">
        <ProjectSectionHead
          icon={agent ? <Bot size={16} /> : <Puzzle size={16} />}
          title={title}
          count={resource && preview.loaded ? `${counts.sync} / ${counts.all}` : undefined}
          hint={hint}
        >
          <button
            type="button"
            role="switch"
            aria-checked={Boolean(resource)}
            aria-label={title}
            className="grid h-7 shrink-0 place-items-center disabled:opacity-50"
            disabled={disabled}
            onClick={onToggle}
          >
            <span className={`ss-sw ${resource ? 'on' : ''}`}>
              <i />
            </span>
          </button>
        </ProjectSectionHead>

        {!resource ? (
          <p className="text-[13px] text-ink-2">{t(`projects.part.off.${part}`)}</p>
        ) : symlink ? (
          <div className="ss-note inf">
            <span className="flex-1">{t('targetDetail.symlinkNoFilters')}</span>
          </div>
        ) : (
          <PreviewList
            kind={kind}
            entries={entries}
            include={include}
            exclude={exclude}
            onChange={setFilters}
            loading={preview.loading}
            error={preview.error}
            disabled={disabled}
            feedback={feedback}
          />
        )}
      </section>
    </RailLayout>
  );
}
