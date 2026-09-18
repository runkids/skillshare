import { useMemo, useState } from 'react';
import { Link, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import type { Components } from 'react-markdown';
import {
  ChevronDown, CircleArrowUp, CircleCheck, Copy, Ellipsis, ExternalLink, File, FileCode2, FileText, Folder,
  FolderOpen, Github, Globe, Pencil, Power, RefreshCw, ShieldAlert, ShieldCheck, Trash2, TriangleAlert, X,
} from 'lucide-react';
import { api, type AuditResult, type Skill } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { clearAuditCache } from '../lib/auditCache';
import { SEV, parseFindings, thresholdOf } from '../lib/auditMessage';
import { parseSkillMarkdown } from '../lib/frontmatter';
import { isMarkdown } from '../lib/highlight';
import { parseRemoteURL } from '../lib/parseRemoteURL';
import { resourceHref } from '../lib/resourceNames';
import { targetFilterPatch } from '../lib/targetFilter';
import { useSyncMatrix } from '../hooks/useSyncMatrix';
import { formatDateTime, formatRelativeTime, useI18n, useT } from '../i18n';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import FindingList from '../components/FindingList';
import PageHeader from '../components/PageHeader';
import CodeView from '../components/CodeView';
import { CODE_EXT, fileTree } from '../lib/fileTree';
import MarkdownView, { ViewToggle } from '../components/MarkdownView';
import { SkillDetailSkeleton } from '../components/Skeleton';
import Spinner from '../components/Spinner';
import { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import { useToast } from '../components/Toast';
import { SkillEditor } from '../components/skill-editor';
import { UninstallDialog } from './ResourcesPage';
import { hasUpdate, updateUnits, useCheckStatuses } from './UpdatePage';

type Tab = 'doc' | 'files' | 'audit';

const SEV_RANK: Record<string, number> = { CRITICAL: 0, HIGH: 1, MEDIUM: 2, LOW: 3, INFO: 4 };

const str = (v: unknown) => (v == null ? '' : Array.isArray(v) ? v.join(', ') : String(v).trim());
const words = (s: string) => s.split(/\s+/).filter(Boolean).length;

export default function ResourceDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const t = useT();
  const requestedKind = pathname.startsWith('/agents/') || searchParams.get('kind') === 'agent'
    ? 'agent'
    : searchParams.get('kind') === 'skill'
      ? 'skill'
      : undefined;
  const { data, isPending, error } = useQuery({
    queryKey: [...queryKeys.skills.detail(name!), requestedKind],
    queryFn: () => api.getResource(name!, requestedKind),
    staleTime: staleTimes.skills,
    enabled: !!name,
  });
  const allSkills = useQuery({ queryKey: queryKeys.skills.all, queryFn: () => api.listSkills(), staleTime: staleTimes.skills });
  const kind = data?.resource.kind;
  const auditQuery = useQuery({
    queryKey: [...queryKeys.audit.skill(name!), kind],
    // The server resolves skills by path under the source dir; nested skills have a flat name in the URL
    queryFn: () => api.auditSkill(kind === 'agent' ? name! : data!.resource.relPath, kind),
    staleTime: staleTimes.auditSkill,
    enabled: !!name && !!kind,
  });
  const [statuses, setStatuses] = useCheckStatuses();
  const [editing, setEditing] = useState(false);
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);
  const [uninstalling, setUninstalling] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [blocked, setBlocked] = useState<string | null>(null);
  const [raw, setRaw] = useState(false);

  const skillMaps = useMemo(() => {
    const byName = new Map<string, Skill>();
    const byFlat = new Map<string, Skill>();
    for (const s of allSkills.data?.resources ?? []) {
      byName.set(s.name, s);
      byFlat.set(s.flatName, s);
    }
    return { byName, byFlat };
  }, [allSkills.data]);

  if (isPending) return <SkillDetailSkeleton />;
  if (error || !data) {
    return (
      <div className="ss-note bad">
        <TriangleAlert size={16} />
        <div className="flex-1"><b>{t('resourceDetail.error.failedToLoad')}</b> {error?.message}</div>
      </div>
    );
  }

  const { resource, skillMdContent = '' } = data;
  const files = data.files ?? [];
  const isAgent = resource.kind === 'agent';
  const listPath = isAgent ? '/agents' : '/skills';
  const { frontmatter, body } = parseSkillMarkdown(skillMdContent);
  const docName = isAgent ? resource.relPath.split('/').pop()! : 'SKILL.md';
  const unit = updateUnits(allSkills.data?.resources ?? [resource], resource.kind)
    .find((u) => u.items.some((i) => i.flatName === resource.flatName));
  const check = statuses.get(resource.name) ?? { status: 'unchecked' as const };
  const updateAvailable = hasUpdate(check);
  const audit = auditQuery.data?.result;

  const requestedTab = searchParams.get('tab');
  const tab: Tab = requestedTab === 'audit' ? 'audit' : requestedTab === 'files' && !isAgent ? 'files' : 'doc';
  const tabSearch = (next: Tab, extra: Record<string, string | number> = {}) => {
    const p = new URLSearchParams();
    const k = searchParams.get('kind');
    if (k) p.set('kind', k);
    if (next !== 'doc') p.set('tab', next);
    for (const [key, v] of Object.entries(extra)) p.set(key, String(v));
    const s = p.toString();
    return s ? `?${s}` : pathname;
  };

  const refreshResource = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name!) });
    await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
  };

  // The check result belongs to the whole update unit, so a repo marks every skill in it
  const markUpToDate = () => {
    const names = unit?.items.map((i) => i.name) ?? [resource.name];
    const checkedAt = new Date().toISOString();
    setStatuses((prev) => {
      const next = new Map(prev);
      for (const n of names) next.set(n, { status: 'up-to-date', checkedAt });
      return next;
    });
  };

  const runUpdate = async (skipAudit: boolean) => {
    setUpdating(true);
    try {
      const target = resource.isInRepo ? resource.relPath.split('/')[0] : isAgent ? resource.flatName : resource.relPath;
      const res = await api.update({ name: target, kind: resource.kind, skipAudit });
      const item = res.results[0];
      if (item?.action === 'updated') {
        setBlocked(null);
        const auditInfo = item.auditRiskLabel
          ? ` · Security: ${item.auditRiskLabel.toUpperCase()}${item.auditRiskScore ? ` (${item.auditRiskScore}/100)` : ''}`
          : '';
        toast(t('resourceDetail.toast.updated', { name: resource.name, message: item.message ?? '', auditInfo }), 'success');
        markUpToDate();
        clearAuditCache(queryClient);
        await refreshResource();
      } else if (item?.action === 'up-to-date') {
        toast(t('resourceDetail.toast.upToDate', { name: resource.name }), 'info');
        markUpToDate();
      } else if (item?.action === 'blocked') {
        setBlocked(item.message ?? '');
      } else if (item?.action === 'error') {
        toast(item.message ?? t('resourceDetail.toast.updateFailed'), 'error');
      } else {
        toast(item?.message ?? t('resourceDetail.toast.skipped'), 'warning');
      }
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setUpdating(false);
    }
  };

  const toggleDisabled = async () => {
    setToggling(true);
    try {
      if (resource.disabled) await api.enableResource(resource.flatName, resource.kind);
      else await api.disableResource(resource.flatName, resource.kind);
      toast(t(resource.disabled ? 'resourceDetail.toast.enabled' : 'resourceDetail.toast.disabled', { name: resource.name }), 'success');
      await refreshResource();
      queryClient.invalidateQueries({ queryKey: ['sync-matrix'] });
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setToggling(false);
    }
  };

  const openInEditor = async () => {
    try {
      const resp = await api.openSkillInEditor(resource.flatName, { kind: resource.kind });
      toast(t('resourceDetail.toast.openedIn', { editor: resp.editor }), 'info');
    } catch (e) {
      toast((e as Error).message, 'error');
    }
  };

  if (editing) {
    return (
      <SkillEditor
        resource={resource}
        docName={docName}
        initialContent={skillMdContent}
        onBack={() => setEditing(false)}
        onSaved={async (next) => {
          queryClient.setQueryData([...queryKeys.skills.detail(name!), requestedKind], (prev: unknown) =>
            prev && typeof prev === 'object' ? { ...prev, skillMdContent: next } : prev);
          await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name!) });
          setEditing(false);
        }}
      />
    );
  }

  const menuItems: ContextMenuItem[] = [
    { key: 'open', label: t('resourceDetail.actions.openInEditor'), icon: <FolderOpen size={14} />, onSelect: openInEditor },
    ...(unit && !updateAvailable && !updating
      ? [{ key: 'update', label: t('resourceDetail.actions.update'), icon: <CircleArrowUp size={14} />, onSelect: () => runUpdate(false) }]
      : []),
    {
      key: 'toggle',
      label: t(resource.disabled ? 'resourceDetail.actions.enable' : 'resourceDetail.actions.disable'),
      icon: resource.disabled ? <CircleCheck size={14} /> : <Power size={14} />,
      onSelect: toggleDisabled,
    },
    {
      key: 'uninstall',
      label: t(resource.isInRepo && !isAgent ? 'resourceDetail.actions.uninstallRepo' : 'resourceDetail.actions.uninstall'),
      icon: <Trash2 size={14} />,
      danger: true,
      onSelect: () => setUninstalling(true),
    },
  ];

  const resolveSkillRef = (ref: string) =>
    skillMaps.byName.get(ref) ?? skillMaps.byFlat.get(`${resource.flatName}__${ref.replace(/\//g, '__')}`);

  const md: Components = {
    a: ({ href, children }) => {
      if (href && !href.startsWith('http') && !href.startsWith('#')) {
        const ref = resolveSkillRef(href);
        if (ref) return <Link to={resourceHref(ref)}>{children}</Link>;
        const file = files.find((f) => f === href || f.endsWith('/' + href));
        if (file && !isAgent) return <Link to={tabSearch('files', { file })}>{children}</Link>;
      }
      return <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>;
    },
  };

  const findingCount = audit?.findings.length ?? 0;
  const tabs = (
    <nav className="ss-tabs" aria-label={t('resourceDetail.tabs.label')}>
      <Link to={tabSearch('doc')} className={tab === 'doc' ? 'on' : ''}>{docName}</Link>
      {!isAgent && (
        <Link to={tabSearch('files')} className={tab === 'files' ? 'on' : ''}>
          {t('resourceDetail.tabs.files')} <span className="ss-cnt">{files.length}</span>
        </Link>
      )}
      <Link to={tabSearch('audit')} className={tab === 'audit' ? 'on' : ''}>
        {t('resourceDetail.tabs.audit')} {findingCount > 0 && <span className="ss-cnt">{findingCount}</span>}
      </Link>
    </nav>
  );

  const description = str(frontmatter.description);

  return (
    <div className="animate-fade-in">
      <PageHeader
        crumbs={[{ label: t(isAgent ? 'layout.nav.agents' : 'layout.nav.skills'), to: listPath }, { label: resource.name }]}
        title={resource.name}
        mono
        subtitle={description && <span className="block max-w-[640px] truncate">{description}</span>}
        actions={
          <>
            <button type="button" className="ss-ib" aria-label={t('resourceDetail.actions.more')} onClick={(e) => {
              const r = e.currentTarget.getBoundingClientRect();
              setMenu({ x: r.left, y: r.bottom + 4 });
            }}>
              <Ellipsis size={16} />
            </button>
            <Button variant="secondary" onClick={() => setEditing(true)}>
              <Pencil size={15} />
              {t('resourceDetail.actions.edit')}
            </Button>
            {/* Also shown while an update started from the menu runs, so it has visible progress */}
            {unit && (updateAvailable || updating) && (
              <Button variant="primary" loading={updating} onClick={() => runUpdate(false)}>
                {!updating && <CircleArrowUp size={15} />}
                {t('resourceDetail.actions.update')}
              </Button>
            )}
          </>
        }
      />

      {resource.disabled && (
        <div className="ss-note warn mb-5">
          <Power size={16} />
          <div className="flex-1">{t(isAgent ? 'resourceDetail.disabled.agent' : 'resourceDetail.disabled.skill')}</div>
          <Button variant="secondary" size="sm" loading={toggling} onClick={toggleDisabled}>{t('resourceDetail.actions.enable')}</Button>
        </div>
      )}

      {tab === 'doc' ? (
        <div className="grid grid-cols-[minmax(0,1fr)_330px] items-start gap-8">
          <div className="flex min-w-0 flex-col gap-[18px]">
            <div className="ss-tabbar">
              {tabs}
              {skillMdContent.trim() && <ViewToggle raw={raw} onChange={setRaw} />}
            </div>
            {!skillMdContent.trim() ? (
              <div className="ss-box !px-[30px] !py-[26px] text-ink-3">{t('resourceDetail.noContent')}</div>
            ) : raw ? (
              // Markdown wraps, so the page scrolls instead of a nested box
              <CodeView content={skillMdContent} lang="md" className="min-h-[330px]" />
            ) : (
              <div className="ss-box !px-[30px] !py-[26px]">
                <MarkdownView size="lg" components={md}>{body.trim() ? body : skillMdContent}</MarkdownView>
              </div>
            )}
          </div>
          <div className="flex flex-col gap-7">
            <MetaBox
              resource={resource}
              frontmatter={frontmatter}
              body={body}
              fileCount={files.length}
              check={check}
              audit={audit}
              auditPending={auditQuery.isPending}
              auditHref={tabSearch('audit')}
            />
            {!resource.disabled && <TargetsSection resource={resource} />}
          </div>
        </div>
      ) : (
        <div className="flex flex-col gap-5">
          {tabs}
          {tab === 'files'
            ? <FilesTab resource={resource} files={files} skillMd={skillMdContent} tabSearch={tabSearch} components={md} raw={raw} onRaw={setRaw} />
            : <AuditTab query={auditQuery} files={files} tabSearch={tabSearch} />}
        </div>
      )}

      <SkillContextMenu open={!!menu} anchorPoint={menu ?? undefined} items={menuItems} onClose={() => setMenu(null)} />
      {uninstalling && (
        <UninstallDialog
          kind={resource.kind}
          selection={[resource]}
          all={allSkills.data?.resources ?? [resource]}
          onClose={(removed) => {
            setUninstalling(false);
            if (removed) navigate(listPath);
          }}
        />
      )}
      {blocked !== null && (
        <BlockedDialog name={resource.name} message={blocked} loading={updating} onSkip={() => runUpdate(true)} onClose={() => setBlocked(null)} />
      )}
    </div>
  );
}

/* -- Sidebar -------------------------------------- */

function MetaBox({ resource, frontmatter, body, fileCount, check, audit, auditPending, auditHref }: {
  resource: Skill;
  frontmatter: Record<string, unknown>;
  body: string;
  fileCount: number;
  check: { status: string; behind?: number };
  audit?: AuditResult;
  auditPending: boolean;
  auditHref: string;
}) {
  const t = useT();
  const { locale } = useI18n();
  const isAgent = resource.kind === 'agent';
  const remote = parseRemoteURL(resource.repoUrl ?? resource.source);
  const compact = new Intl.NumberFormat(locale, { notation: 'compact', maximumFractionDigits: 1 });
  const always = Math.round((str(frontmatter.description).length + str(frontmatter.when_to_use).length) / 4);
  const onDemand = Math.round(body.trim().length / 4);
  const lineCount = body.trim() ? body.trim().split(/\r?\n/).length : 0;
  const behind = check.status === 'behind';
  const updateNote = behind || check.status === 'update-available'
    ? (behind && check.behind ? t('update.check.behind', { count: check.behind }) : t('update.check.updateAvailable'))
    : '';
  const findings = audit?.findings.length ?? 0;

  const rows: [string, React.ReactNode][] = [];
  if (isAgent && str(frontmatter.model)) rows.push([t('resourceDetail.meta.model'), <span className="font-mono">{str(frontmatter.model)}</span>]);
  if (isAgent && str(frontmatter.tools)) rows.push([t('resourceDetail.meta.tools'), <span className="font-mono">{str(frontmatter.tools)}</span>]);
  rows.push([
    t('resourceDetail.metadata.source'),
    remote ? (
      <a href={remote.webURL ?? undefined} target="_blank" rel="noopener noreferrer" className="inline-flex max-w-full items-center gap-1.5 hover:underline">
        {remote.platform === 'github' ? <Github size={14} className="shrink-0" /> : <Globe size={14} className="shrink-0" />}
        <span className="truncate font-mono">{remote.ownerRepo}</span>
        <ExternalLink size={12} className="shrink-0 text-ink-3" />
      </a>
    ) : resource.source ? <span className="block truncate font-mono">{resource.source}</span> : t('resourceDetail.meta.local'),
  ]);
  if (resource.source || resource.isInRepo) {
    rows.push([
      t('resourceDetail.meta.tracked'),
      resource.isInRepo
        ? (resource.branch ? t('resourceDetail.meta.trackedBranch', { branch: resource.branch }) : t('resourceDetail.meta.yes'))
        : t('resourceDetail.meta.no'),
    ]);
  }
  if (resource.version || updateNote) {
    rows.push([
      t('resourceDetail.metadata.version'),
      <>{resource.version && <span className="font-mono">{resource.version}</span>}{updateNote && <span className="text-warn">{resource.version && ' · '}{updateNote}</span>}</>,
    ]);
  }
  if (resource.installedAt) rows.push([t('resourceDetail.metadata.installed'), formatDateTime(resource.installedAt, locale, { dateStyle: 'medium' })]);
  if (str(frontmatter.license)) rows.push([t('resourceDetail.manifest.license'), str(frontmatter.license)]);
  rows.push([t('resourceDetail.metadata.path'), <span className="block truncate font-mono">{resource.relPath}</span>]);
  if (!isAgent) {
    rows.push([t('resourceDetail.meta.size'), [
      t(fileCount === 1 ? 'resourceDetail.meta.file' : 'resourceDetail.meta.files', { count: fileCount }),
      t(lineCount === 1 ? 'resourceDetail.meta.line' : 'resourceDetail.meta.lines', { count: compact.format(lineCount) }),
      t('resourceDetail.meta.words', { count: compact.format(words(body)) }),
    ].join(' · ')]);
  }
  rows.push([t('resourceDetail.meta.context'), t('resourceDetail.meta.contextValue', { always: compact.format(always), onDemand: compact.format(onDemand) })]);
  rows.push([
    t('resourceDetail.tabs.audit'),
    auditPending ? <span className="ss-st off">{t('resourceDetail.security.scanning')}</span> : audit ? (
      <Link to={auditHref} className={`ss-st hover:underline ${findings === 0 ? 'ok' : audit.isBlocked ? 'bad' : 'warn'}`}>
        {findings === 0 ? t('resourceDetail.audit.noFindings') : t(findings === 1 ? 'resourceDetail.audit.finding' : 'resourceDetail.audit.findings', { count: findings })}
      </Link>
    ) : <span className="ss-st off">—</span>,
  ]);

  return (
    <div className="ss-box">
      <dl className="ss-kv">
        {rows.map(([label, value]) => (
          <div key={label} className="contents">
            <dt>{label}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function TargetsSection({ resource }: { resource: Skill }) {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const isAgent = resource.kind === 'agent';
  const targetsQuery = useQuery({ queryKey: queryKeys.targets.all, queryFn: () => api.listTargets(), staleTime: staleTimes.targets });
  const { getSkillTargets, isLoading } = useSyncMatrix();
  const diffQuery = useQuery({ queryKey: queryKeys.diff(), queryFn: () => api.diff(), staleTime: staleTimes.diff, enabled: !isAgent });
  const [pending, setPending] = useState<string | null>(null);

  const targets = [...(targetsQuery.data?.targets ?? [])].sort((a, b) => a.name.localeCompare(b.name));
  if (targets.length === 0 || isLoading) return null;

  const entries = new Map(
    getSkillTargets(resource.flatName).filter((e) => (e.kind ?? 'skill') === resource.kind).map((e) => [e.target, e]),
  );
  const rows = targets.map((target) => {
    const entry = entries.get(target.name);
    const action = diffQuery.data?.diffs.find((d) => d.target === target.name)?.items
      .find((i) => i.skill === resource.flatName && (i.kind ?? 'skill') === 'skill')?.action;
    return { target, entry, action, on: entry?.status === 'synced' || entry?.status === 'na' };
  });
  const supported = rows.filter((r) => r.entry);

  const toggle = async (row: (typeof rows)[number]) => {
    const patch = row.entry && targetFilterPatch(row.entry, row.target, resource.kind, resource.flatName);
    if (!patch) return;
    setPending(row.target.name);
    try {
      await api.updateTarget(row.target.name, patch);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.targets.all }),
        queryClient.invalidateQueries({ queryKey: ['sync-matrix'] }),
        queryClient.invalidateQueries({ queryKey: ['diff'] }),
      ]);
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setPending(null);
    }
  };

  const label = (row: (typeof rows)[number]) => {
    const { entry, action } = row;
    switch (entry?.status) {
      case 'synced':
        if (action === 'skip') return <span className="text-bad">{t('resourceDetail.targets.conflict')}</span>;
        if (action === 'link' || action === 'update') return <span className="text-warn">{t('resourceDetail.targets.notSynced')}</span>;
        return !isAgent && diffQuery.data ? t('resourceDetail.targets.linked') : null;
      case 'excluded':
        return t('resourceDetail.targets.excludedBy', { pattern: entry.reason });
      case 'not_included':
        return t('resourceDetail.targets.notIncluded');
      case 'skill_target_mismatch':
        return t('resourceDetail.targets.mismatch');
      case 'na':
        return t('resourceDetail.targets.symlink');
      default:
        return null;
    }
  };

  return (
    <section>
      <div className="ss-sec">
        <h2>{t('resourceDetail.targets.title')}</h2>
        <span className="ss-cnt">{t('resourceDetail.targets.count', { on: supported.filter((r) => r.on).length, total: supported.length })}</span>
      </div>
      <div className="ss-list">
        {rows.map((row) => {
          if (!row.entry) {
            return (
              <div key={row.target.name} className="ss-r !min-h-11 opacity-55">
                <span className="ss-at"><AgentIcon target={row.target.name} /></span>
                <span className="min-w-0 flex-1 truncate">{row.target.name}</span>
                <span className="text-xs text-ink-3">{t('resourceDetail.targets.noAgentSupport')}</span>
              </div>
            );
          }
          const patch = targetFilterPatch(row.entry, row.target, resource.kind, resource.flatName);
          const text = label(row);
          return (
            <div key={row.target.name} className="ss-r !min-h-[50px]">
              <span className="ss-at"><AgentIcon target={row.target.name} /></span>
              <div className="flex min-w-0 flex-1 flex-col gap-px">
                <span className="truncate font-semibold">{row.target.name}</span>
                {text && <span className="text-xs text-ink-3">{text}</span>}
              </div>
              <button
                type="button"
                role="switch"
                aria-checked={row.on}
                aria-label={row.target.name}
                disabled={!patch || pending !== null}
                className={`ss-sw ${row.on ? 'on' : ''} disabled:cursor-not-allowed disabled:opacity-50`}
                onClick={() => toggle(row)}
              >
                <i />
              </button>
            </div>
          );
        })}
      </div>
      <p className="mt-2.5 text-[13px] text-ink-3">{t(isAgent ? 'resourceDetail.targets.footnoteAgent' : 'resourceDetail.targets.footnote')}</p>
    </section>
  );
}

/* -- Files ---------------------------------------- */

function FilesTab({ resource, files, skillMd, tabSearch, components, raw, onRaw }: {
  resource: Skill;
  files: string[];
  skillMd: string;
  tabSearch: (tab: Tab, extra?: Record<string, string | number>) => string;
  components: Components;
  raw: boolean;
  onRaw: (raw: boolean) => void;
}) {
  const t = useT();
  const { toast } = useToast();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const sorted = useMemo(() => {
    const rest = files.filter((f) => f !== 'SKILL.md').sort((a, b) => a.localeCompare(b));
    return ['SKILL.md', ...rest];
  }, [files]);
  const requested = searchParams.get('file');
  const selected = requested && sorted.includes(requested) ? requested : 'SKILL.md';
  const line = Number(searchParams.get('line')) || 0;

  const fileQuery = useQuery({
    queryKey: ['skill-file', resource.flatName, selected],
    queryFn: () => api.getSkillFile(resource.flatName, selected),
    enabled: selected !== 'SKILL.md',
  });
  const content = selected === 'SKILL.md' ? skillMd : fileQuery.data?.content;
  const markdown = isMarkdown(selected);
  // A line link (from an audit finding) only makes sense in the raw view
  const showRaw = !markdown || raw || line > 0;
  const setView = (next: boolean) => {
    onRaw(next);
    if (!next && line > 0) navigate(tabSearch('files', { file: selected }), { replace: true });
  };

  const tree = fileTree(sorted);

  const copyPath = () => {
    void navigator.clipboard?.writeText(`${resource.sourcePath}/${selected}`);
    toast(t('skillEditor.toast.pathCopied'), 'info');
  };

  return (
    <div className="grid grid-cols-[240px_minmax(0,1fr)] items-start gap-6">
      <div className="flex flex-col gap-0.5">
        {tree.map((row) => {
          const Icon = row.folder ? Folder : row.label.endsWith('.md') ? FileText : CODE_EXT.test(row.label) ? FileCode2 : File;
          const inner = (
            <>
              <Icon size={15} className="shrink-0" />
              <span className="truncate font-mono text-[13px]">{row.label}</span>
            </>
          );
          const style = { paddingLeft: 10 + row.depth * 20 };
          return row.folder
            ? <div key={row.path} className="ss-nv" style={style}>{inner}</div>
            : <Link key={row.path} to={tabSearch('files', { file: row.path })} replace className={`ss-nv ${row.path === selected ? 'on' : ''}`} style={style}>{inner}</Link>;
        })}
        <p className="ml-2.5 mt-3 text-xs text-ink-3">
          {t(sorted.length === 1 ? 'resourceDetail.meta.file' : 'resourceDetail.meta.files', { count: sorted.length })}
        </p>
      </div>
      {/* The viewer stays in view while a long file list scrolls the page. The offsets keep it clear of the
          fixed account avatar (top right) and the scroll-to-top button (bottom right) */}
      <div className="sticky top-20 flex h-[calc(100vh-160px)] min-h-[360px] min-w-0 flex-col gap-2.5">
        <div className="flex min-h-8 items-center justify-between gap-3">
          <span className="truncate font-mono text-[13px] font-semibold">{selected}</span>
          <div className="flex shrink-0 items-center gap-2">
            {markdown && content !== undefined && <ViewToggle raw={showRaw} onChange={setView} />}
            <Button variant="ghost" size="sm" onClick={copyPath}>
              <Copy size={14} />
              {t('skillEditor.copyPath')}
            </Button>
          </div>
        </div>
        {fileQuery.error ? (
          <div className="ss-note bad"><TriangleAlert size={16} /><div className="flex-1">{fileQuery.error.message}</div></div>
        ) : content === undefined ? (
          <div className="ss-code grid min-h-0 flex-1 place-items-center"><Spinner /></div>
        ) : content.includes('\u0000') ? (
          <div className="ss-note"><File size={16} /><div className="flex-1">{t('resourceDetail.files.binary')}</div></div>
        ) : showRaw ? (
          <CodeView content={content} lang={selected} line={line} className="min-h-0 flex-1" />
        ) : (
          <div className="ss-box min-h-0 flex-1 overflow-auto !shadow-none !px-7 !py-6">
            <MarkdownView size="lg" components={components}>{content}</MarkdownView>
          </div>
        )}
      </div>
    </div>
  );
}

/* -- Audit ---------------------------------------- */

function AuditTab({ query, files, tabSearch }: {
  query: UseQueryResult<{ result: AuditResult }>;
  files: string[];
  tabSearch: (tab: Tab, extra?: Record<string, string | number>) => string;
}) {
  const t = useT();
  const { locale } = useI18n();
  const { toast } = useToast();
  const [toggled, setToggled] = useState<Set<number>>(new Set());
  const [rescanning, setRescanning] = useState(false);

  if (query.isPending) return <div className="grid min-h-40 place-items-center"><Spinner /></div>;
  if (query.error) {
    return <div className="ss-note bad"><TriangleAlert size={16} /><div className="flex-1">{query.error.message}</div></div>;
  }

  const { findings, threshold, isBlocked } = query.data.result;
  const rescanNow = async () => {
    setRescanning(true);
    // A scan usually takes milliseconds and returns the same result, so hold the spinner and report the outcome
    const [res] = await Promise.all([query.refetch(), new Promise((r) => window.setTimeout(r, 600))]);
    setRescanning(false);
    if (res.error) {
      toast(res.error.message, 'error');
      return;
    }
    const count = res.data?.result.findings.length ?? 0;
    const key = count === 0 ? 'rescanClean' : count === 1 ? 'rescanFinding' : 'rescanFindings';
    toast(t(`resourceDetail.audit.${key}`, { count }), count === 0 ? 'success' : 'warning');
  };
  const busy = rescanning || query.isFetching;
  const rescan = (
    <Button variant="secondary" size="sm" loading={busy} onClick={rescanNow}>
      {!busy && <RefreshCw size={14} />}
      {t('resourceDetail.audit.scanAgain')}
    </Button>
  );
  const scanned = t('resourceDetail.audit.scanned', { time: formatRelativeTime(query.dataUpdatedAt, locale), threshold });

  if (findings.length === 0) {
    return <EmptyState icon={ShieldCheck} title={t('resourceDetail.audit.noFindings')} description={scanned} action={rescan} />;
  }

  const sorted = [...findings].sort((a, b) => SEV_RANK[a.severity] - SEV_RANK[b.severity]);
  // At or above the block threshold starts open; toggling flips that default
  const defaultOpen = (i: number) => i === 0 || SEV_RANK[sorted[i].severity] <= (SEV_RANK[threshold.toUpperCase()] ?? -1);
  const flip = (i: number) => setToggled((prev) => {
    const next = new Set(prev);
    if (!next.delete(i)) next.add(i);
    return next;
  });

  return (
    <>
      {isBlocked && (
        <div className="ss-note bad">
          <ShieldAlert size={16} />
          <div className="flex-1">{t('resourceDetail.audit.blocked', { threshold })}</div>
          {rescan}
        </div>
      )}
      <div className="ss-list">
        {sorted.map((f, i) => {
          const open = defaultOpen(i) !== toggled.has(i);
          const where = [f.file && (f.line ? `${f.file}:${f.line}` : f.file), f.ruleId && t('resourceDetail.audit.rule', { id: f.ruleId })].filter(Boolean).join(' · ');
          return (
            <div key={i} className="ss-r !items-start !py-3.5">
              <button type="button" aria-expanded={open} className="flex min-w-0 flex-1 cursor-pointer items-start gap-3 text-left" onClick={() => flip(i)}>
                <span className="w-[74px] shrink-0 pt-px"><span className={`ss-sev ${SEV[f.severity]}`}>{f.severity}</span></span>
                <span className="flex min-w-0 flex-1 flex-col gap-2">
                  <span className="font-semibold">{f.message}</span>
                  {open && f.snippet && (
                    <span className="ss-code block !overflow-x-auto !py-2.5">
                      {f.line > 0 && <span className="ln">{f.line}</span>}
                      {f.snippet}
                    </span>
                  )}
                  {where && <span className="truncate font-mono text-xs text-ink-3">{where}</span>}
                </span>
                {!open && <ChevronDown size={16} className="mt-0.5 shrink-0 text-ink-3" />}
              </button>
              {open && files.includes(f.file) && (
                <Link to={tabSearch('files', { file: f.file, ...(f.line ? { line: f.line } : {}) })} className="ss-btn sm">
                  {t('resourceDetail.audit.openFile')}
                </Link>
              )}
            </div>
          );
        })}
      </div>
      <div className="flex items-center gap-3">
        <p className="flex-1 text-[13px] text-ink-3">{scanned}</p>
        {!isBlocked && rescan}
      </div>
    </>
  );
}

/* -- Dialogs -------------------------------------- */

function BlockedDialog({ name, message, loading, onSkip, onClose }: {
  name: string;
  message: string;
  loading: boolean;
  onSkip: () => void;
  onClose: () => void;
}) {
  const t = useT();
  const findings = parseFindings(message.split('\n'), false);
  const threshold = thresholdOf(message);
  const title = t('resourceDetail.blocked.title');
  return (
    <DialogShell open onClose={onClose} padding="none" className="!max-w-[560px]" ariaLabel={title} preventClose={loading}>
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="font-mono text-[13px] text-ink-2">{name}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={loading}><X size={16} /></button>
      </div>
      <div className="db">
        <div className="ss-note bad">
          <TriangleAlert size={16} />
          <div className="flex-1">
            <b>{t('resourceDetail.blocked.notApplied')}</b>{' '}
            {threshold ? t('resourceDetail.blocked.reason', { threshold }) : t('resourceDetail.blocked.reasonNoThreshold')}
          </div>
        </div>
        {findings.length > 0 && <FindingList findings={findings} header={t('resourceDetail.blocked.findings')} showName={false} />}
        <p className="text-[13px] text-ink-2">{t('resourceDetail.blocked.skipPrompt')}</p>
      </div>
      <div className="df">
        <Button variant="danger" loading={loading} onClick={onSkip}>{t('resourceDetail.blocked.skipAuditAndUpdate')}</Button>
        <span className="flex-1" />
        <Button variant="primary" onClick={onClose} disabled={loading}>{t('resourceDetail.blocked.keep')}</Button>
      </div>
    </DialogShell>
  );
}
