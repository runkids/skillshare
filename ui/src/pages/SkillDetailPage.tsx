import { useParams, useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, Trash2, ExternalLink, FileText, ArrowUpRight, RefreshCw } from 'lucide-react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import Badge from '../components/Badge';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import ConfirmDialog from '../components/ConfirmDialog';
import { api, type Skill } from '../api/client';
import { useApi } from '../hooks/useApi';
import { lazy, Suspense, useState, useMemo } from 'react';
import { wobbly, shadows } from '../design';

const FileViewerModal = lazy(() => import('../components/FileViewerModal'));

type SkillManifest = {
  name?: string;
  description?: string;
};

function parseScalarValue(raw: string): string | undefined {
  const trimmed = raw.trim();
  if (!trimmed) return undefined;
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1).trim() || undefined;
  }
  return trimmed;
}

function extractManifestValue(frontmatter: string, key: 'name' | 'description'): string | undefined {
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
  };

  const markdown = content.slice(match[0].length);
  return { manifest, markdown };
}

function skillTypeLabel(type?: string): string | undefined {
  if (!type) return undefined;
  if (type === 'github-subdir') return 'github';
  return type;
}

export default function SkillDetailPage() {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const { data, loading, error, refetch } = useApi(() => api.getSkill(name!), [name]);
  const allSkills = useApi(() => api.listSkills());
  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [updating, setUpdating] = useState(false);
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

  if (loading) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load skill
        </p>
        <p className="text-pencil-light text-sm mt-1">{error}</p>
      </Card>
    );
  }
  if (!data) return null;

  const { skill, skillMdContent, files: rawFiles } = data;
  const files = rawFiles ?? [];
  const parsedDoc = parseSkillMarkdown(skillMdContent ?? '');
  const hasManifest = Boolean(parsedDoc.manifest.name || parsedDoc.manifest.description);
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
                className="text-blue inline-flex items-center gap-0.5 hover:text-accent transition-colors"
                style={{ textDecoration: 'underline', textDecorationStyle: 'wavy', textUnderlineOffset: '3px' }}
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
              <button
                onClick={() => setViewingFile(matchedFile)}
                className="text-blue inline-flex items-center gap-0.5 hover:text-accent transition-colors cursor-pointer"
                style={{ textDecoration: 'underline', textDecorationStyle: 'wavy', textUnderlineOffset: '3px', background: 'none', border: 'none', padding: 0, font: 'inherit' }}
              >
                {children}
              </button>
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
      navigate('/skills');
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
      setDeleting(false);
      setConfirmDelete(false);
    }
  };

  const handleUpdate = async () => {
    setUpdating(true);
    try {
      const res = await api.update({ name: skill.isInRepo ? skill.relPath.split('/')[0] : skill.relPath });
      const item = res.results[0];
      if (item?.action === 'updated') {
        toast(`Updated: ${item.name} — ${item.message}`, 'success');
        refetch();
      } else if (item?.action === 'up-to-date') {
        toast(`${item.name} is already up to date.`, 'info');
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

  return (
    <div className="animate-sketch-in">
      {/* Header — sticky */}
      <div className="flex items-center gap-3 mb-6 sticky top-0 z-20 bg-paper py-3 -mx-4 px-4 md:-mx-8 md:px-8 -mt-3">
        <button
          onClick={() => navigate('/skills')}
          className="w-9 h-9 flex items-center justify-center bg-white border-2 border-pencil text-pencil-light hover:text-pencil transition-colors cursor-pointer"
          style={{
            borderRadius: wobbly.sm,
            boxShadow: shadows.sm,
          }}
        >
          <ArrowLeft size={18} strokeWidth={2.5} />
        </button>
        <div className="flex items-center gap-3 flex-wrap">
          <h2
            className="text-2xl md:text-3xl font-bold text-pencil"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            {skill.name}
          </h2>
          {skill.isInRepo && <Badge variant="warning">tracked repo</Badge>}
          {skillTypeLabel(skill.type) && <Badge variant="info">{skillTypeLabel(skill.type)}</Badge>}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main content: SKILL.md */}
        <div className="lg:col-span-2">
          <Card decoration="tape">
            {hasManifest && (
              <div
                className="mb-4 p-4 border-2 border-dashed border-pencil-light/40 bg-postit/35"
                style={{ borderRadius: wobbly.sm }}
              >
                <p
                  className="text-sm uppercase tracking-wider text-pencil-light mb-2"
                  style={{ fontFamily: 'var(--font-heading)' }}
                >
                  SKILL.md Manifest
                </p>
                <dl className="space-y-2">
                  {parsedDoc.manifest.name && (
                    <div>
                      <dt className="text-sm text-muted-dark uppercase tracking-wide">Name</dt>
                      <dd
                        className="text-xl font-bold text-pencil"
                        style={{ fontFamily: 'var(--font-heading)' }}
                      >
                        {parsedDoc.manifest.name}
                      </dd>
                    </div>
                  )}
                  {parsedDoc.manifest.description && (
                    <div>
                      <dt className="text-sm text-muted-dark uppercase tracking-wide">Description</dt>
                      <dd className="text-base text-pencil">{parsedDoc.manifest.description}</dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
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

        {/* Sidebar: metadata + files — sticky */}
        <div className="space-y-5 lg:sticky lg:top-16 lg:self-start">
          <Card variant="postit">
            <h3
              className="font-bold text-pencil mb-3"
              style={{ fontFamily: 'var(--font-heading)' }}
            >
              Metadata
            </h3>
            <dl className="space-y-3">
              <MetaItem label="Path" value={skill.relPath} mono />
              {skill.source && <MetaItem label="Source" value={skill.source} mono />}
              {skill.version && <MetaItem label="Version" value={skill.version} mono />}
              {skill.installedAt && (
                <MetaItem
                  label="Installed"
                  value={new Date(skill.installedAt).toLocaleDateString()}
                />
              )}
              {skill.repoUrl && (
                <div>
                  <dt className="text-sm text-pencil-light uppercase tracking-wider">Repository</dt>
                  <dd>
                    <a
                      href={skill.repoUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue text-base inline-flex items-center gap-1 hover:text-accent transition-colors"
                      style={{ textDecoration: 'underline', textDecorationStyle: 'wavy' }}
                    >
                      <ExternalLink size={12} strokeWidth={2.5} />
                      {skill.repoUrl.replace('https://', '').replace('.git', '')}
                    </a>
                  </dd>
                </div>
              )}
            </dl>

            {/* Actions */}
            <div className="flex gap-2 mt-4 pt-4 border-t-2 border-dashed border-pencil-light/30">
              {(skill.isInRepo || skill.source) && (
                <HandButton
                  onClick={handleUpdate}
                  disabled={updating}
                  variant="secondary"
                  size="sm"
                  className="flex-1"
                >
                  <RefreshCw size={14} strokeWidth={2.5} className={updating ? 'animate-spin' : ''} />
                  {updating ? 'Updating...' : 'Update'}
                </HandButton>
              )}
              <HandButton
                onClick={() => setConfirmDelete(true)}
                disabled={deleting}
                variant="danger"
                size="sm"
                className="flex-1"
              >
                <Trash2 size={14} strokeWidth={2.5} />
                {deleting
                  ? 'Uninstalling...'
                  : skill.isInRepo
                    ? 'Uninstall Repo'
                    : 'Uninstall'}
              </HandButton>
            </div>
          </Card>

          <Card>
            <h3
              className="font-bold text-pencil mb-3 flex items-center gap-2"
              style={{ fontFamily: 'var(--font-heading)' }}
            >
              <FileText size={16} strokeWidth={2.5} />
              Files ({files.length})
            </h3>
            {files.length > 0 ? (
              <ul className="space-y-1.5 max-h-80 overflow-y-auto">
                {files.map((f) => {
                  const linkedSkill = resolveFileSkill(f);
                  const isSkillMd = f === 'SKILL.md';
                  return (
                    <li
                      key={f}
                      className="text-sm text-pencil-light truncate flex items-center gap-2"
                    >
                      <span className="w-1.5 h-1.5 rounded-full bg-muted-dark shrink-0" />
                      {linkedSkill ? (
                        <Link
                          to={`/skills/${encodeURIComponent(linkedSkill.flatName)}`}
                          className="text-blue hover:text-accent transition-colors inline-flex items-center gap-1"
                          style={{ fontFamily: "'Courier New', monospace", textDecoration: 'underline', textDecorationStyle: 'wavy', textUnderlineOffset: '2px' }}
                          title={`View skill: ${linkedSkill.name}`}
                        >
                          {f}
                          <ArrowUpRight size={11} strokeWidth={2.5} className="shrink-0" />
                        </Link>
                      ) : isSkillMd ? (
                        <span
                          className="truncate"
                          style={{ fontFamily: "'Courier New', monospace" }}
                        >
                          {f}
                        </span>
                      ) : (
                        <button
                          onClick={() => setViewingFile(f)}
                          className="text-blue hover:text-accent transition-colors text-left truncate cursor-pointer inline-flex items-center gap-1"
                          style={{ fontFamily: "'Courier New', monospace", textDecoration: 'underline', textDecorationStyle: 'wavy', textUnderlineOffset: '2px', background: 'none', border: 'none', padding: 0 }}
                          title={`View file: ${f}`}
                        >
                          {f}
                        </button>
                      )}
                    </li>
                  );
                })}
              </ul>
            ) : (
              <p className="text-sm text-muted-dark italic">No files.</p>
            )}
          </Card>
        </div>
      </div>

      {/* File viewer modal */}
      {viewingFile && (
        <Suspense fallback={null}>
          <FileViewerModal
            skillName={skill.flatName}
            filepath={viewingFile}
            onClose={() => setViewingFile(null)}
          />
        </Suspense>
      )}

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
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div>
      <dt className="text-sm text-pencil-light uppercase tracking-wider">{label}</dt>
      <dd
        className="text-base text-pencil break-all"
        style={mono ? { fontFamily: "'Courier New', monospace", fontSize: '0.875rem' } : undefined}
      >
        {value}
      </dd>
    </div>
  );
}
