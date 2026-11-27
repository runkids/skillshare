import { Fragment, useEffect, useId, useState } from 'react';
import { ApiError } from '../../api/client';
import {
  pluginsApi,
  targetMap,
  type PluginBinding,
  type PluginCandidate,
  type PluginDiscovery,
  type PluginPackage,
  type PluginRequest,
  type PluginTarget,
  type PluginTargetDefinition,
  npmDefaultName,
} from '../../api/plugins';
import { AlertCircle, Check, ChevronRight, Package, Search, TriangleAlert } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import AgentStack from '../AgentStack';
import { agentReasons } from './agentReasons';
import { keyedText } from './outcomeText';
import SourceHint from './SourceHint';
import { pluginErrorMessage } from './pluginError';
import PluginDocsLink from './PluginDocsLink';
import Button from '../Button';
import DialogHeader from '../DialogHeader';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import { Checkbox, Input } from '../Input';
import { useT, plural } from '../../i18n';
import { useAppContext } from '../../context/AppContext';
import { shortenPath } from '../../lib/paths';

interface Props {
  onClose: () => void;
  onPreview: (r: PluginRequest) => Promise<void>;
  initialSource?: string;
  initialName?: string;
  /** Targets ticked when they can take the plugin, such as the target page this was opened from. */
  initialTargets?: PluginTarget[];
  /** The page's Agents, for an npm: source, which has no discovery to list them. */
  definitions?: PluginTargetDefinition[];
  /** What the plugin is bound to already. With it the dialog adds Agents to that plugin: the source is known, so it opens on discovery's answer. */
  bound?: Partial<Record<PluginTarget, PluginBinding>>;
  /** What was recorded when the plugin was added, for a plugin that has no Agent yet. */
  recorded?: Pick<PluginPackage, 'sourceRef' | 'entry' | 'plugin'>;
}

// What pi.dev offers to copy, `pi install npm:<package>` or the page's own address, is an npm source.
function npmFromPi(value: string) {
  const command = value.match(/^\s*pi\s+install\s+(npm:\S+)\s*$/);
  const page = value.match(
    /^\s*https?:\/\/(?:www\.)?pi\.dev\/packages\/((?:@[^/\s?#]+\/)?[^/\s?#]+)/,
  );
  if (command) return command[1];
  if (!page) return value;
  try {
    return `npm:${decodeURIComponent(page[1])}`;
  } catch {
    return value;
  }
}

function isHerdrOnly(candidate: PluginCandidate | undefined) {
  return candidate?.targets.length === 1 && candidate.targets[0] === 'herdr-app';
}

// The tag beside a plugin's name: what kind of Herdr package it is, or its version.
function candidateTag(candidate: PluginCandidate) {
  if (isHerdrOnly(candidate))
    return candidate.targetInfo?.['herdr-app']?.entry ? 'launcher' : 'mod';
  return (
    candidate.version ||
    Object.values(candidate.targetInfo ?? {}).find((info) => info?.version)?.version
  );
}

export default function PluginAddDialog({
  onClose,
  onPreview,
  initialSource = '',
  initialName = '',
  initialTargets = [],
  definitions = [],
  bound,
  recorded,
}: Props) {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const known = [...Object.values(bound ?? {}), recorded];
  const extending = !!bound && !!initialSource;
  const [sourceRef, setSourceRef] = useState(known.find((b) => b?.sourceRef)?.sourceRef ?? '');
  const [entry, setEntry] = useState(known.find((b) => b?.entry)?.entry ?? '');
  // Its own field, apart from the OpenCode entry asked later: a path here is what makes this a launcher.
  const [launcherEntry, setLauncherEntry] = useState('');
  const [attachPanel, setAttachPanel] = useState(true);
  const [entryOpen, setEntryOpen] = useState(false);
  // Extending opens from the row's "not supported" link, so the reasons start in view.
  const [blockedOpen, setBlockedOpen] = useState(extending);
  const [source, setSource] = useState(initialSource);
  const [discovery, setDiscovery] = useState<PluginDiscovery | null>(null);
  const [name, setName] = useState('');
  const [selectedMods, setSelectedMods] = useState(() => new Set<string>());
  const [packageName, setPackageName] = useState(initialName);
  const [targets, setTargets] = useState<PluginTarget[]>([]);
  const [busy, setBusy] = useState(extending);
  const [error, setError] = useState('');
  const id = useId();
  const sourceHelpId = `${id}-source-help`;
  const launcherHelpId = `${id}-launcher-help`;
  const chooseId = `${id}-choose`;
  const showLauncher = !isProjectMode && !source.trim().startsWith('npm:');
  const launcherMode = showLauncher && !!launcherEntry.trim();
  // An npm: source is installed by Pi, which downloads it: there is nothing to discover first.
  const [npmSource, setNpmSource] = useState('');
  const npmTargets = definitions.filter(
    (d) => d.npm && d.operations.includes('add') && (!isProjectMode || d.project),
  );
  const pluginTargets = targetMap(discovery?.targetDefinitions);
  const selected = discovery?.candidates.find((c) => c.name === name);
  const herdrOnly = isHerdrOnly(selected);
  const canBatchMods =
    !isProjectMode &&
    !extending &&
    !launcherMode &&
    !!discovery &&
    discovery.candidates.length > 1 &&
    discovery.candidates.every((c) => isHerdrOnly(c) && !c.targetInfo?.['herdr-app']?.entry);
  const availableMods = canBatchMods ? discovery.candidates.filter((c) => !c.problem) : [];
  const chosenMods = availableMods.filter((c) => selectedMods.has(c.name));
  const allModsSelected = chosenMods.length === availableMods.length;
  const reasons = agentReasons(selected, pluginTargets, isProjectMode, t).filter(
    ({ target }) => !herdrOnly || target === 'herdr-app',
  );
  const defaultTargets = (candidate: PluginCandidate | undefined): PluginTarget[] =>
    !isProjectMode && !extending && !initialTargets.length && isHerdrOnly(candidate)
      ? ['herdr-app']
      : initialTargets;
  const usable = reasons.filter((r) => !r.reason);
  const blocked = reasons.filter((r) => r.reason);
  const addable = usable.filter((u) => !bound?.[u.target]).map((u) => u.target);
  const allAdded = addable.every((target) => targets.includes(target));
  const toggle = (target: PluginTarget) =>
    setTargets((old) =>
      old.includes(target) ? old.filter((x) => x !== target) : [...old, target],
    );
  // A preselected target the chosen plugin can't use stays out of the request.
  const chosen = npmSource
    ? targets
    : targets.filter((target) => usable.some((u) => u.target === target));
  const placementCandidate = canBatchMods
    ? chosenMods.length === 1
      ? chosenMods[0]
      : undefined
    : selected;
  const canAttachPanel =
    !extending &&
    (canBatchMods || (chosen.length === 1 && chosen[0] === 'herdr-app')) &&
    !!(placementCandidate?.herdrPanel || placementCandidate?.targetInfo?.['herdr-app']?.herdrDock);
  const submitSource = () => {
    const value = source.trim();
    if (!value.startsWith('npm:')) {
      void discover();
      return;
    }
    setNpmSource(value);
    setError('');
    setTargets(initialTargets.filter((target) => npmTargets.some((d) => d.target === target)));
  };
  // `keep` is the second look at the same source, after an OpenCode entry was given: the choice made so far stays.
  const discover = async (keep = false) => {
    setBusy(true);
    setError('');
    try {
      const d = await pluginsApi.discover(
        source.trim(),
        sourceRef.trim() || undefined,
        (launcherMode ? launcherEntry : keep || extending ? entry : '').trim() || undefined,
      );
      if (launcherMode && !d.candidates.some((c) => c.targetInfo?.['herdr-app']?.entry)) {
        setError(t('plugins.error.herdrLauncherEntry'));
        return;
      }
      setDiscovery(d);
      const plugin = known.find((b) => b?.plugin)?.plugin;
      // A blocked row cannot be chosen, so it is never chosen for the user either.
      const open = d.candidates.filter((c) => !c.problem);
      if (!keep) {
        setSelectedMods(new Set());
        setAttachPanel(true);
        const candidate =
          open.find((c) => c.name === plugin) ?? (open.length === 1 ? open[0] : undefined);
        setName(candidate?.name ?? '');
        setTargets(defaultTargets(candidate));
      }
    } catch (e) {
      setError(
        launcherMode && e instanceof ApiError && e.code === 'plugins.error.noPluginSource'
          ? t('plugins.error.herdrLauncherEntry')
          : pluginErrorMessage(e, t),
      );
    } finally {
      setBusy(false);
    }
  };
  // Asking for a click on a source that is already filled in would be a step with nothing to decide.
  useEffect(() => {
    if (extending) void discover();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const preview = async () => {
    setBusy(true);
    setError('');
    try {
      if (npmSource)
        await onPreview({
          action: 'add',
          source: npmSource,
          name: packageName.trim() || undefined,
          targets: chosen,
        });
      else if (canBatchMods)
        await onPreview({
          action: 'add',
          source: discovery.source,
          sourceRef: discovery.sourceRef || undefined,
          ...(canAttachPanel && attachPanel && placementCandidate
            ? { plugin: placementCandidate.name, herdrPanel: true }
            : { plugins: chosenMods.map((mod) => mod.name) }),
          targets: ['herdr-app'],
        });
      else
        await onPreview({
          action: 'add',
          source: discovery!.source,
          sourceRef: discovery!.sourceRef || undefined,
          entry: (launcherMode ? launcherEntry : entry).trim() || undefined,
          herdrPanel: (canAttachPanel && attachPanel) || undefined,
          plugin: name,
          name: packageName.trim() || undefined,
          targets: chosen,
        });
    } catch (e) {
      setError(pluginErrorMessage(e, t));
    } finally {
      setBusy(false);
    }
  };
  return (
    <DialogShell
      open
      onClose={onClose}
      preventClose={busy}
      ariaLabel={t(extending ? 'plugins.addAgents' : 'plugins.add')}
      maxWidth="2xl"
      padding="none"
    >
      <DialogHeader
        title={t(extending ? 'plugins.addAgents' : 'plugins.add')}
        subtitle={
          npmSource ||
          (discovery ? (
            <>
              {shortenPath(discovery.source)}
              {discovery.commit && ` · ${discovery.commit.slice(0, 7)}`}
            </>
          ) : extending && busy ? (
            initialName
          ) : (
            t('plugins.sourceHelp')
          ))
        }
        subtitleProps={
          npmSource || discovery
            ? {
                className: 'truncate font-mono text-xs text-ink-3',
                title: npmSource || discovery?.source,
              }
            : { id: sourceHelpId }
        }
        onClose={onClose}
        closeDisabled={busy}
      />
      <div className={`db overflow-y-auto ${discovery || npmSource ? 'gap-[22px]' : 'gap-3.5'}`}>
        {!discovery && extending && busy ? (
          <div className="flex flex-col gap-1.5">
            <p className="flex items-center gap-2 text-[13px] text-ink-2">
              <Spinner size="sm" />
              {t('plugins.discovering')}
            </p>
            <SourceHint active />
          </div>
        ) : npmSource ? (
          <>
            <div role="note" className="ss-note warn">
              <TriangleAlert size={16} aria-hidden="true" />
              <span className="flex-1">{t('plugins.npmNotice')}</span>
            </div>
            <Input
              label={t('resources.col.name')}
              value={packageName}
              placeholder={npmDefaultName(npmSource)}
              disabled={busy}
              onChange={(e) => setPackageName(e.target.value)}
            />
            <div className="flex flex-col gap-2">
              <span className="text-[13px] font-semibold">{t('plugins.targets')}</span>
              {npmTargets.length === 0 ? (
                <p className="text-[13px] text-ink-3">{t('plugins.npmNoTargets')}</p>
              ) : (
                <div className="ss-checks">
                  {npmTargets.map((definition) => {
                    const on = targets.includes(definition.target);
                    return (
                      <button
                        key={definition.target}
                        type="button"
                        role="checkbox"
                        aria-checked={on}
                        aria-label={definition.label}
                        disabled={busy}
                        onClick={() => toggle(definition.target)}
                      >
                        <span className={`ss-chk ${on ? 'on' : ''}`}>
                          {on && <Check size={11} strokeWidth={3.5} />}
                        </span>
                        <AgentIcon target={definition.target} size={18} />
                        <span className="nm">{definition.label}</span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </>
        ) : !discovery ? (
          <form
            id="plugin-source"
            className="flex flex-col gap-3.5"
            onSubmit={(e) => {
              e.preventDefault();
              if (source.trim()) submitSource();
            }}
          >
            <Input
              label={t('plugins.source')}
              placeholder="owner/repo"
              className={`font-mono ${error ? '!border-bad' : ''}`}
              aria-invalid={error ? true : undefined}
              value={source}
              disabled={busy}
              autoFocus
              spellCheck={false}
              aria-describedby={sourceHelpId}
              onChange={(e) => {
                setSource(npmFromPi(e.target.value));
                setError('');
              }}
            />
            {/* Both optional, so they sit side by side under the source instead of behind a toggle. */}
            <div className="grid grid-cols-2 items-start gap-3">
              <Input
                label={
                  <span className="flex items-center gap-1.5 text-xs text-ink-2">
                    {t('plugins.sourceRef')}
                    <span className="font-normal text-ink-3">{t('plugins.optional')}</span>
                  </span>
                }
                placeholder="main"
                size="sm"
                className="font-mono"
                value={sourceRef}
                disabled={busy}
                spellCheck={false}
                onChange={(e) => {
                  setSourceRef(e.target.value);
                  setError('');
                }}
              />
              {showLauncher && (
                <div className="flex flex-col gap-1.5">
                  <Input
                    label={
                      <span className="flex items-center gap-1.5 text-xs text-ink-2">
                        <AgentIcon target="herdr-app" size={14} />
                        {t('plugins.launcherEntry')}
                        <span className="font-normal text-ink-3">{t('plugins.optional')}</span>
                      </span>
                    }
                    placeholder="bin/lantern-app"
                    size="sm"
                    className="font-mono"
                    value={launcherEntry}
                    disabled={busy}
                    spellCheck={false}
                    aria-describedby={launcherHelpId}
                    onChange={(e) => {
                      setLauncherEntry(e.target.value);
                      setError('');
                    }}
                  />
                  <p id={launcherHelpId} className="text-xs text-ink-3">
                    {t('plugins.launcherHelp')}
                  </p>
                </div>
              )}
            </div>
            {(busy || error) && <SourceHint active={busy} />}
          </form>
        ) : (
          <>
            {discovery.warnings?.map((warning) => (
              <div key={warning} role="note" className="ss-note warn">
                <TriangleAlert size={16} aria-hidden="true" />
                <span className="flex-1">{warning}</span>
              </div>
            ))}
            {canBatchMods ? (
              <div className="flex flex-col gap-2" role="group" aria-labelledby={chooseId}>
                <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                  <span id={chooseId} className="text-[13px] font-semibold">
                    {t('plugins.herdrChooseMods')}
                  </span>
                  <span className="text-xs text-ink-3" aria-live="polite">
                    {t('resources.select.count', { count: chosenMods.length })}
                  </span>
                  <button
                    type="button"
                    className="ss-more ml-auto"
                    disabled={busy || !availableMods.length}
                    onClick={() =>
                      setSelectedMods(
                        new Set(allModsSelected ? [] : availableMods.map((c) => c.name)),
                      )
                    }
                  >
                    {t(allModsSelected ? 'resources.select.clear' : 'resources.select.selectAll')}
                  </button>
                </div>
                <p className="text-xs text-ink-3">{t('plugins.herdrBatchHelp')}</p>
                <div className="ss-list !shadow-none">
                  {discovery.candidates.map((c) => {
                    const on = selectedMods.has(c.name);
                    const info = c.problem
                      ? keyedText(t, c.problemKey, c.problem, c.problemArgs)
                      : c.description;
                    return (
                      <div
                        key={c.name}
                        className={`ss-choice relative flex-col !gap-0 !p-0 ${on ? 'on' : ''}`}
                      >
                        <Checkbox
                          className="min-h-11 w-full pl-3.5 pr-8 font-mono font-semibold"
                          size="sm"
                          label={c.name}
                          checked={on}
                          disabled={busy || !!c.problem}
                          onChange={(checked) =>
                            setSelectedMods((old) => {
                              const next = new Set(old);
                              if (checked) next.add(c.name);
                              else next.delete(c.name);
                              return next;
                            })
                          }
                        />
                        <span
                          className="pointer-events-none absolute right-3 top-3 text-ink-3"
                          aria-hidden="true"
                        >
                          <AgentIcon target="herdr-app" size={17} />
                        </span>
                        {info && (
                          <p
                            className={`pb-2 pl-9 text-xs ${c.problem ? 'text-bad' : 'text-ink-2'}`}
                          >
                            {info}
                          </p>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            ) : discovery.candidates.length === 1 || (extending && selected) ? (
              (() => {
                const c = selected ?? discovery.candidates[0];
                const tag = candidateTag(c);
                const text = keyedText(t, c.problemKey, c.problem, c.problemArgs) || c.description;
                return (
                  <div
                    className={`flex gap-3 rounded-lg bg-sunken px-4 ${extending || !text ? 'items-center py-3' : 'items-start py-3.5'}`}
                  >
                    {isHerdrOnly(c) ? (
                      <span className="ss-cat size-8 rounded-lg bg-surface">
                        <AgentIcon target="herdr-app" size={18} />
                      </span>
                    ) : (
                      <span className="ss-cat plugin size-8 rounded-lg">
                        <Package size={16} />
                      </span>
                    )}
                    <span className="flex min-w-0 flex-1 flex-col gap-1">
                      <span className="flex items-center gap-2">
                        <span className="font-mono text-[13px] font-semibold tracking-[-0.015em]">
                          {c.name}
                        </span>
                        {tag && <span className="ss-tag bg-surface">{tag}</span>}
                      </span>
                      {!extending && text && (
                        <span
                          className={`text-[13px] leading-normal ${c.problem ? 'text-bad' : 'text-ink-2'}`}
                        >
                          {text}
                        </span>
                      )}
                    </span>
                  </div>
                );
              })()
            ) : (
              <div className="flex flex-col gap-2" role="radiogroup" aria-labelledby={chooseId}>
                <span id={chooseId} className="text-[13px] font-semibold">
                  {t('plugins.choose')}
                </span>
                <div className="ss-list !shadow-none">
                  {discovery.candidates.map((c) => {
                    const on = name === c.name;
                    const tag = candidateTag(c);
                    const text =
                      keyedText(t, c.problemKey, c.problem, c.problemArgs) || c.description;
                    return (
                      <button
                        key={c.name}
                        type="button"
                        role="radio"
                        aria-checked={on}
                        disabled={busy || !!c.problem}
                        className={`ss-choice ${on ? 'on' : ''}`}
                        onClick={() => {
                          if (on) return;
                          setName(c.name);
                          setTargets(defaultTargets(c));
                        }}
                      >
                        <span className={`ss-chk rad mt-px ${on ? 'on' : ''}`} />
                        <span className="flex min-w-0 flex-1 flex-col gap-[3px]">
                          <span className="flex items-center gap-2">
                            <span className="font-mono text-[13px] font-semibold tracking-[-0.015em]">
                              {c.name}
                            </span>
                            {tag && <span className="ss-tag">{tag}</span>}
                          </span>
                          {text && (
                            <span
                              className={`text-[13px] leading-[1.45] ${c.problem ? 'text-bad' : 'text-ink-2'}`}
                            >
                              {text}
                            </span>
                          )}
                        </span>
                        {c.targets.length > 0 && (
                          <AgentStack
                            targets={c.targets}
                            tileClassName="!size-[22px]"
                            className="shrink-0 pt-px"
                            aria-hidden="true"
                          />
                        )}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}
            {!canBatchMods && selected && !extending && (
              <Input
                label={t('resources.col.name')}
                value={packageName}
                placeholder={selected.name}
                disabled={busy}
                onChange={(e) => setPackageName(e.target.value)}
              />
            )}
            {canAttachPanel && (
              <div className="flex flex-col gap-1.5">
                <Checkbox
                  label={t(
                    placementCandidate?.herdrPanel
                      ? 'plugins.herdrPanelAttach'
                      : 'plugins.herdrDockAttach',
                  )}
                  checked={attachPanel}
                  disabled={busy}
                  onChange={(checked) => {
                    setAttachPanel(checked);
                    setError('');
                  }}
                />
                <p className="text-xs text-ink-3">
                  {t(
                    placementCandidate?.herdrPanel
                      ? 'plugins.herdrPanelAttachHelp'
                      : 'plugins.herdrDockAttachHelp',
                  )}
                </p>
              </div>
            )}
            {!canBatchMods && selected && !selected.problem && (
              <div className="flex flex-col gap-2">
                <div className="flex items-baseline gap-2">
                  <span className="text-[13px] font-semibold">{t('plugins.targets')}</span>
                  {!herdrOnly && (
                    <span className="text-xs text-ink-3">
                      {extending && usable.length > addable.length
                        ? t(plural('plugins.targetsExtend', addable.length), {
                            bound: usable.length - addable.length,
                            count: addable.length,
                          })
                        : t(plural('plugins.targetsUsable', reasons.length), {
                            count: usable.length,
                            total: reasons.length,
                          })}
                    </span>
                  )}
                  {!extending && !herdrOnly && addable.length > 1 && (
                    <button
                      type="button"
                      className="ss-more ml-auto"
                      disabled={busy}
                      onClick={() =>
                        setTargets((old) =>
                          allAdded
                            ? old.filter((x) => !addable.includes(x))
                            : [...old, ...addable.filter((x) => !old.includes(x))],
                        )
                      }
                    >
                      {t(allAdded ? 'resources.select.clear' : 'resources.select.selectAll')}
                    </button>
                  )}
                </div>
                {usable.length > 0 && (
                  <div className="ss-checks">
                    {usable.map(({ target, definition }) => {
                      // Already bound: shown ticked so the whole picture is here, locked because unticking belongs to the list.
                      const has = !!bound?.[target];
                      const on = has || targets.includes(target);
                      return (
                        <button
                          key={target}
                          type="button"
                          role="checkbox"
                          aria-checked={on}
                          aria-label={definition.label}
                          disabled={busy || has}
                          onClick={() => toggle(target)}
                        >
                          <span className={`ss-chk ${on ? 'on' : ''} ${has ? 'has' : ''}`}>
                            {on && <Check size={11} strokeWidth={3.5} />}
                          </span>
                          <AgentIcon target={target} size={18} />
                          <span className={`nm ${has ? 'text-ink-3' : ''}`}>
                            {definition.label}
                          </span>
                          <span className="pt">
                            {has
                              ? t('plugins.targetBound')
                              : selected.targetInfo?.[target]?.components.join(' · ')}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                )}
                {herdrOnly && (
                  <p className="text-xs leading-normal text-ink-3">
                    {t('plugins.herdrPreviewHelp')}
                  </p>
                )}
                {/* With nothing to tick the reasons are the whole answer, so they show at once; otherwise they wait behind a count. */}
                {blocked.length > 0 && usable.length > 0 && (
                  <button
                    type="button"
                    aria-expanded={blockedOpen}
                    className="flex min-h-9 items-center gap-2 self-start px-1 text-ink-2"
                    onClick={() => setBlockedOpen(!blockedOpen)}
                  >
                    <ChevronRight
                      size={15}
                      className={`transition-transform duration-150 ease-out motion-reduce:transition-none ${blockedOpen ? 'rotate-90' : ''}`}
                    />
                    <span className="text-[13px] font-semibold">{t('plugins.targetsBlocked')}</span>
                    <span className="rounded-full bg-sunken px-[7px] py-0.5 text-[11px] font-medium">
                      {blocked.length}
                    </span>
                    {!blockedOpen && (
                      <span className="ml-1 flex gap-1 opacity-55" aria-hidden="true">
                        {blocked.map(({ target }) => (
                          <AgentIcon key={target} target={target} size={15} />
                        ))}
                      </span>
                    )}
                  </button>
                )}
                {blocked.length > 0 && (usable.length === 0 || blockedOpen) && (
                  <div className="ss-list !shadow-none">
                    {blocked.map(({ target, definition, reason }) => (
                      <Fragment key={target}>
                        <div className="ss-r !min-h-0 !items-start !gap-2.5 !py-2.5 !pl-4 !pr-3">
                          <span className="mt-px opacity-55">
                            <AgentIcon target={target} size={16} />
                          </span>
                          <span className="w-36 shrink-0 text-[13px] text-ink-2">
                            {definition.label}
                          </span>
                          <span className="min-w-0 flex-1 text-xs leading-normal text-ink-3">
                            {reason}
                          </span>
                          {target === 'opencode' && (
                            <button
                              type="button"
                              className="ss-more shrink-0 !text-xs"
                              aria-expanded={entryOpen}
                              onClick={() => setEntryOpen(!entryOpen)}
                            >
                              {t('plugins.entrySet')}
                            </button>
                          )}
                          <PluginDocsLink target={target} label={definition.label} iconOnly />
                        </div>
                        {/* Only OpenCode needs an entry, and only when it could not work one out, so it is asked here and not of every source. */}
                        {target === 'opencode' && entryOpen && (
                          <form
                            className="ss-r fold !min-h-0 flex-col !items-stretch gap-1.5 !py-3 !pl-[42px] !pr-4"
                            onSubmit={(e) => {
                              e.preventDefault();
                              void discover(true);
                            }}
                          >
                            <div className="flex items-end gap-2">
                              <div className="min-w-0 flex-1">
                                <Input
                                  label={t('plugins.entry')}
                                  placeholder="dist/index.js"
                                  className="font-mono"
                                  value={entry}
                                  disabled={busy}
                                  autoFocus
                                  spellCheck={false}
                                  onChange={(e) => setEntry(e.target.value)}
                                />
                              </div>
                              <Button type="submit" variant="secondary" loading={busy}>
                                {t('plugins.rediscover')}
                              </Button>
                            </div>
                            <p className="text-xs leading-normal text-ink-3">
                              {t('plugins.entryHelp')}
                            </p>
                          </form>
                        )}
                      </Fragment>
                    ))}
                  </div>
                )}
              </div>
            )}
          </>
        )}
        {error && (
          <div role="alert" className="ss-note bad">
            <AlertCircle size={16} aria-hidden="true" />
            <span className="flex-1">{error}</span>
          </div>
        )}
      </div>
      <div className="df !bg-transparent">
        {(discovery || npmSource) && !extending && (
          <Button
            variant="ghost"
            className="mr-auto"
            disabled={busy}
            onClick={() => {
              setDiscovery(null);
              setNpmSource('');
              setError('');
              setEntry('');
              setEntryOpen(false);
              setBlockedOpen(false);
            }}
          >
            {t('common.back')}
          </Button>
        )}
        {/* Beside the button it explains, and only while it applies: nothing ticked adds the plugin without installing it. */}
        {!canBatchMods && selected && !selected.problem && !extending && !chosen.length && (
          <span className="text-xs text-ink-3">{t('plugins.targetsOptional')}</span>
        )}
        <Button variant="ghost" disabled={busy} onClick={onClose}>
          {t('common.cancel')}
        </Button>
        {npmSource ? (
          <Button loading={busy} disabled={!chosen.length} onClick={() => void preview()}>
            {t('plugins.preview')}
          </Button>
        ) : discovery ? (
          <Button
            loading={busy}
            disabled={
              canBatchMods
                ? !chosenMods.length
                : !selected || !!selected.problem || (extending && !chosen.length)
            }
            onClick={() => void preview()}
          >
            {t('plugins.preview')}
          </Button>
        ) : (
          <Button type="submit" form="plugin-source" loading={busy} disabled={!source.trim()}>
            <Search size={15} />
            {t(source.trim().startsWith('npm:') ? 'plugins.npmContinue' : 'plugins.discover')}
          </Button>
        )}
      </div>
    </DialogShell>
  );
}
