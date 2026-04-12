import { useParams, useNavigate, Link, useBlocker, useBeforeUnload } from 'react-router-dom';
import {
  ArrowLeft, Trash2, ExternalLink, FileText, ArrowUpRight, RefreshCw, Target,
  Type, AlignLeft, Files, Scale,
  FileCode2, Braces, Settings, BookOpen, File, FolderOpen, Zap,
  ShieldCheck, Link2, EyeOff, Eye, ChevronDown, ChevronUp,
} from 'lucide-react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Badge from '../components/Badge';
import Card from '../components/Card';
import CopyButton from '../components/CopyButton';
import Button from '../components/Button';
import IconButton from '../components/IconButton';
import SegmentedControl from '../components/SegmentedControl';
import SkillFrontmatterGuide from '../components/SkillFrontmatterGuide';
import { createSkillMarkdownComponents } from '../components/SkillMarkdownComponents';
import { SkillDetailSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import Spinner from '../components/Spinner';
import ConfirmDialog from '../components/ConfirmDialog';
import Tooltip from '../components/Tooltip';
import { api, type Skill, type SkillStats } from '../api/client';
import type { SkillMarkdownEditorSurface } from '../components/SkillMarkdownEditor';
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react';
import { radius, shadows } from '../design';
import { BlockStamp, RiskMeter } from '../components/audit';
import { severityBadgeVariant } from '../lib/severity';
import { useSyncMatrix } from '../hooks/useSyncMatrix';
import { buildSkillDraftStats, buildSkillTokenBreakdown, parseSkillMarkdown } from '../lib/skillMarkdown';
import {
  formatFrontmatterValue,
  getAdditionalFrontmatterEntries,
  getReferenceFrontmatterEntries,
} from '../lib/skillFrontmatter';

const FileViewerModal = lazy(() => import('../components/FileViewerModal'));
const SkillMarkdownEditor = lazy(() => import('../components/SkillMarkdownEditor'));

type DetailMode = 'read' | 'edit' | 'split';

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

/** Content stats bar showing server-provided content counts, file count, license */
function ContentStatsBar({
  stats,
  fileCount,
  license,
  tokenBreakdown,
}: {
  stats: SkillStats;
  fileCount: number;
  license?: string;
  tokenBreakdown: { loadTokens: number; previewTokens: number };
}) {
  return (
    <div className="ss-detail-stats flex items-center gap-4 flex-wrap text-sm text-pencil-light py-3 mb-4 border-b border-dashed border-pencil-light/30">
      <span className="inline-flex items-center gap-1.5">
        <Type size={12} strokeWidth={2.5} />
        {stats.wordCount.toLocaleString()} words
      </span>
      <span className="inline-flex items-center gap-1.5">
        <AlignLeft size={12} strokeWidth={2.5} />
        {stats.lineCount.toLocaleString()} lines
      </span>
      <span className="inline-flex items-center gap-1.5">
        <Files size={12} strokeWidth={2.5} />
        {fileCount} file{fileCount !== 1 ? 's' : ''}
      </span>
      <Tooltip
        content={(
          <div className="space-y-1 text-left">
            <div>Loading the skill: {tokenBreakdown.loadTokens.toLocaleString()} tokens</div>
            <div>Reading the preview: {tokenBreakdown.previewTokens.toLocaleString()} tokens</div>
          </div>
        )}
      >
        <span className="inline-flex items-center gap-1.5">
          <Zap size={12} strokeWidth={2.5} />
          {stats.tokenCount.toLocaleString()} tokens
        </span>
      </Tooltip>
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
  const [savedContent, setSavedContent] = useState('');
  const [draftContent, setDraftContent] = useState('');
  const [mode, setMode] = useState<DetailMode>('read');
  const [editorSurface, setEditorSurface] = useState<SkillMarkdownEditorSurface>('rich');
  const [saveError, setSaveError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [openingFilePath, setOpeningFilePath] = useState<string | null>(null);
  const [showFrontmatterGuide, setShowFrontmatterGuide] = useState(false);
  const lastLoadedSkillRef = useRef<string | null>(null);
  const contentAuthorityRef = useRef<'query' | 'save-response'>('query');
  const { toast } = useToast();
  const skill = data?.skill;
  const skillMdContent = data?.skillMdContent ?? '';
  const files = data?.files ?? [];
  const isDirty = draftContent !== savedContent;
  const blocker = useBlocker(isDirty);
  const previewSource = draftContent;
  const parsedDoc = useMemo(() => parseSkillMarkdown(previewSource), [previewSource]);
  const savedParsedDoc = useMemo(() => parseSkillMarkdown(skillMdContent), [skillMdContent]);
  const referenceFrontmatterEntries = useMemo(
    () => getReferenceFrontmatterEntries(parsedDoc.frontmatter),
    [parsedDoc.frontmatter],
  );
  const additionalFrontmatterEntries = useMemo(
    () => getAdditionalFrontmatterEntries(parsedDoc.frontmatter),
    [parsedDoc.frontmatter],
  );
  const configuredFrontmatterEntries = useMemo(
    () => referenceFrontmatterEntries.filter((entry) => entry.isSet),
    [referenceFrontmatterEntries],
  );
  const compactFrontmatterEntries = useMemo(
    () => configuredFrontmatterEntries.filter((entry) => entry.key !== 'name' && entry.key !== 'description'),
    [configuredFrontmatterEntries],
  );
  const savedHasConfiguredFrontmatter = useMemo(() => {
    const savedReferenceEntries = getReferenceFrontmatterEntries(savedParsedDoc.frontmatter);
    const savedAdditionalEntries = getAdditionalFrontmatterEntries(savedParsedDoc.frontmatter);
    return savedReferenceEntries.some((entry) => entry.isSet) || savedAdditionalEntries.length > 0;
  }, [savedParsedDoc.frontmatter]);
  const displayedStats = useMemo(
    () => (isDirty ? buildSkillDraftStats(draftContent) : data?.stats ?? buildSkillDraftStats(draftContent)),
    [data?.stats, draftContent, isDirty],
  );
  const tokenBreakdown = useMemo(() => {
    const breakdown = buildSkillTokenBreakdown(draftContent);
    return breakdown;
  }, [draftContent]);
  const hasConfiguredFrontmatter = configuredFrontmatterEntries.length > 0 || additionalFrontmatterEntries.length > 0;
  const renderedMarkdown = parsedDoc.markdown.trim() ? parsedDoc.markdown : previewSource;

  const handleSave = async (nextContent = draftContent) => {
    if (!skill || !name || isSaving) return;

    setIsSaving(true);
    setSaveError(null);
    try {
      const response = await api.saveSkillFile(skill.flatName, 'SKILL.md', nextContent);
      contentAuthorityRef.current = 'save-response';
      setSavedContent(response.content);
      setDraftContent(response.content);
      toast('SKILL.md saved.', 'success');
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name) });
    } catch (e: unknown) {
      const message = (e as Error).message;
      setSaveError(message);
      toast(message, 'error');
    } finally {
      setIsSaving(false);
    }
  };

  const handleDiscardDraft = () => {
    setDraftContent(savedContent);
    setSaveError(null);
  };

  const handleOpenLocalFile = async (filepath: string) => {
    if (!skill || openingFilePath) return;

    setOpeningFilePath(filepath);
    try {
      await api.openSkillFile(skill.flatName, filepath);
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setOpeningFilePath(null);
    }
  };

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
  const mdComponents = useMemo(() => createSkillMarkdownComponents({
    renderLink: ({ href, children, props }) => {
      if (href && skill) {
        if (!href.startsWith('http') && !href.startsWith('#')) {
          if (skillMaps.byName.has(href)) {
            const resolved = skillMaps.byName.get(href);
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
          }

          const childFlat = `${skill.flatName}__${href.replace(/\//g, '__')}`;
          const childSkill = skillMaps.byFlat.get(childFlat);
          if (childSkill) {
            return (
              <Link
                to={`/skills/${encodeURIComponent(childSkill.flatName)}`}
                className="link-subtle inline-flex items-center gap-0.5"
              >
                {children}
                <ArrowUpRight size={12} strokeWidth={2.5} className="shrink-0" />
              </Link>
            );
          }

          const matchedFile = files.find((file) => file === href || file.endsWith('/' + href));
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

      return (
        <a href={href} target="_blank" rel="noopener noreferrer" {...props}>
          {children}
        </a>
      );
    },
  }), [files, skill, skillMaps.byFlat, skillMaps.byName]);

  useEffect(() => {
    if (!skill) return;

    const skillChanged = lastLoadedSkillRef.current !== skill.flatName;
    lastLoadedSkillRef.current = skill.flatName;

    if (skillChanged) {
      contentAuthorityRef.current = 'query';
      setSavedContent(skillMdContent);
      setDraftContent(skillMdContent);
      setSaveError(null);
      return;
    }

    if (skillMdContent === savedContent) {
      contentAuthorityRef.current = 'query';
      return;
    }

    if (!isDirty && contentAuthorityRef.current === 'query') {
      setSavedContent(skillMdContent);
      setDraftContent(skillMdContent);
      setSaveError(null);
    }
  }, [isDirty, savedContent, skill, skillMdContent]);

  useEffect(() => {
    if (!skill) return;
    setShowFrontmatterGuide(!savedHasConfiguredFrontmatter);
  }, [savedHasConfiguredFrontmatter, skill?.flatName]);

  useBeforeUnload(
    (event) => {
      if (!isDirty) return;
      event.preventDefault();
      event.returnValue = '';
    },
    { capture: true },
  );

  useEffect(() => {
    if (mode === 'read') return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== 's') return;
      event.preventDefault();
      if (!isDirty || isSaving) return;
      void handleSave();
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [draftContent, isDirty, isSaving, mode, skill, name]);

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
  if (!data || !skill) return null;
  const currentSkill = skill;

  /** Try to resolve a file path to a known skill */
  function resolveFileSkill(filePath: string): Skill | undefined {
    // Skip non-directory files (files with extensions)
    if (/\.[a-z]+$/i.test(filePath) && !filePath.endsWith('.md')) return undefined;
    const flat = `${currentSkill.flatName}__${filePath.replace(/\//g, '__')}`;
    return skillMaps.byFlat.get(flat);
  }

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
            <div
              className="ss-detail-manifest mb-4 p-4 pt-5 border-2 border-dashed border-pencil-light/30"
              style={{ borderRadius: radius.sm }}
            >
              <div className="mb-4 space-y-3">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  {parsedDoc.manifest.name ? (
                    <div className="min-w-0 flex-1">
                      <div className="text-sm text-muted-dark uppercase tracking-wide">Name</div>
                      <div className="text-xl font-bold text-pencil">{parsedDoc.manifest.name}</div>
                    </div>
                  ) : (
                    <div className="min-w-0 flex-1" />
                  )}
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setShowFrontmatterGuide((current) => !current)}
                  >
                    {showFrontmatterGuide ? <ChevronUp size={14} strokeWidth={2.5} /> : <ChevronDown size={14} strokeWidth={2.5} />}
                    {showFrontmatterGuide ? 'Hide frontmatter' : 'Show frontmatter'}
                  </Button>
                </div>

                {parsedDoc.manifest.description ? (
                  <div className="min-w-0">
                    <div className="text-sm text-muted-dark uppercase tracking-wide">Description</div>
                    <div className="text-base text-pencil">{parsedDoc.manifest.description}</div>
                  </div>
                ) : null}
              </div>

              {!hasConfiguredFrontmatter ? (
                <p className="rounded-[var(--radius-sm)] border border-muted/80 bg-paper/60 px-3 py-2 text-sm text-pencil-light">
                  No frontmatter fields are set yet. The field guide below shows every option you can add.
                </p>
              ) : null}

              {compactFrontmatterEntries.length > 0 || additionalFrontmatterEntries.length > 0 ? (
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                  {compactFrontmatterEntries.map((entry) => (
                    <div key={entry.key} className="rounded-[var(--radius-sm)] border border-muted/80 bg-paper/60 px-3 py-2">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-dark">{entry.key}</div>
                      <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words text-sm text-pencil">
                        <code>{formatFrontmatterValue(entry.value)}</code>
                      </pre>
                    </div>
                  ))}
                  {additionalFrontmatterEntries.map((entry) => (
                    <div key={entry.key} className="rounded-[var(--radius-sm)] border border-muted/80 bg-paper/60 px-3 py-2">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-dark">{entry.key}</div>
                      <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words text-sm text-pencil">
                        <code>{formatFrontmatterValue(entry.value)}</code>
                      </pre>
                    </div>
                  ))}
                </div>
              ) : null}

              {showFrontmatterGuide ? (
                <div className="mt-4 border-t border-dashed border-pencil-light/30 pt-4">
                  <SkillFrontmatterGuide
                    frontmatter={parsedDoc.frontmatter}
                    headingLevel="h3"
                  />
                </div>
              ) : null}
            </div>
            <ContentStatsBar
              stats={displayedStats}
              fileCount={files.length}
              license={parsedDoc.manifest.license}
              tokenBreakdown={tokenBreakdown}
            />
            <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
              <SegmentedControl
                value={mode}
                onChange={setMode}
                options={[
                  { value: 'read', label: 'Read' },
                  { value: 'edit', label: 'Edit' },
                  { value: 'split', label: 'Split' },
                ]}
              />
              <div className="flex flex-wrap items-center gap-2">
                {isSaving ? (
                  <span className="rounded-full border border-blue bg-blue/10 px-3 py-1 text-sm font-medium text-blue inline-flex items-center gap-1.5">
                    <Spinner size="sm" />
                    Saving...
                  </span>
                ) : null}
                {isDirty ? (
                  <span className="rounded-full border border-warning bg-warning-light px-3 py-1 text-sm font-medium text-warning">
                    Unsaved changes
                  </span>
                ) : null}
              </div>
            </div>
            {saveError ? (
              <div
                role="alert"
                className="mb-4 rounded-[var(--radius-md)] border border-danger/40 bg-danger/5 px-4 py-3 text-sm text-danger"
              >
                {saveError}
              </div>
            ) : null}
            {mode === 'read' ? (
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
            ) : mode === 'edit' ? (
              <Suspense fallback={<EditorShellFallback mode="edit" />}>
                <div className={editorShellClassName(isSaving)} aria-busy={isSaving}>
                  <SkillMarkdownEditor
                    value={draftContent}
                    onChange={(next) => {
                      setDraftContent(next);
                      setSaveError(null);
                    }}
                    onSave={(next) => void handleSave(next)}
                    onDiscard={handleDiscardDraft}
                    onSurfaceChange={setEditorSurface}
                    surface={editorSurface}
                    mode="edit"
                    isDirty={isDirty}
                  />
                </div>
              </Suspense>
            ) : (
              <div className="grid grid-cols-1 items-stretch gap-4 xl:grid-cols-2 xl:auto-rows-fr">
                <Suspense fallback={<EditorShellFallback mode="split" />}>
                  <div className={editorShellClassName(isSaving, true)} aria-busy={isSaving}>
                    <SkillMarkdownEditor
                      value={draftContent}
                      onChange={(next) => {
                        setDraftContent(next);
                        setSaveError(null);
                      }}
                      onSave={(next) => void handleSave(next)}
                      onDiscard={handleDiscardDraft}
                      onSurfaceChange={setEditorSurface}
                      surface={editorSurface}
                      mode="split"
                      isDirty={isDirty}
                    />
                  </div>
                </Suspense>
                <div className="flex h-full min-h-0 flex-col rounded-[var(--radius-lg)] border-2 border-muted bg-surface p-4">
                  <div className="mb-3 text-sm font-medium text-pencil-light">Preview</div>
                  <div className="prose-hand min-h-0 flex-1">
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
                </div>
              </div>
            )}
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
                  const { icon: FileIcon, className: iconClass } = getFileIcon(f);
                  const isOpening = openingFilePath === f;
                  const canPreviewInModal = f !== 'SKILL.md';
                  return (
                    <li
                      key={f}
                      className="text-sm text-pencil-light truncate flex items-center gap-2"
                    >
                      <FileIcon size={14} strokeWidth={2} className={`shrink-0 ${iconClass}`} />
                      <Button
                        variant="link"
                        onClick={() => void handleOpenLocalFile(f)}
                        disabled={isOpening}
                        aria-label={`Open file ${f} locally`}
                        className="font-mono link-subtle text-left truncate inline-flex items-center"
                        style={{ fontSize: '0.8125rem' }}
                        title={`Open locally: ${f}`}
                      >
                        {f}
                      </Button>
                      {canPreviewInModal ? (
                        <Button
                          variant="link"
                          onClick={() => setViewingFile(f)}
                          aria-label={`Preview file ${f}`}
                          className="link-subtle inline-flex items-center gap-0.5 shrink-0"
                          title={`Preview file: ${f}`}
                        >
                          <Eye size={12} strokeWidth={2.5} className="shrink-0" />
                        </Button>
                      ) : null}
                      {linkedSkill ? (
                        <Link
                          to={`/skills/${encodeURIComponent(linkedSkill.flatName)}`}
                          className="link-subtle inline-flex items-center gap-0.5 shrink-0"
                          title={`View nested skill: ${linkedSkill.name}`}
                          aria-label={`View nested skill ${linkedSkill.name}`}
                        >
                          <ArrowUpRight size={11} strokeWidth={2.5} className="shrink-0" />
                        </Link>
                      ) : null}
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

      <ConfirmDialog
        open={blocker.state === 'blocked'}
        title="Unsaved Changes"
        message="You have unsaved changes that will be lost. Discard them?"
        confirmText="Discard"
        variant="danger"
        onConfirm={() => blocker.proceed?.()}
        onCancel={() => blocker.reset?.()}
      />
    </div>
  );
}

function editorShellClassName(isSaving: boolean, stretch = false): string {
  return [
    'transition-opacity',
    stretch ? 'h-full min-h-0' : '',
    isSaving ? 'pointer-events-none opacity-60' : '',
  ].filter(Boolean).join(' ');
}

function EditorShellFallback({ mode }: { mode: 'edit' | 'split' }) {
  return (
    <div className="rounded-[var(--radius-lg)] border-2 border-muted bg-surface p-4 min-h-[20rem]">
      <div className="flex items-center gap-2 text-sm text-pencil-light">
        <Spinner size="sm" />
        Loading editor for {mode} mode...
      </div>
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
    <Card variant="outlined" className="ss-detail-pinned ss-detail-pinned-green">
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
    <Card className="ss-detail-pinned ss-detail-pinned-blue">
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
                e.status === 'skill_target_mismatch' ? 'bg-warning' :
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
                e.status === 'skill_target_mismatch' ? 'text-warning' :
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
    <Card variant="outlined" className="ss-detail-pinned ss-detail-pinned-cyan">
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
