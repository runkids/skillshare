import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDownToLine, CirclePause, Folder, Info, Plus, Target as TargetIcon, TriangleAlert } from 'lucide-react';
import { api, type Target } from '../api/client';
import Button from '../components/Button';
import CollectDialog from '../components/CollectDialog';
import EmptyState from '../components/EmptyState';
import { Select } from '../components/Input';
import PageHeader from '../components/PageHeader';
import SegmentedControl from '../components/SegmentedControl';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import FilterSection, { ModePicker } from '../components/targets/FilterSection';
import { targetWarning, targetSkipHints } from '../components/targets/targetView';
import RemoveTargetDialog from '../components/targets/RemoveTargetDialog';
import SkillsOffDialog from '../components/targets/SkillsOffDialog';
import TargetMCP from '../components/targets/TargetMCP';
import TargetHooks from '../components/targets/TargetHooks';
import TargetOmpExtensions from '../components/targets/TargetOmpExtensions';
import TargetPiExtensions from '../components/targets/TargetPiExtensions';
import { isOmpTarget, ompExtensionsApi } from '../api/ompExtensions';
import { isPiTarget, piExtensionsApi } from '../api/piExtensions';
import { hookAgentOf, hookCount, scopePaths } from '../components/hooks/hooksView';
import TargetInstructions from '../components/instructions/TargetInstructions';
import AddFileDialog from '../components/targetFiles/AddFileDialog';
import FileTabMenu from '../components/targetFiles/FileTabMenu';
import TargetFileTab from '../components/targetFiles/TargetFileTab';
import { mcpClient, serverCount } from '../components/mcp/mcpView';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { fileName, shortenHome } from '../lib/paths';
import { useT, plural } from '../i18n';
import { useAvailableTargetsQuery, useHooksQuery, useMcpQuery } from '../hooks/useSharedQueries';
import { invalidate } from '../lib/queryEvents';

type Kind = 'skill' | 'agent';
// File tabs (the instruction file first) past this many go into a menu.
const MAX_FILE_TABS = 3;
const draftOf = (target: Target) => ({
  include: target.include ?? [], exclude: target.exclude ?? [], mode: target.mode || 'merge', naming: target.targetNaming || 'flat',
  agentInclude: target.agentInclude ?? [], agentExclude: target.agentExclude ?? [], agentMode: target.agentMode || 'merge',
  agentExtension: target.agentExtension ?? '',
});
type Draft = ReturnType<typeof draftOf>;
const same = (a: string[], b: string[]) => a.length === b.length && a.every((x, i) => x === b[i]);

export default function TargetDetailPage() {
  const { name = '' } = useParams();
  const t = useT();
  const { data, isPending, error } = useQuery({ queryKey: queryKeys.targets.all, queryFn: () => api.listTargets(), staleTime: staleTimes.targets });
  const target = data?.targets.find((x) => x.name === name);

  if (isPending) return <PageSkeleton />;
  if (error) return <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>;
  if (!target) {
    return (
      <EmptyState
        icon={TargetIcon}
        title={t('targetDetail.notFound', { name })}
        action={<Link to="/targets"><Button variant="secondary">{t('targetDetail.backToTargets')}</Button></Link>}
      />
    );
  }
  return <TargetEditor key={name} target={target} targets={data.targets} />;
}

function TargetEditor({ target, targets }: { target: Target; targets: Target[] }) {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [params] = useSearchParams();
  const mcp = useMcpQuery();
  const hooks = useHooksQuery();
  // Hooks are managed per Agent; a project target reads its project's hooks, never the global ones.
  const hookAgent = hookAgentOf(target.name, target.agent);
  const hooksPath = hookAgent && hooks.data && scopePaths(hooks.data, target.project)[hookAgent];
  // Only an Agent that has an MCP file in this scope (global, or the -p project) gets the tab.
  const client = mcpClient(target.name);
  const mcpPath = mcp.data?.paths[client];
  // Until the list arrives, loading or failing, the tab stays so it can say which.
  const filePath = params.get('tab') === 'file' ? params.get('path') ?? '' : '';
  const pi = isPiTarget(target);
  const omp = isOmpTarget(target);
  // Pi and Oh My Pi both switch their extensions here.
  const hasExtensions = pi || omp;
  const tab: Kind | 'mcp' | 'hooks' | 'extensions' | 'instructions' | 'file' = filePath ? 'file' : params.get('tab') === 'instructions' ? 'instructions'
    : hasExtensions && params.get('tab') === 'extensions' ? 'extensions'
    : target.agentPath && params.get('tab') === 'agents' ? 'agent' : params.get('tab') === 'mcp' && (mcpPath || !mcp.data) ? 'mcp' : params.get('tab') === 'hooks' && hookAgent && (hooksPath || !hooks.data) ? 'hooks' : 'skill';
  const kind: Kind = tab === 'agent' ? 'agent' : 'skill';
  const tabs = (['skill', 'agent', 'mcp', 'hooks', 'extensions', 'instructions'] as const).filter((k) => k === 'skill' || k === 'instructions' || (k === 'extensions' ? hasExtensions : k === 'agent' ? target.agentPath : k === 'hooks' ? hookAgent && (hooksPath || tab === 'hooks') : mcpPath || tab === 'mcp'));
  const instructions = useQuery({ queryKey: queryKeys.instructions.target(target.name), queryFn: () => api.getTargetInstructions(target.name) });
  const files = useQuery({ queryKey: queryKeys.targetFiles.list(target.name), queryFn: () => api.listTargetFiles(target.name) });
  // Asks Pi for its version, so it runs only once the tab is opened; the count stays after that.
  const piExtensions = useQuery({ queryKey: queryKeys.piExtensions(target.name), queryFn: () => piExtensionsApi.get(target.name), enabled: pi && tab === 'extensions' });
  const ompExtensions = useQuery({ queryKey: queryKeys.ompExtensions(target.name), queryFn: () => ompExtensionsApi.get(target.name), enabled: omp && tab === 'extensions' });
  const syncTab = tab === 'skill' || tab === 'agent';
  const saved = draftOf(target);
  const [draft, setDraft] = useState<Draft>(saved);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [collecting, setCollecting] = useState(false);
  const [stoppingSkills, setStoppingSkills] = useState(false);
  const [resuming, setResuming] = useState(false);
  const [addingFile, setAddingFile] = useState(false);
  const skillsOn = target.skillsEnabled !== false;
  const available = useAvailableTargetsQuery({ enabled: tab === 'skill' });
  // While skills are on the target list leaves skillsReadFrom out, so the confirm dialog asks available-targets.
  const readsFrom = (target.skillsReadFrom ?? available.data?.targets.find((a) => a.name === target.name)?.readsFrom ?? []).filter((n) => n !== target.name);
  const readFromName = readsFrom[0];
  const readFrom = targets.find((x) => x.name === readFromName);
  // Everyone else reading this folder, so turning it off shows who loses the skills with it.
  const readerEntries = available.data?.targets.filter((a) => a.name !== target.name && a.readsFrom?.includes(target.name)) ?? [];
  const readerOff = (name: string) => targets.find((x) => x.name === name)?.skillsEnabled === false;
  const readers = {
    off: readerEntries.filter((a) => a.installed && readerOff(a.name)).map((a) => a.name),
    local: readerEntries.filter((a) => !a.installed && a.detected).map((a) => a.name),
    on: readerEntries.filter((a) => a.installed && !readerOff(a.name)).map((a) => a.name),
  };
  const { data: extData } = useQuery({ queryKey: queryKeys.extrasExtensions, queryFn: () => api.listExtraExtensions(), staleTime: staleTimes.extras, enabled: syncTab });
  const extensions = extData?.extensions ?? [];

  // Preview the draft filters once typing settles.
  const [filters, setFilters] = useState(draft);
  useEffect(() => {
    const id = setTimeout(() => setFilters(draft), 400);
    return () => clearTimeout(id);
  }, [draft]);
  const preview = useQuery({
    queryKey: queryKeys.syncMatrixPreview(target.name, filters.include, filters.exclude, filters.agentInclude, filters.agentExclude),
    queryFn: () => api.previewSyncMatrix(target.name, filters.include, filters.exclude, filters.agentInclude, filters.agentExclude),
    placeholderData: keepPreviousData,
  });
  const entriesOf = (k: Kind) => (preview.data?.entries ?? []).filter((e) => (e.kind === 'agent') === (k === 'agent') && e.status !== 'na');
  const entries = entriesOf(kind);

  const payload: Parameters<typeof api.updateTarget>[1] = {
    ...(!same(draft.include, saved.include) && { include: draft.include }),
    ...(!same(draft.exclude, saved.exclude) && { exclude: draft.exclude }),
    ...(draft.mode !== saved.mode && { mode: draft.mode }),
    ...(draft.naming !== saved.naming && { target_naming: draft.naming }),
    ...(!same(draft.agentInclude, saved.agentInclude) && { agent_include: draft.agentInclude }),
    ...(!same(draft.agentExclude, saved.agentExclude) && { agent_exclude: draft.agentExclude }),
    ...(draft.agentMode !== saved.agentMode && { agent_mode: draft.agentMode }),
    ...(draft.agentExtension !== saved.agentExtension && { agent_extension: draft.agentExtension }),
  };
  const dirty = Object.keys(payload).length > 0;
  const save = async () => {
    setSaving(true);
    try {
      await api.updateTarget(target.name, payload);
      void invalidate(queryClient, 'targetsChanged');
      toast(t('targetDetail.saved', { name: target.name }), 'success');
    } catch (err) {
      toast((err as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const agent = kind === 'agent';
  const include = agent ? draft.agentInclude : draft.include;
  const exclude = agent ? draft.agentExclude : draft.exclude;
  const mode = agent ? draft.agentMode : draft.mode;
  const setFiltersFor = (next: { include: string[]; exclude: string[] }) =>
    setDraft(agent ? { ...draft, agentInclude: next.include, agentExclude: next.exclude } : { ...draft, ...next });
  const local = agent ? target.agentLocalCount ?? 0 : target.localCount;
  // prefixed rewrites name: inside each copy, so it cannot outlive copy mode.
  const prefixedOff = draft.mode !== 'copy';
  const setSkillsMode = (m: string) => {
    let naming = draft.naming;
    if (m !== 'copy' && naming === 'prefixed') naming = saved.naming === 'prefixed' ? 'flat' : saved.naming;
    setDraft({ ...draft, mode: m, naming });
  };
  const namingOptions = [
    { value: 'flat', label: 'flat' },
    { value: 'standard', label: 'standard' },
    { value: 'prefixed', label: 'prefixed', disabled: prefixedOff, title: prefixedOff ? t('targetDetail.prefixedNeedsCopyTitle') : undefined },
  ];
  // Another target writing this folder with other settings than the draft: each sync would undo the other's.
  const sharing = (target.skillsSharedWith ?? []).map((n) => targets.find((x) => x.name === n)).filter((x): x is Target => !!x);
  const modeClash = agent ? undefined : sharing.find((o) => o.mode !== draft.mode);
  // Symlink links the whole folder, so naming only matters once the modes agree on something else.
  const namingClash = modeClash ? undefined : sharing.find((o) => o.targetNaming !== draft.naming);
  const sameList = (a: string[], b: string[]) => a.length === b.length && [...a].sort().join('\n') === [...b].sort().join('\n');
  const filtersClash = modeClash || draft.mode === 'symlink' ? undefined : sharing.find((o) => !sameList(o.include, draft.include) || !sameList(o.exclude, draft.exclude));
  const sharedNote = (o: Target, field: 'mode' | 'naming') => {
    const theirs = field === 'mode' ? o.mode : o.targetNaming;
    return (
      <div className="ss-note warn">
        <TriangleAlert size={16} />
        <div className="flex min-w-0 flex-col gap-2">
          <span>{t(`targetDetail.shared.${field}`, { other: o.name, path: shortenHome(target.path), theirs, mine: draft[field] })}</span>
          <Button variant="secondary" size="sm" className="self-start" onClick={() => (field === 'mode' ? setSkillsMode(theirs) : setDraft({ ...draft, naming: theirs }))} disabled={saving}>
            {t('targetDetail.shared.use', { value: theirs, other: o.name })}
          </Button>
        </div>
      </div>
    );
  };

  const tabCount = (k: (typeof tabs)[number]) =>
    k === 'skill' && !skillsOn ? null : (k === 'mcp' ? mcp.data && serverCount(mcp.data, client)
      : k === 'hooks' ? hooks.data && hookAgent && hookCount(hooks.data, hookAgent, target.project)
      : k === 'instructions' ? instructions.data?.read_order.filter((e) => e.read).length
        : k === 'extensions' ? (omp ? ompExtensions.data?.rows.length : piExtensions.data && [...piExtensions.data.packages, ...piExtensions.data.folders].reduce((n, p) => n + p.rows.length, 0))
        : entriesOf(k).length) || null;
  // Name the tab after the file this target actually reads (CLAUDE.md, GEMINI.md, …).
  const instructionsTab = instructions.data?.supported && instructions.data.path ? fileName(instructions.data.path) : 'AGENTS.md';
  const tabLabel = (k: (typeof tabs)[number]) => (k === 'agent' ? 'Agents' : k === 'mcp' ? 'MCP' : k === 'hooks' ? 'Hooks' : k === 'extensions' ? 'Extensions' : k === 'instructions' ? instructionsTab : 'Skills');
  const tabLink = (k: (typeof tabs)[number]) => (k === 'agent' ? '?tab=agents' : k === 'mcp' ? '?tab=mcp' : k === 'hooks' ? '?tab=hooks' : k === 'extensions' ? '?tab=extensions' : k === 'instructions' ? '?tab=instructions' : '?');
  // The instruction file and the other files the tool reads; the open one always shows, taking the last slot if it has to.
  const fileTabs = [
    ...(tabs.includes('instructions') ? [{ id: 'instructions', label: instructionsTab, to: tabLink('instructions') }] : []),
    ...(files.data?.files ?? []).map((f) => ({ id: `file:${f.path}`, label: f.path, to: `?tab=file&path=${encodeURIComponent(f.path)}` })),
  ];
  const activeFile = fileTabs.find((f) => f.id === (tab === 'file' ? `file:${filePath}` : tab));
  let shownFiles = fileTabs.slice(0, MAX_FILE_TABS);
  if (activeFile && !shownFiles.includes(activeFile)) shownFiles = [...shownFiles.slice(0, -1), activeFile];
  const hiddenFiles = fileTabs.filter((f) => !shownFiles.includes(f));
  const openFile = files.data?.files.find((f) => f.path === filePath);
  const resume = async () => {
    setResuming(true);
    try {
      await api.updateTarget(target.name, { skills_enabled: true });
      void invalidate(queryClient, 'targetsChanged');
      toast(t('targetDetail.skillsOff.resumed', { name: target.name }), 'success');
    } catch (err) {
      toast((err as Error).message, 'error');
    } finally {
      setResuming(false);
    }
  };
  const subtitle = tab === 'file' ? openFile?.abs ?? '' : tab === 'mcp' ? mcpPath ?? '' : tab === 'hooks' ? hooksPath || '' : tab === 'instructions' ? (instructions.data?.supported ? instructions.data.path ?? '' : '') : agent ? target.agentPath ?? '' : target.path;
  return (
    <div className="animate-fade-in">
      <PageHeader
        crumbs={[{ label: t('targets.title'), to: '/targets' }, { label: target.name }]}
        title={target.name}
        subtitle={(subtitle || target.cli) && (
          <span className="flex flex-wrap items-center gap-x-2">
            {subtitle && <span className="font-mono">{shortenHome(subtitle)}</span>}
            {subtitle && target.cli && <span aria-hidden>·</span>}
            {target.cli && <span>{t('targets.add.accountCli')} <span className="font-mono">{shortenHome(target.cli)}</span></span>}
          </span>
        )}
        actions={
          <>
            {tab === 'skill' && <Link to={`/skills?tab=analyze&target=${encodeURIComponent(target.name)}`} className="ss-btn ghost">{t('analyze.open')}</Link>}
            <Button variant="ghost" onClick={() => setRemoving(true)}>{t('targetDetail.remove')}</Button>
            {/* A switch on the MCP tab saves as it flips; the instructions tab saves its own file. */}
            {syncTab && (skillsOn || agent) && <Button variant="primary" onClick={save} loading={saving} disabled={!dirty}>{t('common.save')}</Button>}
          </>
        }
      />

      {target.warnings?.map((warning) => {
        const diagnostic = targetWarning(warning);
        return <div key={warning} className="ss-note warn mb-3" role="status"><span>{diagnostic ? <><span className="font-mono">{diagnostic.path}</span>: {t(diagnostic.key, { name: target.name })}</> : warning}</span></div>;
      })}

      {tabs.length > 1 && (
        <nav className="ss-tabs mb-7" aria-label={t('targetDetail.tabs')}>
          {tabs.filter((k) => k !== 'instructions').map((k) => (
            <Link key={k} to={tabLink(k)} replace className={tab === k ? 'on' : ''}>
              {tabLabel(k)}
              {tabCount(k) !== null && <span className="ss-cnt">{tabCount(k)}</span>}
            </Link>
          ))}
          {shownFiles.map((f) => (
            <Link key={f.id} to={f.to} replace className={f === activeFile ? 'on' : ''} title={f.label}>
              <span className="max-w-[180px] truncate">{f.label}</span>
              {f.id === 'instructions' && tabCount('instructions') !== null && <span className="ss-cnt">{tabCount('instructions')}</span>}
            </Link>
          ))}
          {hiddenFiles.length > 0 && <FileTabMenu files={hiddenFiles} />}
          {files.data?.root && (
            <span>
              <button type="button" className="grid h-6 w-6 place-items-center rounded-full border border-dashed border-line-2 text-ink-3 hover:border-ink hover:text-ink" aria-label={t('targetFiles.add')} title={t('targetFiles.add')} onClick={() => setAddingFile(true)}>
                <Plus size={14} />
              </button>
            </span>
          )}
        </nav>
      )}

      {tab === 'file' ? (
        <TargetFileTab key={filePath} target={target.name} path={filePath} project={files.data?.project ?? false} />
      ) : tab === 'instructions' ? (
        <TargetInstructions name={target.name} />
      ) : tab === 'mcp' ? (
        mcp.data ? <TargetMCP name={client} data={mcp.data} /> : mcp.error ? <div className="ss-note bad"><span className="flex-1">{mcp.error.message}</span></div> : <PageSkeleton />
      ) : tab === 'extensions' ? (
        omp ? <TargetOmpExtensions name={target.name} /> : <TargetPiExtensions name={target.name} />
      ) : tab === 'hooks' && hookAgent ? (
        hooks.data ? <TargetHooks agent={hookAgent} data={hooks.data} project={target.project} /> : hooks.error ? <div className="ss-note bad"><span className="flex-1">{hooks.error.message}</span></div> : <PageSkeleton />
      ) : !agent && !skillsOn ? (
        <div className="ss-empty !py-16">
          <CirclePause size={24} className="text-ink-3" />
          <h3 className="font-semibold text-ink">{t('targetDetail.skillsOff.title', { name: target.name })}</h3>
          <p className="max-w-md text-[13px]">
            {t('targetDetail.skillsOff.noWrites', { path: shortenHome(target.path) })}
            <br />
            {readFrom
              ? t(plural('targetDetail.skillsOff.readsFrom', readFrom.linkedCount), { name: target.name, from: readFrom.name, path: shortenHome(readFrom.path), count: readFrom.linkedCount })
              : t('targetDetail.skillsOff.generic')}
          </p>
          <div className="mt-2"><Button variant="secondary" onClick={resume} loading={resuming}>{t('targetDetail.skillsOff.resume')}</Button></div>
        </div>
      ) : (
        <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] items-start gap-12">
          <section className="flex flex-col gap-5">
            <div className="flex items-center justify-between gap-3">
              <h2 className="ss-h2">{t('targetDetail.whatSyncs')}</h2>
              {!agent && (
                <Button variant="ghost" size="sm" onClick={() => setStoppingSkills(true)} disabled={saving}>
                  <CirclePause size={15} />{t('targetDetail.skillsOff.stop')}
                </Button>
              )}
            </div>
            <FilterSection kind={kind} mode={mode} name={target.name} include={include} exclude={exclude} onChange={setFiltersFor} entries={entries} loaded={Boolean(preview.data)} loading={preview.isPending} error={preview.error} disabled={saving} alsoReadBy={agent ? undefined : target.skillsAlsoReadBy} readsFrom={agent ? undefined : readsFrom} />
            {!agent && filtersClash && (
              <div className="ss-note warn">
                <TriangleAlert size={16} />
                <div className="flex min-w-0 flex-col gap-2">
                  <span>{t('targetDetail.shared.filters', { other: filtersClash.name, path: shortenHome(target.path) })}</span>
                  <Button variant="secondary" size="sm" className="self-start" onClick={() => setDraft({ ...draft, include: filtersClash.include, exclude: filtersClash.exclude })} disabled={saving}>
                    {t('targetDetail.shared.useFilters', { other: filtersClash.name })}
                  </Button>
                </div>
              </div>
            )}
          </section>

          <aside className="flex flex-col gap-7">
            {agent && (
              <div className="flex flex-col gap-1.5">
                <span className="text-[13px] font-semibold">{t('targetDetail.agentsFolder')}</span>
                <span className="flex items-center gap-2 font-mono text-[13px]"><Folder size={15} className="shrink-0 text-ink-3" />{shortenHome(target.agentPath ?? '')}</span>
                <span className="text-[12.5px] text-ink-3">{t('targetDetail.agentsFolderHint')}</span>
              </div>
            )}
            {agent && (
              <div className="flex flex-col gap-1.5">
                <span className="text-[13px] font-semibold">{t('extras.modal.colExtension')}</span>
                <Select
                  value={draft.agentExtension}
                  // An extension converts each agent, so it always writes copies
                  onChange={(v) => setDraft({ ...draft, agentExtension: v, ...(v ? { agentMode: 'copy' } : {}) })}
                  options={[
                    { value: '', label: t('extras.noExtension') },
                    ...[...new Set([...extensions, ...(draft.agentExtension ? [draft.agentExtension] : [])])].map((e) => ({ value: e, label: e })),
                  ]}
                  disabled={saving || (extensions.length === 0 && !draft.agentExtension)}
                />
                <span className="text-[12.5px] text-ink-3">
                  {t('extras.hint.extension')}{' '}
                  {extensions.length === 0 && <Link to="/config?tab=extensions" className="font-semibold text-ink-2 hover:text-ink">{t('extras.installExtensionHint')}</Link>}
                </span>
              </div>
            )}
            <div className="flex flex-col gap-3">
              <h2 className="ss-h2">{t('targetDetail.syncMode')}</h2>
              <ModePicker kind={kind} mode={mode} onChange={(m) => (agent ? setDraft({ ...draft, agentMode: m }) : setSkillsMode(m))} disabled={saving || (agent && draft.agentExtension !== '')} />
              {modeClash && sharedNote(modeClash, 'mode')}
            </div>

            {!agent && draft.mode !== 'symlink' && (
              <div className="flex flex-col gap-1.5">
                <div className="flex items-center gap-4">
                  <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex items-center gap-2 text-[13px] font-semibold">
                      {t('targetDetail.naming')}
                      {draft.naming !== saved.naming && <span className="ss-tag inf">{t('targetDetail.pending')}</span>}
                    </span>
                    <span className="text-[13px] text-ink-2">{t(draft.naming === 'standard' ? 'targetDetail.namingStandard' : draft.naming === 'prefixed' ? 'targetDetail.namingPrefixed' : 'targetDetail.namingFlat')}</span>
                  </div>
                  <SegmentedControl value={draft.naming} onChange={(naming) => setDraft({ ...draft, naming })} options={namingOptions} />
                </div>
                {prefixedOff && (
                  <span className="flex items-center gap-1.5 text-[13px] text-ink-3"><Info size={14} className="shrink-0" />{t('targetDetail.prefixedNeedsCopy')}</span>
                )}
                {namingClash && sharedNote(namingClash, 'naming')}
                {targetSkipHints(target).length > 0 && (
                  <span className="text-[13px] text-warn">
                    {t(plural('sync.skipped', target.skippedSkillCount ?? 0), { count: target.skippedSkillCount ?? 0, name: target.name })}{' '}
                    {targetSkipHints(target).map((key) => t(key, { name: target.name })).join(' ')}
                  </span>
                )}
              </div>
            )}

            {local > 0 && (
              <div className="ss-box flex flex-col gap-3">
                <span className="flex items-center gap-2 font-semibold"><ArrowDownToLine size={16} />{t('targetDetail.collect')}</span>
                <p className="text-[13px] text-ink-2">{t(plural(`targetDetail.collectHint.${agent ? 'agents' : 'skills'}`, local), { count: local })}</p>
                <Button variant="secondary" onClick={() => setCollecting(true)}>
                  {t(plural(`collectDialog.run.${kind}`, local), { count: local })}
                </Button>
              </div>
            )}
          </aside>
        </div>
      )}

      {removing && (
        <RemoveTargetDialog
          target={target}
          onClose={() => setRemoving(false)}
          onRemoved={(warnings) => {
            void invalidate(queryClient, 'targetsChanged');
            toast(t('targets.targetRemoved', { name: target.name }), 'success');
            warnings.forEach((warning) => toast(warning, 'warning'));
            navigate('/targets');
          }}
        />
      )}
      {stoppingSkills && (
        <SkillsOffDialog
          target={target}
          readFrom={readFrom}
          readers={readers}
          managed={tabs.filter((k) => k !== 'skill').map(tabLabel)}
          onClose={() => setStoppingSkills(false)}
          onStopped={(removed) => {
            setStoppingSkills(false);
            void invalidate(queryClient, 'targetsChanged');
            toast(t(plural('targetDetail.skillsOff.stopped', removed), { name: target.name, count: removed }), 'success');
          }}
        />
      )}
      {addingFile && files.data?.root && (
        <AddFileDialog
          target={target.name}
          root={files.data.root}
          onClose={() => setAddingFile(false)}
          onAdded={(path) => {
            setAddingFile(false);
            navigate(`?tab=file&path=${encodeURIComponent(path)}`, { replace: true });
          }}
        />
      )}
      {collecting && <CollectDialog target={target.name} kind={kind} onClose={() => setCollecting(false)} />}
    </div>
  );
}
