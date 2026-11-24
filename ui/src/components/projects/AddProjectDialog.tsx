import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, Check, Folder, TriangleAlert, X } from 'lucide-react';
import { api, type ProjectList, type ProjectScan, type ProjectStorage } from '../../api/client';
import { hooksApi } from '../../api/hooks';
import { mcpApi, mcpTargets } from '../../api/mcp';
import Button from '../Button';
import { Select } from '../Select';
import DialogShell from '../DialogShell';
import { useT, plural } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import PatternInput from '../targets/PatternInput';
import ProjectTools from './ProjectTools';
import { baseName, projectUrl, writeFolders } from './projectView';
import { opensProject, renamed, scanCounts, scanNote, scanWarning } from './scanView';

interface Props {
  tools: ProjectList['tools'];
  common: string[];
  /** The layer a new project goes to unless the user picks the other */
  defaultStorage?: ProjectStorage;
  /** The file each layer is saved in */
  storagePaths?: Record<ProjectStorage, string>;
  /** Absolute paths of the projects that exist */
  existing: string[];
  onClose: () => void;
  onAdded: (root: string) => void;
  /** After adding a folder's checkouts too; count is how many projects were added */
  onScanned: (count: number) => void;
}

const PARTS = ['skills', 'agents', 'mcp', 'hooks'] as const;
type PartName = (typeof PARTS)[number];
const PART_LABEL: Record<PartName, string> = {
  skills: 'Skills',
  agents: 'Agents',
  mcp: 'MCP',
  hooks: 'Hooks',
};
const RESULT = {
  added: { label: 'Added', tone: 'ok' },
  kept: { label: 'Kept', tone: 'off' },
  skipped: { label: 'Skipped', tone: 'warn' },
} as const;

/**
 * Add one folder to projects, or that folder and each Git checkout inside it
 * after a review. Filters and MCP servers are edited on the project's page.
 */
export default function AddProjectDialog({
  tools,
  common,
  defaultStorage = 'shared',
  storagePaths,
  existing,
  onClose,
  onAdded,
  onScanned,
}: Props) {
  const t = useT();
  const [path, setPath] = useState('');
  const [targets, setTargets] = useState<string[]>([]);
  const [parts, setParts] = useState<Record<PartName, boolean>>({
    skills: true,
    agents: false,
    mcp: false,
    hooks: false,
  });
  const [storage, setStorage] = useState<ProjectStorage>(defaultStorage);
  const [scan, setScan] = useState(false);
  const [scanDirs, setScanDirs] = useState<string[]>(['.']);
  // The dry run being reviewed; nothing is written until it is confirmed.
  const [review, setReview] = useState<ProjectScan | null>(null);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  // Trailing separators aside, this is the key config.yaml will hold.
  const root = path.trim().replace(/(?<=.)[\\/]+$/, '');
  const absolute = /^(~($|[\\/])|\/|[A-Za-z]:[\\/])/.test(root);
  const project = (key: string) => existing.find((x) => x === key || shortenHome(x) === key);
  // A scan keeps a folder that is already a project, so only a single add refuses it.
  const taken = !scan && Boolean(project(root));
  const pathError =
    root && !absolute ? t('projects.add.folderHint') : taken ? t('projects.add.taken') : '';
  // The server refuses these too, but in CLI words.
  const scanError =
    scan && scanDirs.some((d) => /^([\\/]|[A-Za-z]:)/.test(d) || d.split(/[\\/]/).includes('..'))
      ? t('projects.scan.inside')
      : '';
  const agentTools = targets.filter((x) => tools.find((tool) => tool.name === x)?.agentsPath);
  const folders = writeFolders(tools, targets, 'skill').folders.length;
  const counts = scanCounts(review?.rows ?? []);
  const canSave =
    Boolean(root) &&
    !pathError &&
    targets.length > 0 &&
    PARTS.some((p) => parts[p]) &&
    (!scan || scanDirs.length > 0) &&
    !scanError &&
    (!review || counts.added > 0) &&
    !saving;

  const partHint = (p: PartName) => {
    if (p === 'agents' && targets.length > 0) {
      return agentTools.length === 0
        ? t('projects.add.agentsNone')
        : agentTools.length < targets.length
          ? t('projects.add.agentsSome', { targets: agentTools.join(', ') })
          : t('projects.add.agentsHint');
    }
    return t(`projects.add.${p}Hint`);
  };

  const where = (layer: ProjectStorage) => {
    const full = storagePaths?.[layer];
    return full ? { detail: shortenHome(full), detailTitle: full } : {};
  };
  const storageOptions = [
    {
      value: 'shared',
      label: 'config.yaml',
      mono: true,
      description: t('projects.add.storageShared'),
      ...where('shared'),
    },
    {
      value: 'local',
      label: t('projects.add.storageLocal'),
      triggerLabel: t('projects.add.storageLocalShort'),
      description: t('projects.add.storageLocalHint'),
      ...where('local'),
    },
  ];

  // MCP and hooks are not part of a project entry; each new project gets them here.
  const addParts = async (roots: string[]) => {
    const clients = mcpTargets.filter((x) => targets.includes(x));
    for (const project of roots) {
      if (parts.mcp) {
        await mcpApi.save({
          project,
          settings: { targets: clients.length > 0 ? clients : undefined },
        });
      }
      // An empty hooks block: the project's own hooks are added on its Hooks tab.
      if (parts.hooks) await hooksApi.save({ project });
    }
  };

  const save = async () => {
    if (!canSave) return;
    setSaving(true);
    setError('');
    const all = { mode: '', include: [], exclude: [] };
    const settings = {
      // Like the CLI without a flag, the default storage keeps a project saved in the other place.
      storage: scan && storage === defaultStorage ? undefined : storage,
      root,
      targets,
      skills: parts.skills ? all : null,
      agents: parts.agents ? all : null,
    };
    try {
      if (!scan) {
        const saved = await api.saveProject({ create: true, ...settings });
        await addParts([saved.root]);
        onAdded(saved.root);
        return;
      }
      const result = await api.scanProjects({ ...settings, scan: scanDirs, dryRun: !review });
      if (!review) {
        setReview(result);
        setSaving(false);
        return;
      }
      // The write scans again, so count what it added, not what was reviewed.
      const added = result.rows.filter((r) => r.result === 'added').map((r) => r.root);
      if (added.length === 0) {
        setReview(result);
        setSaving(false);
        return;
      }
      try {
        await addParts(added);
      } catch (e) {
        // The projects are saved; a new review would only show them as kept.
        setReview(null);
        throw e;
      }
      onScanned(added.length);
    } catch (e) {
      setError((e as Error).message);
      setSaving(false);
    }
  };

  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      preventClose={saving}
      ariaLabel={t('projects.add.title')}
      className="!max-w-[640px]"
    >
      <div className={`dh ${review ? '!gap-2' : ''}`}>
        {review && (
          <button
            type="button"
            className="ss-ib"
            aria-label={t('common.back')}
            onClick={() => setReview(null)}
            disabled={saving}
          >
            <ArrowLeft size={16} />
          </button>
        )}
        <h2 className="ss-h2 flex-1">
          {t(review ? 'projects.scan.reviewTitle' : 'projects.add.title')}
        </h2>
        <button
          type="button"
          className="ss-ib"
          aria-label={t('common.close')}
          onClick={onClose}
          disabled={saving}
        >
          <X size={16} />
        </button>
      </div>
      <form
        id="add-project"
        className="db"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        {review ? (
          <>
            <div className="flex flex-col gap-1">
              <span className="font-mono text-[13px] font-semibold">{root}</span>
              <span className="text-[13px] text-ink-2">
                {t('projects.scan.same', {
                  targets: targets.join(', '),
                  parts: PARTS.filter((p) => parts[p])
                    .map((p) => PART_LABEL[p])
                    .join(' · '),
                })}
              </span>
            </div>
            <div className="flex flex-wrap gap-4">
              <span className="ss-st ok">
                {t(plural('projects.scan.toAdd', counts.added), { count: counts.added })}
              </span>
              {counts.kept > 0 && (
                <span className="ss-st off">
                  {t(plural('projects.scan.already', counts.kept), { count: counts.kept })}
                </span>
              )}
              {counts.skipped > 0 && (
                <span className="ss-st warn">
                  {t(plural('projects.scan.skipped', counts.skipped), { count: counts.skipped })}
                </span>
              )}
            </div>
            <div className="ss-list">
              {review.rows.map((row, i) => {
                const note = scanNote(row);
                const target = opensProject(row) ? project(row.root) : undefined;
                return (
                  <div
                    key={row.root}
                    className={`flex min-h-[52px] items-start gap-3 px-4 py-2.5 ${i > 0 ? 'border-t border-line-soft' : ''}`}
                  >
                    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-[13px] font-semibold break-all">
                          {row.path === '.' ? baseName(root) : row.path}
                        </span>
                        {renamed(row) && <span className="ss-tag">{row.name}</span>}
                      </span>
                      {(note || row.reason) && (
                        <span className="text-[13px] text-ink-2">
                          {note ? t(note.key, note.params) : row.reason}
                          {target && (
                            <>
                              {' '}
                              <Link to={projectUrl(target)} className="text-accent hover:underline">
                                {t('projects.scan.open', { name: row.name })}
                              </Link>
                            </>
                          )}
                        </span>
                      )}
                    </span>
                    <span className={`ss-st w-[72px] shrink-0 ${RESULT[row.result].tone}`}>
                      {RESULT[row.result].label}
                    </span>
                  </div>
                );
              })}
            </div>
            {review.warnings.length > 0 && (
              <div className="ss-note warn">
                <TriangleAlert size={16} />
                <span className="flex flex-1 flex-col gap-1">
                  {review.warnings.map((w) => {
                    const message = scanWarning(w);
                    return (
                      <span key={`${w.kind}:${w.folder}`}>{t(message.key, message.params)}</span>
                    );
                  })}
                </span>
              </div>
            )}
            {counts.added === 0 && <span className="hp">{t('projects.scan.nothing')}</span>}
          </>
        ) : (
          <>
            <div className="ss-fld">
              <label htmlFor="add-project-path">{t('projects.add.folder')}</label>
              <span className={`ss-inp font-mono ${pathError ? 'err' : ''}`}>
                <Folder size={15} className="shrink-0 text-ink-3" />
                <input
                  id="add-project-path"
                  autoFocus
                  value={path}
                  onChange={(e) => setPath(e.target.value)}
                  placeholder="~/work/my-project"
                  disabled={saving}
                />
              </span>
              <span className={`hp ${pathError ? '!text-bad' : ''}`}>
                {pathError || t('projects.add.folderHint')}
              </span>
            </div>
            <div className="ss-fld">
              <span className="text-[13px] font-semibold">{t('projects.targets')}</span>
              <ProjectTools
                tools={tools}
                common={common}
                selected={targets}
                onChange={setTargets}
                disabled={saving}
              />
              <span className="hp">{t('projects.add.targetsHint')}</span>
              {folders > 0 && (
                <span className="hp">
                  {t(plural('projects.add.writes', folders), { count: folders })}
                </span>
              )}
            </div>
            <div className="ss-fld">
              <span className="text-[13px] font-semibold">{t('projects.add.parts')}</span>
              <div className="ss-list">
                {PARTS.map((p, i) => (
                  <button
                    key={p}
                    type="button"
                    role="checkbox"
                    aria-checked={parts[p]}
                    className={`flex min-h-[52px] w-full items-center gap-3 px-4 py-2 text-left ${i > 0 ? 'border-t border-line-soft' : ''}`}
                    onClick={() => setParts({ ...parts, [p]: !parts[p] })}
                    disabled={saving}
                  >
                    <span className={`ss-chk ${parts[p] ? 'on' : ''}`}>
                      {parts[p] && <Check size={12} strokeWidth={3} />}
                    </span>
                    <span className="flex flex-col gap-px">
                      <span className="text-[14px] font-semibold">{PART_LABEL[p]}</span>
                      <span className="text-[13px] text-ink-2">{partHint(p)}</span>
                    </span>
                  </button>
                ))}
              </div>
            </div>
            <div className="ss-fld">
              <span className="text-[13px] font-semibold">{t('projects.scan.title')}</span>
              <div className="ss-list">
                <button
                  type="button"
                  role="checkbox"
                  aria-checked={scan}
                  className="flex min-h-[52px] w-full items-center gap-3 px-4 py-2 text-left"
                  onClick={() => setScan(!scan)}
                  disabled={saving}
                >
                  <span className={`ss-chk ${scan ? 'on' : ''}`}>
                    {scan && <Check size={12} strokeWidth={3} />}
                  </span>
                  <span className="flex flex-col gap-px">
                    <span className="text-[14px] font-semibold">{t('projects.scan.toggle')}</span>
                    <span className="text-[13px] text-ink-2">{t('projects.scan.toggleHint')}</span>
                  </span>
                </button>
                {scan && (
                  <div className="flex flex-col gap-2 border-t border-line-soft bg-sunken px-4 py-3">
                    <label htmlFor="add-project-scan" className="text-[13px] text-ink-2">
                      {t('projects.scan.lookIn')}
                    </label>
                    <PatternInput
                      id="add-project-scan"
                      patterns={scanDirs}
                      onChange={setScanDirs}
                      placeholder={t('projects.scan.addFolder')}
                      disabled={saving}
                    />
                    <span className={`hp ${scanError ? '!text-bad' : ''}`}>
                      {scanError || t('projects.scan.foldersHint')}
                    </span>
                  </div>
                )}
              </div>
            </div>
          </>
        )}
        {error && (
          <div className="ss-note bad">
            <span className="flex-1">{error}</span>
          </div>
        )}
      </form>
      <div className="df flex-wrap">
        <span className="flex flex-1 items-center gap-1 text-[13px] whitespace-nowrap text-ink-3">
          {t('projects.add.savedTo')}
          <Select
            quiet
            up
            ariaLabel={t('projects.add.savedTo')}
            value={storage}
            onChange={(v) => setStorage(v as ProjectStorage)}
            options={storageOptions}
            disabled={saving || Boolean(review)}
          />
        </span>
        <span className="ml-auto flex gap-2">
          <Button
            variant="ghost"
            onClick={review ? () => setReview(null) : onClose}
            disabled={saving}
          >
            {t(review ? 'common.back' : 'common.cancel')}
          </Button>
          <Button
            type="submit"
            form="add-project"
            variant="primary"
            loading={saving}
            disabled={!canSave}
          >
            {review
              ? t(plural('projects.scan.submit', counts.added), { count: counts.added })
              : t(scan ? 'projects.scan.review' : 'projects.add.submit')}
          </Button>
        </span>
      </div>
    </DialogShell>
  );
}
