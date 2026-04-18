import { useParams, useNavigate, Link, useSearchParams } from 'react-router-dom';
import {
  ArrowLeft, Trash2, ExternalLink, FileText, ArrowUpRight, RefreshCw, Target,
  Type, AlignLeft, Files, Scale, Zap,
  FileCode2, Braces, Settings, BookOpen, File, FolderOpen,
  ShieldCheck, Link2, EyeOff, Eye, Pencil,
} from 'lucide-react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Badge from '../components/Badge';
import KindBadge from '../components/KindBadge';
import SourceBadge from '../components/SourceBadge';
import Card from '../components/Card';
import CopyButton from '../components/CopyButton';
import Button from '../components/Button';
import Tooltip from '../components/Tooltip';
import IconButton from '../components/IconButton';
import { SkillDetailSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import Spinner from '../components/Spinner';
import ConfirmDialog from '../components/ConfirmDialog';
import { api, type Skill } from '../api/client';
import { lazy, Suspense, useState, useMemo } from 'react';
import { radius, shadows } from '../design';
import { BlockStamp, RiskMeter } from '../components/audit';
import { severityBadgeVariant } from '../lib/severity';
import { useSyncMatrix } from '../hooks/useSyncMatrix';
import { clearAuditCache } from '../lib/auditCache';
import { formatSkillDisplayName, formatTrackedRepoName } from '../lib/resourceNames';
import { syncMatrixReasonText } from '../lib/syncMatrixText';
import { SkillEditor, Outline } from '../components/skill-editor';
import ScrollToTop from '../components/ScrollToTop';
import { parseSkillMarkdown } from '../lib/frontmatter';
import { useT } from '../i18n';

const FileViewerModal = lazy(() => import('../components/FileViewerModal'));

type SkillManifest = {
  name?: string;
  description?: string;
  license?: string;
};

function parseSkillDoc(content: string): { manifest: SkillManifest; markdown: string } {
  const { frontmatter, body } = parseSkillMarkdown(content ?? '');
  const pick = (k: 'name' | 'description' | 'license'): string | undefined => {
    const v = frontmatter[k];
    if (v == null) return undefined;
    const s = String(v).trim();
    return s || undefined;
  };
  return {
    manifest: { name: pick('name'), description: pick('description'), license: pick('license') },
    markdown: body,
  };
}

/** Returns a lucide icon component + color class for a filename */
function getFileIcon(filename: string): { icon: typeof File; className: string } {
  if (filename === 'SKILL.md') return { icon: FileText, className: 'text-blue' };
  if (/\.(ts|tsx|js|jsx|go|py|rs|rb|sh|bash)$/i.test(filename)) return { icon: FileCode2, className: 'text-pencil-light' };
  if (/\.json$/i.test(filename)) return { icon: Braces, className: 'text-pencil-light' };
  if (/\.(yaml|yml|toml)$/i.test(filename)) return { icon: Settings, className: 'text-pencil-light' };
  if (/\.md$/i.test(filename)) return { icon: BookOpen, className: 'text-pencil-light' };
  if (filename.endsWith('/')) return { icon: FolderOpen, className: 'text-warning' };
  return { icon: File, className: 'text-pencil-light' };
}

/** Content stats bar showing word count, line count, file count, license */
function ContentStatsBar({ content, description, body, fileCount, license, trailing }: { content: string; description?: string; body?: string; fileCount: number; license?: string; trailing?: React.ReactNode }) {
  const t = useT();
  const trimmed = content.trim();
  const wordCount = trimmed ? trimmed.split(/\s+/).length : 0;
  const lineCount = trimmed ? trimmed.split(/\r?\n/).length : 0;
  const descTokens = description ? Math.round(description.length / 4) : 0;
  const bodyTokens = body ? Math.round(body.trim().length / 4) : 0;
  const totalTokens = descTokens + bodyTokens || Math.round(trimmed.length / 4);

  return (
    <div className="ss-detail-stats flex items-center gap-4 flex-wrap text-sm text-pencil-light py-3 mb-4 border-b border-muted">
      <Tooltip content={t('resourceDetail.stats.tokensDesc', { desc: descTokens.toLocaleString(), body: bodyTokens.toLocaleString(), total: totalTokens.toLocaleString() })}>
        <span className="inline-flex items-center gap-1.5">
          <Zap size={12} strokeWidth={2.5} />
          {t('resourceDetail.stats.tokens', { count: totalTokens.toLocaleString() })}
          {descTokens > 0 && <span className="text-pencil-light/60">{t('resourceDetail.stats.tokensParts', { desc: descTokens.toLocaleString(), body: bodyTokens.toLocaleString() })}</span>}
        </span>
      </Tooltip>
      <span className="inline-flex items-center gap-1.5">
        <Type size={12} strokeWidth={2.5} />
        {t('resourceDetail.stats.words', { count: wordCount.toLocaleString() })}
      </span>
      <span className="inline-flex items-center gap-1.5">
        <AlignLeft size={12} strokeWidth={2.5} />
        {t('resourceDetail.stats.lines', { count: lineCount.toLocaleString() })}
      </span>
      <span className="inline-flex items-center gap-1.5">
        <Files size={12} strokeWidth={2.5} />
        {t('resourceDetail.stats.files', { count: fileCount })}
      </span>
      {license && (
        <span className="inline-flex items-center gap-1.5">
          <Scale size={12} strokeWidth={2.5} />
          {license}
        </span>
      )}
      {trailing && <span className="ml-auto">{trailing}</span>}
    </div>
  );
}

export default function SkillDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const requestedKind = searchParams.get('kind') === 'agent'
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
  const allSkills = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const allTargets = useQuery({
    queryKey: queryKeys.targets.all,
    queryFn: () => api.listTargets(),
    staleTime: staleTimes.targets,
  });
  const skillKind = data?.resource.kind;
  const auditQuery = useQuery({
    queryKey: [...queryKeys.audit.skill(name!), skillKind],
    queryFn: () => api.auditSkill(name!, skillKind),
    staleTime: staleTimes.auditSkill,
    enabled: !!name && !!skillKind,
  });
  const diffQuery = useQuery({
    queryKey: queryKeys.diff(),
    queryFn: () => api.diff(),
    staleTime: staleTimes.diff,
  });
  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [blockedMessage, setBlockedMessage] = useState<string | null>(null);
  const [viewingFile, setViewingFile] = useState<string | null>(null);
  const [editMode, setEditMode] = useState(false);
  const { toast } = useToast();
  const t = useT();

  // Build lookup maps for skill cross-referencing
  const skillMaps = useMemo(() => {
    const skills = allSkills.data?.resources ?? [];
    const byName = new Map<string, Skill>();
    const byFlat = new Map<string, Skill>();
    for (const s of skills) {
      byName.set(s.name, s);
      byFlat.set(s.flatName, s);
    }
    return { byName, byFlat };
  }, [allSkills.data]);

  if (isPending) return <SkillDetailSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">
          {t('resourceDetail.error.failedToLoad')}
        </p>
        <p className="text-pencil-light text-sm mt-1">{error.message}</p>
      </Card>
    );
  }
  if (!data) return null;

  const { resource, skillMdContent, files: rawFiles } = data;
  const files = rawFiles ?? [];
  const parsedDoc = parseSkillDoc(skillMdContent ?? '');
  const hasManifest = Boolean(parsedDoc.manifest.name || parsedDoc.manifest.description || parsedDoc.manifest.license);
  const renderedMarkdown = parsedDoc.markdown.trim() ? parsedDoc.markdown : skillMdContent;

  /** Try to resolve a reference to a known skill */
  function resolveSkillRef(ref: string): Skill | undefined {
    // Direct name match
    if (skillMaps.byName.has(ref)) return skillMaps.byName.get(ref);
    // Try as child: currentFlatName__ref (with / replaced by __)
    const childFlat = `${resource.flatName}__${ref.replace(/\//g, '__')}`;
    if (skillMaps.byFlat.has(childFlat)) return skillMaps.byFlat.get(childFlat);
    return undefined;
  }

  /** Try to resolve a file path to a known skill */
  function resolveFileSkill(filePath: string): Skill | undefined {
    // Skip non-directory files (files with extensions)
    if (/\.[a-z]+$/i.test(filePath) && !filePath.endsWith('.md')) return undefined;
    const flat = `${resource.flatName}__${filePath.replace(/\//g, '__')}`;
    return skillMaps.byFlat.get(flat);
  }

  // Custom Markdown link component: resolve skill references to internal links
  const mdComponents: Components = {
    a: ({ href, children, ...props }) => {
      if (href) {
        // Check if href is a skill reference (not a URL)
        if (!href.startsWith('http') && !href.startsWith('#')) {
          const resolved = resolveSkillRef(href);
          if (resolved) {
            return (
              <Link
                to={`/resources/${encodeURIComponent(resolved.flatName)}`}
                className="link-subtle inline-flex items-center gap-0.5"
              >
                {children}
                <ArrowUpRight size={12} strokeWidth={2.5} className="shrink-0" />
              </Link>
            );
          }
          // Check if href matches a file in this skill — open in modal
          const matchedFile = files.find((f) => f === href || f.endsWith('/' + href));
          if (matchedFile) {
            return (
              <Button
                variant="link"
                onClick={() => setViewingFile(matchedFile)}
                className="link-subtle inline-flex items-center gap-0.5"
                style={{ font: 'inherit' }}
              >
                {children}
              </Button>
            );
          }
        }
      }
      // Default: external link
      return (
        <a href={href} target="_blank" rel="noopener noreferrer" {...props}>
          {children}
        </a>
      );
    },
  };

  const handleDelete = async () => {
    setDeleting(true);
    try {
      if (resource.isInRepo) {
        const repoName = resource.relPath.split('/')[0];
        await api.deleteRepo(repoName);
        toast(t('resourceDetail.toast.repoUninstalled', { name: formatTrackedRepoName(repoName) }), 'success');
      } else {
        await api.deleteResource(resource.flatName, resource.kind);
        toast(t('resourceDetail.toast.resourceUninstalled', { kind: resource.kind === 'agent' ? 'Agent' : 'Skill', name: resource.name }), 'success');
      }
      clearAuditCache(queryClient);
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      await queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      navigate('/resources');
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
      setDeleting(false);
      setConfirmDelete(false);
    }
  };

  const handleUpdate = async (skipAudit = false) => {
    setUpdating(true);
    setBlockedMessage(null);
    try {
      const resourceName = resource.isInRepo
        ? resource.relPath.split('/')[0]
        : resource.kind === 'agent'
          ? resource.flatName
          : resource.relPath;
      const res = await api.update({ name: resourceName, kind: resource.kind, skipAudit });
      const item = res.results[0];
      if (item?.action === 'updated') {
        const auditInfo = item.auditRiskLabel
          ? ` · Security: ${item.auditRiskLabel.toUpperCase()}${item.auditRiskScore ? ` (${item.auditRiskScore}/100)` : ''}`
          : '';
        toast(t('resourceDetail.toast.updated', { name: formatSkillDisplayName(item.name), message: item.message ?? '', auditInfo }), 'success');
        clearAuditCache(queryClient);
        await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name!) });
        await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
        await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      } else if (item?.action === 'up-to-date') {
        toast(t('resourceDetail.toast.upToDate', { name: formatSkillDisplayName(item.name) }), 'info');
      } else if (item?.action === 'blocked') {
        setBlockedMessage(item.message ?? t('resourceDetail.toast.blockedDefault'));
      } else if (item?.action === 'error') {
        toast(item.message ?? t('resourceDetail.toast.updateFailed'), 'error');
      } else {
        toast(item?.message ?? t('resourceDetail.toast.skipped'), 'warning');
      }
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setUpdating(false);
    }
  };

  const handleToggleDisabled = async () => {
    setToggling(true);
    try {
      if (resource.disabled) {
        await api.enableResource(resource.flatName, resource.kind);
        toast(t('resourceDetail.toast.enabled', { name: resource.name }), 'success');
      } else {
        await api.disableResource(resource.flatName, resource.kind);
        toast(t('resourceDetail.toast.disabled', { name: resource.name }), 'success');
      }
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name!) });
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setToggling(false);
    }
  };

  const handleOpenInEditor = async () => {
    try {
      const resp = await api.openSkillInEditor(resource.flatName, { kind: resource.kind });
      toast(t('resourceDetail.toast.openedIn', { editor: resp.editor }), 'info');
    } catch (e) {
      toast((e as Error).message, 'error');
    }
  };

  if (editMode) {
    // Show all configured targets so the user can toggle each on/off.
    // Targets currently linked to this resource start as enabled.
    const linkedTargetNames = new Set(resource.targets ?? []);
    const configuredTargets = allTargets.data?.targets ?? [];
    const editorTargets = (configuredTargets.length > 0
      ? configuredTargets.map((t) => ({
          id: t.name,
          name: t.name,
          status: (linkedTargetNames.has(t.name) ? 'ok' : 'off') as 'ok' | 'off',
        }))
      : Array.from(linkedTargetNames).map((tname) => ({
          id: tname,
          name: tname,
          status: 'ok' as const,
        })));

    return (
      <div className="-mx-4 -my-3 md:-mx-8 md:-my-3 animate-fade-in">
        <ScrollToTop />
        <SkillEditor
          skillName={resource.flatName}
          displayName={resource.name}
          kind={resource.kind}
          path={resource.relPath}
          tracked={resource.isInRepo}
          initialContent={skillMdContent ?? ''}
          fileCount={files.length}
          derived={{
            path: resource.relPath,
            source: resource.source,
            version: resource.version,
            branch: resource.branch,
            license: parsedDoc.manifest.license,
          }}
          availableTargets={editorTargets}
          onBack={() => setEditMode(false)}
          onSaved={async (next) => {
            queryClient.setQueryData(
              [...queryKeys.skills.detail(name!), requestedKind],
              (prev: unknown) => {
                if (!prev || typeof prev !== 'object') return prev;
                return { ...(prev as object), skillMdContent: next };
              }
            );
            await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name!) });
            setEditMode(false);
          }}
        />
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <ScrollToTop />
      {/* Header — sticky */}
      <div className="flex items-center gap-3 mb-2 sticky top-0 z-20 bg-paper py-3 -mx-4 px-4 md:-mx-8 md:px-8 -mt-3">
        <IconButton
          icon={<ArrowLeft size={18} strokeWidth={2.5} />}
          label={t('resourceDetail.backToResources')}
          size="lg"
          variant="outline"
          onClick={() => navigate('/resources')}
          className="bg-surface"
          style={{ boxShadow: shadows.sm }}
        />
        <div className="flex items-center gap-3 flex-wrap">
          <h2
            className="ss-detail-title text-2xl md:text-3xl font-bold text-pencil"
          >
            {resource.name}
          </h2>
          <KindBadge kind={resource.kind} />
          {resource.disabled && <Badge variant="danger">Disabled</Badge>}
          <SourceBadge type={resource.type} isInRepo={resource.isInRepo} />
          {resource.targets && resource.targets.length > 0 && (
            <span className="inline-flex items-center gap-1">
              <Target size={13} strokeWidth={2.5} className="text-pencil-light" />
              {resource.targets.map((t) => (
                <Badge key={t} variant="default">{t}</Badge>
              ))}
            </span>
          )}
        </div>
        <div className="ml-auto flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={handleOpenInEditor}>
            <ExternalLink size={14} /> {t('resourceDetail.actions.openInEditor')}
          </Button>
          <Button variant="primary" size="sm" onClick={() => setEditMode(true)}>
            <Pencil size={14} /> {t('resourceDetail.actions.edit')}
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main content: SKILL.md */}
        <div className="lg:col-span-2">
          <Card>
            {hasManifest && (
              <div
                className="ss-detail-manifest mb-4 p-4 pt-5 border-2 border-dashed border-pencil-light/30"
                style={{ borderRadius: radius.sm }}
              >
                <dl className="space-y-2">
                  {parsedDoc.manifest.name && (
                    <div>
                      <dt className="text-sm text-muted-dark uppercase tracking-wide">{t('resourceDetail.manifest.name')}</dt>
                      <dd className="text-xl font-bold text-pencil">{parsedDoc.manifest.name}</dd>
                    </div>
                  )}
                  {parsedDoc.manifest.description && (
                    <div>
                      <dt className="text-sm text-muted-dark uppercase tracking-wide">{t('resourceDetail.manifest.description')}</dt>
                      <dd className="text-base text-pencil">{parsedDoc.manifest.description}</dd>
                    </div>
                  )}
                  {parsedDoc.manifest.license && (
                    <div>
                      <dt className="text-sm text-muted-dark uppercase tracking-wide">{t('resourceDetail.manifest.license')}</dt>
                      <dd className="text-base text-pencil">{parsedDoc.manifest.license}</dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
            <ContentStatsBar
              content={skillMdContent ?? ''}
              description={parsedDoc.manifest.description}
              body={parsedDoc.markdown}
              fileCount={files.length}
              license={parsedDoc.manifest.license}
              trailing={
                renderedMarkdown ? (
                  <Outline
                    markdown={renderedMarkdown}
                    onJump={(h) => {
                      const candidates = document.querySelectorAll<HTMLElement>(
                        '.prose-hand h1, .prose-hand h2, .prose-hand h3, .prose-hand h4, .prose-hand h5, .prose-hand h6'
                      );
                      const target = Array.from(candidates).find(
                        (el) => (el.textContent ?? '').trim() === h.text.trim()
                      );
                      target?.scrollIntoView({ behavior: 'smooth', block: 'start' });
                    }}
                  />
                ) : undefined
              }
            />
            <div className="prose-hand">
              {renderedMarkdown ? (
                <Markdown remarkPlugins={[remarkGfm]} components={mdComponents}>
                  {renderedMarkdown}
                </Markdown>
              ) : (
                <p className="text-pencil-light italic text-center py-8">
                  {t('resourceDetail.noContent')}
                </p>
              )}
            </div>
          </Card>
        </div>

        {/* Sidebar: metadata + files — sticky + independently scrollable */}
        <div className="space-y-5 lg:sticky lg:top-16 lg:self-start lg:max-h-[calc(100vh-5rem)] lg:overflow-y-auto lg:-mr-2 lg:pr-2">
          <Card className="ss-detail-pinned" overflow >
            <h3
              className="ss-detail-heading font-bold text-pencil mb-3"
            >
              {t('resourceDetail.metadata.title')}
            </h3>
            <dl className="space-y-2">
              <MetaItem label={t('resourceDetail.metadata.path')} value={resource.relPath} mono copyable copyValue={resource.sourcePath} />
              {resource.source && <MetaItem label={t('resourceDetail.metadata.source')} value={resource.source} mono />}
              {resource.version && <MetaItem label={t('resourceDetail.metadata.version')} value={resource.version} mono />}
              {resource.branch && <MetaItem label={t('resourceDetail.metadata.branch')} value={resource.branch} mono />}
              {resource.installedAt && (
                <MetaItem
                  label={t('resourceDetail.metadata.installed')}
                  value={new Date(resource.installedAt).toLocaleDateString()}
                />
              )}
              {resource.targets && resource.targets.length > 0 && (
                <div className="flex items-baseline gap-3">
                  <dt className="text-xs text-pencil-light uppercase tracking-wider shrink-0 min-w-[4.5rem]">{t('resourceDetail.metadata.targets')}</dt>
                  <dd className="min-w-0 flex flex-wrap gap-1.5">
                    {resource.targets.map((tgt) => (
                      <Badge key={tgt} variant="default">{tgt}</Badge>
                    ))}
                  </dd>
                </div>
              )}
              {resource.repoUrl && (
                <div className="flex items-baseline gap-3">
                  <dt className="text-xs text-pencil-light uppercase tracking-wider shrink-0 min-w-[4.5rem]">{t('resourceDetail.metadata.repo')}</dt>
                  <dd className="min-w-0">
                    <a
                      href={resource.repoUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="link-subtle text-sm break-all"
                    >
                      <ExternalLink size={11} strokeWidth={2.5} className="inline -mt-0.5 mr-0.5" />
                      {resource.repoUrl.replace('https://', '').replace('.git', '')}
                    </a>
                  </dd>
                </div>
              )}
            </dl>

            {/* Actions */}
            <div className="flex flex-col gap-2 mt-4 pt-4 border-t border-dashed border-pencil-light/30">
              <div className="flex gap-2">
                <Button
                  onClick={handleToggleDisabled}
                  disabled={toggling}
                  variant={resource.disabled ? 'primary' : 'secondary'}
                  size="sm"
                  className="flex-1"
                >
                  {toggling ? (
                    <Spinner size="sm" />
                  ) : resource.disabled ? (
                    <Eye size={14} strokeWidth={2.5} />
                  ) : (
                    <EyeOff size={14} strokeWidth={2.5} />
                  )}
                  {toggling
                    ? (resource.disabled ? t('resourceDetail.actions.enabling') : t('resourceDetail.actions.disabling'))
                    : (resource.disabled ? t('resourceDetail.actions.enable') : t('resourceDetail.actions.disable'))}
                </Button>
                {(resource.isInRepo || resource.source) && (
                  <Button
                    onClick={() => handleUpdate()}
                    disabled={updating}
                    variant="secondary"
                    size="sm"
                    className="flex-1"
                  >
                    {updating ? <Spinner size="sm" /> : <RefreshCw size={14} strokeWidth={2.5} />}
                    {updating ? t('resourceDetail.actions.updating') : t('resourceDetail.actions.update')}
                  </Button>
                )}
              </div>
              <Button
                onClick={() => setConfirmDelete(true)}
                disabled={deleting}
                variant="danger"
                size="sm"
              >
                <Trash2 size={12} strokeWidth={2.5} />
                {deleting
                  ? t('resourceDetail.actions.uninstalling')
                  : resource.isInRepo
                    ? t('resourceDetail.actions.uninstallRepo')
                    : t('resourceDetail.actions.uninstall')}
              </Button>
            </div>
          </Card>

          {resource.kind !== 'agent' && <Card className="ss-detail-pinned" overflow>
            <h3
              className="ss-detail-heading font-bold text-pencil mb-3 flex items-center gap-2"
            >
              <FileText size={16} strokeWidth={2.5} />
              {t('resourceDetail.files.title', { count: files.length })}
            </h3>
            {files.length > 0 ? (
              <ul className="space-y-1.5 max-h-80 overflow-y-auto">
                {files.map((f) => {
                  const linkedSkill = resolveFileSkill(f);
                  const isSkillMd = f === 'SKILL.md';
                  const { icon: FileIcon, className: iconClass } = getFileIcon(f);
                  return (
                    <li
                      key={f}
                      className="text-sm text-pencil-light truncate flex items-center gap-2"
                    >
                      <FileIcon size={14} strokeWidth={2} className={`shrink-0 ${iconClass}`} />
                      {linkedSkill ? (
                        <Link
                          to={`/resources/${encodeURIComponent(linkedSkill.flatName)}`}
                          className="font-mono link-subtle inline-flex items-center gap-1"
                          style={{ fontSize: '0.8125rem' }}
                          title={`View skill: ${linkedSkill.name}`}
                        >
                          {f}
                          <ArrowUpRight size={11} strokeWidth={2.5} className="shrink-0" />
                        </Link>
                      ) : isSkillMd ? (
                        <span
                          className="font-mono truncate"
                        >
                          {f}
                        </span>
                      ) : (
                        <Button
                          variant="link"
                          onClick={() => setViewingFile(f)}
                          className="font-mono link-subtle text-left truncate inline-flex items-center gap-1"
                          style={{ fontSize: '0.8125rem' }}
                          title={`View file: ${f}`}
                        >
                          {f}
                        </Button>
                      )}
                    </li>
                  );
                })}
              </ul>
            ) : (
              <p className="text-sm text-muted-dark italic">{t('resourceDetail.files.noFiles')}</p>
            )}
          </Card>}

          {/* Security Audit */}
          <SecurityAuditCard auditQuery={auditQuery} />

          {/* Target Distribution */}
          <TargetDistribution flatName={resource.flatName} kind={resource.kind} />

          {/* Target Sync Status */}
          <SyncStatusCard diffQuery={diffQuery} skillFlatName={resource.flatName} />
        </div>
      </div>

      {/* File viewer modal */}
      {viewingFile && (
        <Suspense fallback={null}>
          <FileViewerModal
            skillName={resource.flatName}
            filepath={viewingFile}
            sourcePath={resource.sourcePath}
            onClose={() => setViewingFile(null)}
          />
        </Suspense>
      )}

      {/* Blocked by security audit dialog */}
      <ConfirmDialog
        open={blockedMessage !== null}
        title={t('resourceDetail.blocked.title')}
        message={
          <>
            <p className="text-danger text-sm mb-2">{blockedMessage}</p>
            <p className="text-pencil-light text-sm">{t('resourceDetail.blocked.skipPrompt')}</p>
          </>
        }
        confirmText={t('resourceDetail.blocked.skipAuditAndUpdate')}
        variant="danger"
        loading={updating}
        onConfirm={() => {
          setBlockedMessage(null);
          handleUpdate(true);
        }}
        onCancel={() => setBlockedMessage(null)}
      />

      {/* Confirm uninstall dialog */}
      <ConfirmDialog
        open={confirmDelete}
        title={resource.isInRepo ? t('resourceDetail.confirm.titleRepo') : t('resourceDetail.confirm.titleResource', { kind: resource.kind === 'agent' ? 'Agent' : 'Skill' })}
        message={
          resource.isInRepo
            ? t('resourceDetail.confirm.repoMessage', { name: resource.relPath.split('/')[0] })
            : t('resourceDetail.confirm.resourceMessage', { kind: resource.kind === 'agent' ? 'agent' : 'skill', name: resource.name })
        }
        confirmText={t('resourceDetail.actions.uninstall')}
        variant="danger"
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
      />
    </div>
  );
}

function MetaItem({
  label,
  value,
  mono,
  copyable,
  copyValue,
}: {
  label: string;
  value: string;
  mono?: boolean;
  copyable?: boolean;
  copyValue?: string;
}) {
  return (
    <div className="flex items-baseline gap-3">
      <dt className="text-xs text-pencil-light uppercase tracking-wider shrink-0 min-w-[4.5rem]">
        {label}
      </dt>
      <dd
        className={`text-sm text-pencil min-w-0 break-all${mono ? ' font-mono' : ''}`}
      >
        {value}
        {copyable && (
          <CopyButton
            value={copyValue ?? value}
            className="ml-1 align-middle"
          />
        )}
      </dd>
    </div>
  );
}

/** Security Audit sidebar card */
function SecurityAuditCard({
  auditQuery,
}: {
  auditQuery: ReturnType<typeof useQuery<Awaited<ReturnType<typeof api.auditSkill>>>>;
}) {
  const t = useT();
  if (auditQuery.isPending) {
    return (
      <Card variant="outlined">
        <div className="flex items-center gap-2 animate-pulse">
          <ShieldCheck size={16} strokeWidth={2.5} className="text-pencil-light" />
          <span className="text-sm text-pencil-light">
            {t('resourceDetail.security.scanning')}
          </span>
        </div>
      </Card>
    );
  }

  if (auditQuery.error || !auditQuery.data) return null;

  const { result } = auditQuery.data;
  const findingCounts = result.findings.reduce(
    (acc, f) => {
      acc[f.severity] = (acc[f.severity] || 0) + 1;
      return acc;
    },
    {} as Record<string, number>,
  );

  return (
    <Card variant="outlined" className="ss-detail-pinned ss-detail-pinned-green ss-detail-outlined">
      <h3
        className="ss-detail-heading font-bold text-pencil mb-3 flex items-center gap-2"
      >
        <ShieldCheck size={16} strokeWidth={2.5} />
        {t('resourceDetail.security.title')}
      </h3>
      <div className="space-y-3">
        <div className="flex items-stretch gap-2 flex-wrap">
          <BlockStamp isBlocked={result.isBlocked} />
          <RiskMeter riskLabel={result.riskLabel} riskScore={result.riskScore} />
        </div>
        {result.findings.length > 0 && (
          <div className="flex flex-wrap gap-1.5 pt-2" style={{ borderTop: '1px dashed rgba(139,132,120,0.3)' }}>
            {Object.entries(findingCounts)
              .sort(([a], [b]) => sevOrder(a) - sevOrder(b))
              .map(([sev, count]) => (
                <Badge key={sev} variant={severityBadgeVariant(sev)}>
                  {count} {sev}
                </Badge>
              ))}
          </div>
        )}
        {result.findings.length === 0 && (
          <p className="text-sm text-success">
            {t('resourceDetail.security.noIssues')}
          </p>
        )}
      </div>
    </Card>
  );
}

function sevOrder(sev: string): number {
  switch (sev) {
    case 'CRITICAL': return 0;
    case 'HIGH': return 1;
    case 'MEDIUM': return 2;
    case 'LOW': return 3;
    case 'INFO': return 4;
    default: return 5;
  }
}

/** Target Distribution sidebar card */
function TargetDistribution({ flatName, kind }: { flatName: string; kind: 'skill' | 'agent' }) {
  const { getSkillTargets } = useSyncMatrix();
  const t = useT();
  const entries = getSkillTargets(flatName);

  if (entries.length === 0) return null;

  return (
    <Card className="ss-detail-pinned ss-detail-pinned-blue ss-detail-outlined">
      <h3 className="ss-detail-heading font-bold text-pencil mb-3 flex items-center gap-2">
        <Target size={16} strokeWidth={2.5} />
        {t('resourceDetail.targetDistribution.title')}
      </h3>
      <div className="space-y-3">
        {entries.map(e => (
          <div key={e.target} className="text-sm border-b border-dashed border-pencil-light/30 pb-2 last:border-0 last:pb-0">
            <div className="flex items-center gap-2">
              <span className={`w-2 h-2 rounded-full shrink-0 ${
                e.status === 'synced' ? 'bg-success' :
                e.status === 'na' ? 'bg-muted' : 'bg-danger'
              }`} />
              <Link to={`/targets/${encodeURIComponent(e.target)}/filters?kind=${kind}`}
                    className="font-bold text-pencil hover:text-blue truncate">
                {e.target}
              </Link>
            </div>
            <div className="flex items-center justify-between mt-1 pl-4">
              <span className={`text-xs ${
                e.status === 'synced' ? 'text-success' :
                e.status === 'skill_target_mismatch' ? 'text-purple-600' :
                e.status === 'na' ? 'text-muted-dark' : 'text-danger'
              }`}>
                {e.status === 'synced' && `\u2713 ${syncMatrixReasonText(e, t)}`}
                {e.status === 'excluded' && `\u2717 ${syncMatrixReasonText(e, t)}`}
                {e.status === 'not_included' && `\u2717 ${syncMatrixReasonText(e, t)}`}
                {e.status === 'skill_target_mismatch' && syncMatrixReasonText(e, t)}
                {e.status === 'na' && `\u2014 ${syncMatrixReasonText(e, t)}`}
              </span>
            </div>
          </div>
        ))}
      </div>
      <p className="text-xs text-pencil-light mt-3">
        {t('resourceDetail.targetDistribution.filterNote')}{' '}
        <Link to="/targets" className="text-blue hover:underline">{t('resourceDetail.targetDistribution.manageTargets')}</Link>
      </p>
    </Card>
  );
}

/** Sync Status sidebar card */
function SyncStatusCard({
  diffQuery,
  skillFlatName,
}: {
  diffQuery: ReturnType<typeof useQuery<Awaited<ReturnType<typeof api.diff>>>>;
  skillFlatName: string;
}) {
  const t = useT();
  if (diffQuery.isPending || !diffQuery.data) return null;

  // Find which targets have this skill and their status
  const targetStatuses: { name: string; status: 'linked' | 'missing' | 'excluded' | 'conflict' }[] = [];

  for (const dt of diffQuery.data.diffs) {
    const item = dt.items.find((i) => i.skill === skillFlatName);
    if (item) {
      const status = item.action === 'ok' || item.action === 'linked'
        ? 'linked'
        : item.action === 'excluded'
          ? 'excluded'
          : item.action === 'conflict' || item.action === 'broken'
            ? 'conflict'
            : 'missing';
      targetStatuses.push({ name: dt.target, status });
    } else {
      // Skill not in diff for this target — check if it's because it's already synced (no diff entry = linked)
      targetStatuses.push({ name: dt.target, status: 'linked' });
    }
  }

  if (targetStatuses.length === 0) return null;

  const statusDot: Record<string, string> = {
    linked: 'bg-success',
    missing: 'bg-warning',
    conflict: 'bg-danger',
    excluded: 'bg-muted-dark',
  };

  const statusLabel: Record<string, string> = {
    linked: t('resourceDetail.syncStatus.linked'),
    missing: t('resourceDetail.syncStatus.notSynced'),
    conflict: t('resourceDetail.syncStatus.conflict'),
    excluded: t('resourceDetail.syncStatus.excluded'),
  };

  return (
    <Card variant="outlined" className="ss-detail-pinned ss-detail-pinned-cyan ss-detail-outlined">
      <h3
        className="ss-detail-heading font-bold text-pencil mb-3 flex items-center gap-2"
      >
        <Link2 size={16} strokeWidth={2.5} />
        {t('resourceDetail.syncStatus.title')}
      </h3>
      <ul className="space-y-1.5">
        {targetStatuses.map((t) => (
          <li key={t.name} className="flex items-center gap-2 text-sm">
            <span className={`w-2 h-2 rounded-full shrink-0 ${statusDot[t.status]}`} />
            <span className="font-mono text-pencil font-medium" style={{ fontSize: '0.8125rem' }}>
              {t.name}
            </span>
            <span className="text-pencil-light text-xs">{statusLabel[t.status]}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}
