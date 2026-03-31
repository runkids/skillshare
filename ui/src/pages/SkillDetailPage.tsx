import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  ArrowLeft, Trash2, ExternalLink, FileText, ArrowUpRight, RefreshCw, Target,
  Type, AlignLeft, Files, Scale, Zap,
  FileCode2, Braces, Settings, BookOpen, File, FolderOpen,
  ShieldCheck, Link2, EyeOff, Eye,
} from 'lucide-react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Badge from '../components/Badge';
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

const FileViewerModal = lazy(() => import('../components/FileViewerModal'));

type SkillManifest = {
  name?: string;
  description?: string;
  license?: string;
};

function parseScalarValue(raw: string): string | undefined {
  const trimmed = raw.trim();
  if (!trimmed) return undefined;
  // YAML block scalar indicators — fall through to block reader
  if (/^[>|][+-]?$/.test(trimmed)) return undefined;
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1).trim() || undefined;
  }
  return trimmed;
}

function extractManifestValue(frontmatter: string, key: 'name' | 'description' | 'license'): string | undefined {
  const lines = frontmatter.split(/\r?\n/);
  const keyPrefix = `${key}:`;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!line.startsWith(keyPrefix)) continue;

    const inline = parseScalarValue(line.slice(keyPrefix.length));
    if (inline) return inline;

    const blockLines: string[] = [];
    for (let j = i + 1; j < lines.length; j++) {
      const candidate = lines[j];
      if (candidate.trim() === '') {
        blockLines.push('');
        continue;
      }
      if (!candidate.startsWith(' ') && !candidate.startsWith('\t')) break;
      blockLines.push(candidate.trim());
      i = j;
    }

    const block = blockLines.join(' ').replace(/\s+/g, ' ').trim();
    return block || undefined;
  }

  return undefined;
}

function parseSkillMarkdown(content: string): { manifest: SkillManifest; markdown: string } {
  if (!content) return { manifest: {}, markdown: '' };

  const match = content.match(/^---\s*\r?\n([\s\S]*?)\r?\n---\s*(?:\r?\n)?/);
  if (!match) return { manifest: {}, markdown: content };

  const frontmatter = match[1];
  const manifest: SkillManifest = {
    name: extractManifestValue(frontmatter, 'name'),
    description: extractManifestValue(frontmatter, 'description'),
    license: extractManifestValue(frontmatter, 'license'),
  };

  const markdown = content.slice(match[0].length);
  return { manifest, markdown };
}

function skillTypeLabel(type?: string): string | undefined {
  if (!type) return undefined;
  if (type === 'github-subdir') return 'github';
  return type;
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
function ContentStatsBar({ content, description, body, fileCount, license }: { content: string; description?: string; body?: string; fileCount: number; license?: string }) {
  const trimmed = content.trim();
  const wordCount = trimmed ? trimmed.split(/\s+/).length : 0;
  const lineCount = trimmed ? trimmed.split(/\r?\n/).length : 0;
  const descTokens = description ? Math.round(description.length / 4) : 0;
  const bodyTokens = body ? Math.round(body.trim().length / 4) : 0;
  const totalTokens = descTokens + bodyTokens || Math.round(trimmed.length / 4);

  return (
    <div className="ss-detail-stats flex items-center gap-4 flex-wrap text-sm text-pencil-light py-3 mb-4 border-b border-muted">
      <Tooltip content={`Description: ~${descTokens.toLocaleString()}\nBody: ~${bodyTokens.toLocaleString()}\nTotal: ~${totalTokens.toLocaleString()}\n(~4 chars/token estimate)`}>
        <span className="inline-flex items-center gap-1.5">
          <Zap size={12} strokeWidth={2.5} />
          ~{totalTokens.toLocaleString()} tokens
          {descTokens > 0 && <span className="text-pencil-light/60">(desc ~{descTokens.toLocaleString()} · body ~{bodyTokens.toLocaleString()})</span>}
        </span>
      </Tooltip>
      <span className="inline-flex items-center gap-1.5">
        <Type size={12} strokeWidth={2.5} />
        {wordCount.toLocaleString()} words
      </span>
      <span className="inline-flex items-center gap-1.5">
        <AlignLeft size={12} strokeWidth={2.5} />
        {lineCount.toLocaleString()} lines
      </span>
      <span className="inline-flex items-center gap-1.5">
        <Files size={12} strokeWidth={2.5} />
        {fileCount} file{fileCount !== 1 ? 's' : ''}
      </span>
      {license && (
        <span className="inline-flex items-center gap-1.5">
          <Scale size={12} strokeWidth={2.5} />
          {license}
        </span>
      )}
    </div>
  );
}

export default function SkillDetailPage() {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.skills.detail(name!),
    queryFn: () => api.getSkill(name!),
    staleTime: staleTimes.skills,
    enabled: !!name,
  });
  const allSkills = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const auditQuery = useQuery({
    queryKey: queryKeys.audit.skill(name!),
    queryFn: () => api.auditSkill(name!),
    staleTime: staleTimes.auditSkill,
    enabled: !!name,
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
  const { toast } = useToast();

  // Build lookup maps for skill cross-referencing
  const skillMaps = useMemo(() => {
    const skills = allSkills.data?.skills ?? [];
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
          Failed to load skill
        </p>
        <p className="text-pencil-light text-sm mt-1">{error.message}</p>
      </Card>
    );
  }
  if (!data) return null;

  const { skill, skillMdContent, files: rawFiles } = data;
  const files = rawFiles ?? [];
  const parsedDoc = parseSkillMarkdown(skillMdContent ?? '');
  const hasManifest = Boolean(parsedDoc.manifest.name || parsedDoc.manifest.description || parsedDoc.manifest.license);
  const renderedMarkdown = parsedDoc.markdown.trim() ? parsedDoc.markdown : skillMdContent;

  /** Try to resolve a reference to a known skill */
  function resolveSkillRef(ref: string): Skill | undefined {
    // Direct name match
    if (skillMaps.byName.has(ref)) return skillMaps.byName.get(ref);
    // Try as child: currentFlatName__ref (with / replaced by __)
    const childFlat = `${skill.flatName}__${ref.replace(/\//g, '__')}`;
    if (skillMaps.byFlat.has(childFlat)) return skillMaps.byFlat.get(childFlat);
    return undefined;
  }

  /** Try to resolve a file path to a known skill */
  function resolveFileSkill(filePath: string): Skill | undefined {
    // Skip non-directory files (files with extensions)
    if (/\.[a-z]+$/i.test(filePath) && !filePath.endsWith('.md')) return undefined;
    const flat = `${skill.flatName}__${filePath.replace(/\//g, '__')}`;
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
                to={`/skills/${encodeURIComponent(resolved.flatName)}`}
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
      if (skill.isInRepo) {
        const repoName = skill.relPath.split('/')[0];
        await api.deleteRepo(repoName);
        toast(`Repository "${repoName}" uninstalled.`, 'success');
      } else {
        await api.deleteSkill(skill.flatName);
        toast(`Skill "${skill.name}" uninstalled.`, 'success');
      }
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      await queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      navigate('/skills');
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
      const skillName = skill.isInRepo ? skill.relPath.split('/')[0] : skill.relPath;
      const res = await api.update({ name: skillName, skipAudit });
      const item = res.results[0];
      if (item?.action === 'updated') {
        const auditInfo = item.auditRiskLabel
          ? ` · Security: ${item.auditRiskLabel.toUpperCase()}${item.auditRiskScore ? ` (${item.auditRiskScore}/100)` : ''}`
          : '';
        toast(`Updated: ${item.name} — ${item.message}${auditInfo}`, 'success');
        await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name!) });
        await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
        await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      } else if (item?.action === 'up-to-date') {
        toast(`${item.name} is already up to date.`, 'info');
      } else if (item?.action === 'blocked') {
        setBlockedMessage(item.message ?? 'Blocked by security audit — HIGH/CRITICAL findings detected');
      } else if (item?.action === 'error') {
        toast(item.message ?? 'Update failed', 'error');
      } else {
        toast(item?.message ?? 'Skipped', 'warning');
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
      if (skill.disabled) {
        await api.enableSkill(skill.flatName);
        toast(`Enabled: ${skill.name}`, 'success');
      } else {
        await api.disableSkill(skill.flatName);
        toast(`Disabled: ${skill.name}`, 'success');
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

  return (
    <div className="animate-fade-in">
      {/* Header — sticky */}
      <div className="flex items-center gap-3 mb-2 sticky top-0 z-20 bg-paper py-3 -mx-4 px-4 md:-mx-8 md:px-8 -mt-3">
        <IconButton
          icon={<ArrowLeft size={18} strokeWidth={2.5} />}
          label="Back to skills"
          size="lg"
          variant="outline"
          onClick={() => navigate('/skills')}
          className="bg-surface"
          style={{ boxShadow: shadows.sm }}
        />
        <div className="flex items-center gap-3 flex-wrap">
          <h2
            className="ss-detail-title text-2xl md:text-3xl font-bold text-pencil"
          >
            {skill.name}
          </h2>
          {skill.disabled && <Badge variant="danger">disabled</Badge>}
          {skill.isInRepo && <Badge variant="warning">tracked repo</Badge>}
          {skillTypeLabel(skill.type) && <Badge variant="info">{skillTypeLabel(skill.type)}</Badge>}
          {skill.targets && skill.targets.length > 0 && (
            <span className="inline-flex items-center gap-1">
              <Target size={13} strokeWidth={2.5} className="text-pencil-light" />
              {skill.targets.map((t) => (
                <Badge key={t} variant="default">{t}</Badge>
              ))}
            </span>
          )}
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
                      <dt className="text-sm text-muted-dark uppercase tracking-wide">Name</dt>
                      <dd className="text-xl font-bold text-pencil">{parsedDoc.manifest.name}</dd>
                    </div>
                  )}
                  {parsedDoc.manifest.description && (
                    <div>
                      <dt className="text-sm text-muted-dark uppercase tracking-wide">Description</dt>
                      <dd className="text-base text-pencil">{parsedDoc.manifest.description}</dd>
                    </div>
                  )}
                  {parsedDoc.manifest.license && (
                    <div>
                      <dt className="text-sm text-muted-dark uppercase tracking-wide">License</dt>
                      <dd className="text-base text-pencil">{parsedDoc.manifest.license}</dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
            {/* Stage 1: Content Stats Bar */}
            <ContentStatsBar
              content={skillMdContent ?? ''}
              description={parsedDoc.manifest.description}
              body={parsedDoc.markdown}
              fileCount={files.length}
              license={parsedDoc.manifest.license}
            />
            <div className="prose-hand">
              {renderedMarkdown ? (
                <Markdown remarkPlugins={[remarkGfm]} components={mdComponents}>
                  {renderedMarkdown}
                </Markdown>
              ) : (
                <p className="text-pencil-light italic text-center py-8">
                  No SKILL.md content available.
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
              Metadata
            </h3>
            <dl className="space-y-2">
              <MetaItem label="Path" value={skill.relPath} mono copyable copyValue={skill.sourcePath} />
              {skill.source && <MetaItem label="Source" value={skill.source} mono />}
              {skill.version && <MetaItem label="Version" value={skill.version} mono />}
              {skill.branch && <MetaItem label="Branch" value={skill.branch} mono />}
              {skill.installedAt && (
                <MetaItem
                  label="Installed"
                  value={new Date(skill.installedAt).toLocaleDateString()}
                />
              )}
              {skill.targets && skill.targets.length > 0 && (
                <div className="flex items-baseline gap-3">
                  <dt className="text-xs text-pencil-light uppercase tracking-wider shrink-0 min-w-[4.5rem]">Targets</dt>
                  <dd className="flex flex-wrap gap-1.5 min-w-0">
                    {skill.targets.map((t) => (
                      <Badge key={t} variant="default">{t}</Badge>
                    ))}
                  </dd>
                </div>
              )}
              {skill.repoUrl && (
                <div className="flex items-baseline gap-3">
                  <dt className="text-xs text-pencil-light uppercase tracking-wider shrink-0 min-w-[4.5rem]">Repo</dt>
                  <dd className="min-w-0">
                    <a
                      href={skill.repoUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="link-subtle text-sm break-all"
                    >
                      <ExternalLink size={11} strokeWidth={2.5} className="inline -mt-0.5 mr-0.5" />
                      {skill.repoUrl.replace('https://', '').replace('.git', '')}
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
                  variant={skill.disabled ? 'primary' : 'secondary'}
                  size="sm"
                  className="flex-1"
                >
                  {toggling ? (
                    <Spinner size="sm" />
                  ) : skill.disabled ? (
                    <Eye size={14} strokeWidth={2.5} />
                  ) : (
                    <EyeOff size={14} strokeWidth={2.5} />
                  )}
                  {toggling
                    ? (skill.disabled ? 'Enabling...' : 'Disabling...')
                    : (skill.disabled ? 'Enable' : 'Disable')}
                </Button>
                {(skill.isInRepo || skill.source) && (
                  <Button
                    onClick={() => handleUpdate()}
                    disabled={updating}
                    variant="secondary"
                    size="sm"
                    className="flex-1"
                  >
                    {updating ? <Spinner size="sm" /> : <RefreshCw size={14} strokeWidth={2.5} />}
                    {updating ? 'Updating...' : 'Update'}
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
                  ? 'Uninstalling...'
                  : skill.isInRepo
                    ? 'Uninstall Repo'
                    : 'Uninstall'}
              </Button>
            </div>
          </Card>

          <Card className="ss-detail-pinned" overflow>
            <h3
              className="ss-detail-heading font-bold text-pencil mb-3 flex items-center gap-2"
            >
              <FileText size={16} strokeWidth={2.5} />
              Files ({files.length})
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
                          to={`/skills/${encodeURIComponent(linkedSkill.flatName)}`}
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
              <p className="text-sm text-muted-dark italic">No files.</p>
            )}
          </Card>

          {/* Security Audit */}
          <SecurityAuditCard auditQuery={auditQuery} />

          {/* Target Distribution */}
          <TargetDistribution flatName={skill.flatName} />

          {/* Target Sync Status */}
          <SyncStatusCard diffQuery={diffQuery} skillFlatName={skill.flatName} />
        </div>
      </div>

      {/* File viewer modal */}
      {viewingFile && (
        <Suspense fallback={null}>
          <FileViewerModal
            skillName={skill.flatName}
            filepath={viewingFile}
            sourcePath={skill.sourcePath}
            onClose={() => setViewingFile(null)}
          />
        </Suspense>
      )}

      {/* Blocked by security audit dialog */}
      <ConfirmDialog
        open={blockedMessage !== null}
        title="Blocked by Security Audit"
        message={
          <>
            <p className="text-danger text-sm mb-2">{blockedMessage}</p>
            <p className="text-pencil-light text-sm">Skip the audit and apply the update anyway?</p>
          </>
        }
        confirmText="Skip Audit & Update"
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
        title={skill.isInRepo ? 'Uninstall Repository' : 'Uninstall Skill'}
        message={
          skill.isInRepo
            ? `Remove repository "${skill.relPath.split('/')[0]}"? This will move all skills in the repo to trash.`
            : `Uninstall skill "${skill.name}"? It will be moved to trash and can be restored within 7 days.`
        }
        confirmText="Uninstall"
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
  if (auditQuery.isPending) {
    return (
      <Card variant="outlined">
        <div className="flex items-center gap-2 animate-pulse">
          <ShieldCheck size={16} strokeWidth={2.5} className="text-pencil-light" />
          <span className="text-sm text-pencil-light">
            Scanning security...
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
        Security
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
            No security issues detected
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
function TargetDistribution({ flatName }: { flatName: string }) {
  const { getSkillTargets } = useSyncMatrix();
  const entries = getSkillTargets(flatName);

  if (entries.length === 0) return null;

  return (
    <Card className="ss-detail-pinned ss-detail-pinned-blue ss-detail-outlined">
      <h3 className="ss-detail-heading font-bold text-pencil mb-3 flex items-center gap-2">
        <Target size={16} strokeWidth={2.5} />
        Target Distribution
      </h3>
      <div className="space-y-3">
        {entries.map(e => (
          <div key={e.target} className="text-sm border-b border-dashed border-pencil-light/30 pb-2 last:border-0 last:pb-0">
            <div className="flex items-center gap-2">
              <span className={`w-2 h-2 rounded-full shrink-0 ${
                e.status === 'synced' ? 'bg-success' :
                e.status === 'na' ? 'bg-muted' : 'bg-danger'
              }`} />
              <Link to={`/targets/${encodeURIComponent(e.target)}/filters`}
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
                {e.status === 'synced' && '\u2713 Synced'}
                {e.status === 'excluded' && `\u2717 Excluded (${e.reason})`}
                {e.status === 'not_included' && '\u2717 Not included'}
                {e.status === 'skill_target_mismatch' && `Targets: ${e.reason}`}
                {e.status === 'na' && '\u2014 Symlink mode'}
              </span>
            </div>
          </div>
        ))}
      </div>
      <p className="text-xs text-pencil-light mt-3">
        Filters only apply to merge/copy mode targets.{' '}
        <Link to="/targets" className="text-blue hover:underline">Manage targets &rarr;</Link>
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
    linked: 'linked',
    missing: 'not synced',
    conflict: 'conflict',
    excluded: 'excluded',
  };

  return (
    <Card variant="outlined" className="ss-detail-pinned ss-detail-pinned-cyan ss-detail-outlined">
      <h3
        className="ss-detail-heading font-bold text-pencil mb-3 flex items-center gap-2"
      >
        <Link2 size={16} strokeWidth={2.5} />
        Target Sync
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
