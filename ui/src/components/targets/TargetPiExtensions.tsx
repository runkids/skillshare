import { useState } from 'react';
import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ArrowRight, ChevronDown, CircleCheck, Folder, Info, Lock, Puzzle, RotateCcw, X } from 'lucide-react';
import { ApiError } from '../../api/client';
import { piExtensionsApi } from '../../api/piExtensions';
import PiPackageIcon from '../PiPackageIcon';
import type { PiExtensionAction, PiExtensionChange, PiExtensionFolder, PiExtensionPackage, PiExtensionRow, PiExtensionsPlan, PiExtensionsView, PiSelection } from '../../api/piExtensions';
import { queryKeys } from '../../lib/queryKeys';
import { shortenHome } from '../../lib/paths';
import { useT } from '../../i18n';
import Button from '../Button';
import DialogShell from '../DialogShell';
import EmptyState from '../EmptyState';
import { PageSkeleton } from '../Skeleton';
import Tooltip from '../Tooltip';

type T = ReturnType<typeof useT>;
type Scope = PiExtensionChange['scope'];
type SetAction = (scope: Scope, index: number, path: string, action?: PiExtensionAction) => void;
// A project lists entries of two files, so an index is only unique with its scope.
const keyOf = (scope: Scope, index: number, path: string) => `${scope}\u0000${index}\u0000${path}`;
const selTone: Record<PiSelection, string> = { loads: 'ok', skipped: 'off', unknown: 'warn', none: 'off' };

/** The Extensions tab of a Pi target: what its settings select per package extension. */
export default function TargetPiExtensions({ name }: { name: string }) {
  const t = useT();
  const query = useQuery({ queryKey: queryKeys.piExtensions(name), queryFn: () => piExtensionsApi.get(name) });
  // Outlives the remount that a new revision causes, so the reload notice stays after apply.
  const [applied, setApplied] = useState('');
  if (query.error) return <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{query.error.message}</span></div>;
  if (!query.data) return <PageSkeleton />;
  // A new revision (applied here or edited elsewhere) starts from no pending changes.
  return <ExtensionsView key={query.data.revision} name={name} view={query.data} applied={applied} setApplied={setApplied} t={t} />;
}

function ExtensionsView({ name, view, applied, setApplied, t }: { name: string; view: PiExtensionsView; applied: string; setApplied: (path: string) => void; t: T }) {
  // Pending choices per package row; nothing is written until the review applies them.
  const [pending, setPending] = useState<Record<string, PiExtensionAction>>({});
  const [reviewing, setReviewing] = useState(false);
  const project = view.scope === 'project';
  // Opens the add dialog with this target ticked; a project's target is not a Plugins page Agent.
  const addPackage = `/plugins?add=${project ? '' : encodeURIComponent(name)}`;
  const changes: PiExtensionChange[] = Object.entries(pending).map(([key, action]) => {
    const [scope, index, path] = key.split('\u0000') as [Scope, string, string];
    const source = view.packages.find((p) => p.scope === scope && p.index === Number(index))?.source ?? '';
    return { scope, index: Number(index), source, path, action };
  });
  const set: SetAction = (scope, index, path, action) => {
    setApplied('');
    setPending((prev) => {
      const next = { ...prev };
      if (action) next[keyOf(scope, index, path)] = action;
      else delete next[keyOf(scope, index, path)];
      return next;
    });
  };
  const rows = view.packages.reduce((n, p) => n + p.rows.length, 0) + view.folders.reduce((n, f) => n + f.rows.length, 0);
  const empty = rows === 0 && view.packages.length === 0;

  return (
    <div className="flex flex-col gap-5">
      {applied && (
        <div className="ss-note ok" role="status"><CircleCheck size={16} /><span className="flex-1">{t(project ? 'targetDetail.piExtensions.appliedProject' : 'targetDetail.piExtensions.applied', { path: shortenHome(applied) })}</span></div>
      )}
      <ReadOnlyNote view={view} t={t} />
      {view.problem && <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{t(`targetDetail.piExtensions.settingsProblem.${view.problem}`)}</span></div>}
      <p className="flex items-center gap-1.5 text-[13px] text-ink-2">
        <span>{project ? t('targetDetail.piExtensions.projectHint', { path: shortenHome(view.settingsPath) }) : t('targetDetail.piExtensions.hint', { name })}</span>
        <PageInfo view={view} t={t} />
      </p>

      {empty ? (
        <EmptyState icon={Puzzle} title={t('targetDetail.piExtensions.emptyTitle')} description={t('targetDetail.piExtensions.emptyDescription', { name })} action={<Link to={addPackage} className="ss-btn">{t('targetDetail.piExtensions.addPackage')}<ArrowRight size={14} /></Link>} />
      ) : (
        <div className="flex flex-col gap-4">
          {view.packages.map((p) => (
            <PackageCard key={`${p.scope}:${p.index}`} pkg={p} name={name} ownsRules={!project || p.scope === 'project'} pending={pending} set={set} t={t} />
          ))}
          {view.folders.map((f) => <FolderCard key={`${f.scope ?? ''}:${f.path}`} folder={f} t={t} />)}
        </div>
      )}
      <p className="text-[13px] text-ink-3">
        {t('targetDetail.piExtensions.pluginsHint')} {!empty && <><Link to={addPackage} className="font-semibold text-ink-2 hover:text-ink">{t('targetDetail.piExtensions.addPackage')}</Link> · </>}<Link to="/plugins" className="font-semibold text-ink-2 hover:text-ink">{t('targetDetail.piExtensions.openPlugins')}</Link>
      </p>

      {changes.length > 0 && (
        <div className="sticky bottom-4 flex items-center gap-3 rounded-[var(--r-box)] border border-line-2 bg-surface px-4 py-3 shadow-[var(--sh-float)]">
          <span className="flex-1 text-[13.5px]">
            <span className="font-semibold">{t(changes.length === 1 ? 'targetDetail.piExtensions.pending.one' : 'targetDetail.piExtensions.pending.other', { count: changes.length })}</span>
            <span className="text-ink-3"> · {t('targetDetail.piExtensions.pending.nothingYet')}</span>
          </span>
          <Button variant="ghost" onClick={() => setPending({})}>{t('targetDetail.piExtensions.discard')}</Button>
          <Button variant="primary" onClick={() => setReviewing(true)}>{t('targetDetail.piExtensions.review')}</Button>
        </div>
      )}
      {reviewing && (
        <ReviewDialog
          name={name}
          view={view}
          changes={changes}
          onClose={() => setReviewing(false)}
          onApplied={(plan) => { setReviewing(false); setPending({}); setApplied(plan.settingsPath); }}
          t={t}
        />
      )}
    </div>
  );
}

function ReadOnlyNote({ view, t }: { view: PiExtensionsView; t: T }) {
  if (!view.readOnly) return null;
  const values = { name: view.target, version: view.version || '?', min: view.minVersion };
  return <div className="ss-note warn"><AlertCircle size={16} /><span className="flex-1">{t(`targetDetail.piExtensions.readOnly.${view.readOnly}`, values)}</span></div>;
}

/** What this page does and doesn't decide, with a project's trust hints, kept out of the page until asked for. */
function PageInfo({ view, t }: { view: PiExtensionsView; t: T }) {
  const content = (
    <span className="flex flex-col gap-2">
      <span>{t('targetDetail.piExtensions.info.text')}</span>
      {view.scope === 'project' && <span>{t('targetDetail.piExtensions.trust.detail')}</span>}
      {view.trust && (
        <span>
          {t('targetDetail.piExtensions.trust.saved', { value: view.trust.saved })} · {t('targetDetail.piExtensions.trust.default', { value: view.trust.default })} · {t('targetDetail.piExtensions.trust.hint')}
        </span>
      )}
    </span>
  );
  return (
    <Tooltip content={content}>
      <button type="button" className="ss-ib !h-6 !w-6 !text-inherit" aria-label={t('targetDetail.piExtensions.info.label')}><Info size={14} /></button>
    </Tooltip>
  );
}

/** The folder all paths share ("extensions/"), shown once above them instead of on every row. */
function sharedDir(paths: string[]) {
  const dir = paths[0]?.slice(0, paths[0].lastIndexOf('/') + 1) ?? '';
  return dir && paths.every((p) => p.startsWith(dir)) ? dir : '';
}

/** Whether a row ends up on: a pending switch decides, a pending rule removal keeps what the settings say. */
const isOn = (row: PiExtensionRow, action?: PiExtensionAction) =>
  action === 'default' && row.unruled ? row.unruled === 'loads' : action === 'select' || (action !== 'exclude' && row.selection === 'loads');

/** One package: a summary header, then its extensions as a grid. ownsRules: the rules shown are in the file this view writes, so one can be removed. */
function PackageCard({ pkg, name, ownsRules, pending, set, t }: { pkg: PiExtensionPackage; name: string; ownsRules: boolean; pending: Record<string, PiExtensionAction>; set: SetAction; t: T }) {
  const [open, setOpen] = useState(true);
  const [details, setDetails] = useState(false);
  // A plugin Skillshare installs is a local path into its state directory: its name says which one.
  const title = pkg.managedBy || pkg.identity || pkg.source;
  const actionOf = (r: PiExtensionRow) => pending[keyOf(pkg.scope, pkg.index, r.path)];
  const on = pkg.rows.filter((r) => isOn(r, actionOf(r))).length;
  const locked = Boolean(pkg.readOnly) || (pkg.rows.length > 0 && pkg.rows.every((r) => !r.editable));
  const dir = sharedDir(pkg.rows.map((r) => r.path));
  const list = (rules: string[] | null | undefined) => (rules ? rules.join(', ') || '[]' : t('targetDetail.piExtensions.details.none'));
  return (
    <section className="ss-list !shadow-none" aria-label={title}>
      <div className="ss-r !min-h-[60px]">
        <span className="ss-cat bg-sunken text-ink" aria-hidden><PiPackageIcon size={26} /></span>
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="nm m truncate" title={pkg.source}>{title}</span>
          {(pkg.kind || pkg.version || pkg.shape || pkg.managedBy) && (
            <span className="flex flex-wrap items-center gap-1.5">
              {(pkg.kind || pkg.version) && <span className="ss-tag">{[pkg.kind, pkg.version].filter(Boolean).join(' ')}</span>}
              {pkg.shape && <span className="ss-tag inf">{t(`targetDetail.piExtensions.shape.${pkg.shape}`)}</span>}
              {pkg.managedBy && <Link to="/plugins" className="ss-tag hover:text-ink">{t('targetDetail.piExtensions.managedBy')}</Link>}
            </span>
          )}
        </span>
        {pkg.rows.length > 0 && <span className="shrink-0 text-[13px] text-ink-2">{t('targetDetail.piExtensions.summary', { on, total: pkg.rows.length })}</span>}
        {locked && <span className="ss-tag shrink-0"><Lock size={11} />{t('targetDetail.piExtensions.readOnlyTag')}</span>}
        <button type="button" className="inline-flex shrink-0 items-center gap-1 text-[12px] font-semibold text-ink-2 hover:text-ink" aria-expanded={details} onClick={() => setDetails(!details)}>
          {t('targetDetail.piExtensions.details.show')}<ChevronDown size={12} className={details ? 'rotate-180' : ''} />
        </button>
        <button type="button" className="ss-ib shrink-0" aria-expanded={open} aria-label={t(open ? 'targetDetail.piExtensions.collapse' : 'targetDetail.piExtensions.expand', { pkg: title })} onClick={() => setOpen(!open)}>
          <ChevronDown size={16} className={open ? 'rotate-180' : ''} />
        </button>
      </div>
      {details && (
        <div role="region" aria-label={t('targetDetail.piExtensions.details.label', { pkg: title })} className="ss-r !min-h-0 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-[12.5px]">
          <span className="text-ink-3">{t('targetDetail.piExtensions.details.source')}</span><span className="break-all font-mono">{pkg.source}</span>
          <span className="text-ink-3">{t('targetDetail.piExtensions.details.rules')}</span><span className="break-all font-mono">{list(pkg.rules)}</span>
          {pkg.globalRules && <><span className="text-ink-3">{t('targetDetail.piExtensions.details.globalRules')}</span><span className="break-all font-mono">{list(pkg.globalRules)}</span></>}
          <span className="text-ink-3">{t('targetDetail.piExtensions.details.otherKeys')}</span><span className="font-mono">{pkg.otherKeys.join(', ') || t('targetDetail.piExtensions.details.none')}</span>
        </div>
      )}
      {open && (
        <>
          {pkg.problem && <div className="ss-r !min-h-0 bg-sunken text-[13px] text-ink-2"><AlertCircle size={14} className="shrink-0 text-warn" />{t(`targetDetail.piExtensions.problem.${pkg.problem}`)}</div>}
          {pkg.readOnly && <div className="ss-r !min-h-0 bg-sunken text-[13px] text-ink-2"><Lock size={14} className="shrink-0 text-ink-3" />{t(`targetDetail.piExtensions.packageReadOnly.${pkg.readOnly}`)}</div>}
          {pkg.rows.length === 0 && !pkg.problem && <div className="ss-r !min-h-[42px] text-[13px] text-ink-3">{t('targetDetail.piExtensions.noExtensions')}</div>}
          {pkg.rows.length > 0 && (
            <RowGrid dir={dir}>
              {pkg.rows.map((r) => (
                <ExtensionCell key={r.path} pkg={pkg} row={r} dir={dir} name={name} ownsRules={ownsRules} action={actionOf(r)} set={set} t={t} />
              ))}
            </RowGrid>
          )}
        </>
      )}
    </section>
  );
}

/** A folder Pi loads on its own; Skillshare only shows it. */
function FolderCard({ folder: f, t }: { folder: PiExtensionFolder; t: T }) {
  const empty = f.rows.length === 0 && !f.problem;
  const dir = sharedDir(f.rows.map((r) => r.path));
  return (
    <section className="ss-list !shadow-none" aria-label={shortenHome(f.path)}>
      <div className="ss-r !min-h-[60px]">
        <span className="ss-cat bg-sunken text-ink-2" aria-hidden><Folder size={16} /></span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex flex-wrap items-center gap-2">
            <span className="font-semibold">{t(f.kind === 'settings' ? 'targetDetail.piExtensions.folder.settings' : 'targetDetail.piExtensions.folder.title')}</span>
            {f.scope && <span className="ss-tag inf">{t(f.scope === 'global' ? 'targetDetail.piExtensions.shape.global' : 'targetDetail.piExtensions.shape.projectOnly')}</span>}
          </span>
          <span className="truncate font-mono text-[12px] text-ink-3" title={f.path}>{shortenHome(f.path)}</span>
        </span>
        {empty
          ? <span className="shrink-0 text-[13px] text-ink-3">{t('targetDetail.piExtensions.folder.empty')}</span>
          : <span className="ss-tag shrink-0"><Lock size={11} />{t('targetDetail.piExtensions.readOnlyTag')}</span>}
      </div>
      {!empty && <div className="ss-r !min-h-0 bg-sunken text-[13px] text-ink-2"><Info size={14} className="shrink-0 text-ink-3" />{t(f.kind === 'settings' ? 'targetDetail.piExtensions.folder.settingsHint' : 'targetDetail.piExtensions.folder.hint')}</div>}
      {f.problem && <div className="ss-r !min-h-0 text-[13px] text-ink-2">{t(`targetDetail.piExtensions.problem.${f.problem}`)}</div>}
      {f.rows.length > 0 && (
        <RowGrid dir={dir}>
          {f.rows.map((r) => (
            <li key={r.path} className="flex min-h-[46px] min-w-0 items-center gap-2.5 px-2 py-1.5">
              <StatusDot value={r.selection} t={t} />
              <span className="flex min-w-0 flex-1 flex-col">
                <RowPath path={r.path} dir={dir} missing={r.file === 'missing'} t={t} />
                <span className="text-[12px] text-ink-3">{r.provenance === 'extras' ? t('targetDetail.piExtensions.folder.extra', { name: r.extra ?? '' }) : t('targetDetail.piExtensions.folder.native')}</span>
              </span>
              <OddSelection value={r.selection} t={t} />
            </li>
          ))}
        </RowGrid>
      )}
    </section>
  );
}

/** Extensions as a grid of cells, their shared folder named once above them. */
function RowGrid({ dir, children }: { dir: string; children: ReactNode }) {
  return (
    <div className="ss-r !block !min-h-0 !p-0">
      {dir && <div className="px-4 pt-3 font-mono text-[12px] text-ink-3">{dir}</div>}
      <ul className="grid grid-cols-[repeat(auto-fill,minmax(440px,1fr))] gap-x-4 px-2 pb-2 pt-1">{children}</ul>
    </div>
  );
}

function ExtensionCell({ pkg, row, dir, name, ownsRules, action, set, t }: { pkg: PiExtensionPackage; row: PiExtensionRow; dir: string; name: string; ownsRules: boolean; action?: PiExtensionAction; set: SetAction; t: T }) {
  const label = t('targetDetail.piExtensions.switchLabel', { path: row.path, pkg: pkg.identity || pkg.source, name });
  const change = (next?: PiExtensionAction) => set(pkg.scope, pkg.index, row.path, next);
  // When removing the row's own rule gives the other state, the switch removes it, so no rule is left behind.
  const dropsRule = Boolean(row.unruled) && row.unruled !== row.selection;
  const canRemove = ownsRules && row.editable && !dropsRule && /^[+-]/.test(row.rule ?? '') && (row.origin === 'rule' || row.origin === 'project');
  const switchable = row.editable && row.file === 'present' && (action !== 'default' || dropsRule);
  // The package default needs no note on every row; anything else says where the selection comes from.
  const showOrigin = Boolean(action) || row.origin !== 'default' || row.selection === 'unknown' || canRemove;
  return (
    <li className={`flex min-h-[46px] min-w-0 items-center gap-2.5 rounded-[var(--r-ctl)] px-2 py-1.5 ${action ? 'bg-link-bg' : ''}`}>
      {switchable
        ? <RowSwitch on={isOn(row, action)} label={label} onToggle={() => change(action ? undefined : dropsRule ? 'default' : row.selection === 'loads' ? 'exclude' : 'select')} />
        : <StatusDot value={row.selection} t={t} />}
      <span className="flex min-w-0 flex-1 flex-col">
        <RowPath path={row.path} dir={dir} missing={row.file === 'missing'} t={t} />
        {showOrigin && (
          <span className="flex flex-wrap items-center gap-x-2 text-[12px] text-ink-3">
            <RowOrigin row={row} action={action} canRemove={canRemove} onDefault={() => change('default')} onKeep={dropsRule ? undefined : () => change()} t={t} />
          </span>
        )}
      </span>
      {action ? <PendingSelection from={row.selection} action={action} unruled={row.unruled} t={t} /> : !switchable && <OddSelection value={row.selection} t={t} />}
    </li>
  );
}

function RowSwitch({ on, label, onToggle }: { on: boolean; label: string; onToggle: () => void }) {
  return (
    <button type="button" role="switch" aria-checked={on} aria-label={label} className="grid h-8 w-[38px] shrink-0 place-items-center" onClick={onToggle}>
      <span className={`ss-sw ${on ? 'on' : ''}`}><i /></span>
    </button>
  );
}

/** A row without a switch shows on or off as a dot; its name is for screen readers. */
function StatusDot({ value, t }: { value: PiSelection; t: T }) {
  if (value !== 'loads' && value !== 'skipped') return <span className="w-[38px] shrink-0" aria-hidden />;
  return <span className={`ss-st ${selTone[value]} w-[38px] shrink-0 justify-center`}><span className="sr-only">{t(`targetDetail.piExtensions.sel.${value}`)}</span></span>;
}

/** The file's name under its shared folder; a file the settings name but the package lacks is marked. */
function RowPath({ path, dir, missing, t }: { path: string; dir: string; missing: boolean; t: T }) {
  return (
    <span className="flex min-w-0 items-center gap-2">
      <span className={`truncate font-mono text-[13px] font-semibold ${missing ? 'text-ink-3 line-through' : ''}`} title={path}>{path.slice(dir.length)}</span>
      {missing && <MissingBadge t={t} />}
    </span>
  );
}

function MissingBadge({ t }: { t: T }) {
  return <span className="ss-st bad shrink-0">{t('targetDetail.piExtensions.file.missing')}</span>;
}

/** The selection a pending change leads to. */
function PendingSelection({ from, action, unruled, t }: { from: PiSelection; action: PiExtensionAction; unruled?: PiSelection; t: T }) {
  // Without its rule the file follows the rules that remain; unless the view knows that, the preview computes it.
  const to = action === 'default' ? unruled ?? 'afterReview' : action === 'select' ? 'loads' : 'skipped';
  return <span className="shrink-0 text-[13px] font-semibold text-link">{t(`targetDetail.piExtensions.sel.${from}`)} → {t(`targetDetail.piExtensions.sel.${to}`)}</span>;
}

/** Where the row's selection comes from, and the button that removes or keeps its exact rule. */
function RowOrigin({ row, action, canRemove, onDefault, onKeep, t }: { row: PiExtensionRow; action?: PiExtensionAction; canRemove: boolean; onDefault: () => void; onKeep?: () => void; t: T }) {
  if (action === 'default') {
    return (
      <>
        <span>{t('targetDetail.piExtensions.origin.pendingDefault')}</span>
        {onKeep && <button type="button" className="font-semibold text-ink-2 hover:text-ink" onClick={onKeep}>{t('targetDetail.piExtensions.keepRule')}</button>}
      </>
    );
  }
  if (action) return <span>{t('targetDetail.piExtensions.origin.pendingRule', { rule: `${action === 'select' ? '+' : '-'}${row.path}` })}</span>;
  // Skillshare can't say what an undecidable rule selects; it says why and where to change it.
  if (row.selection === 'unknown') {
    return <span>{row.globs?.length ? t('targetDetail.piExtensions.unknown.glob', { globs: row.globs.join(' ') }) : t('targetDetail.piExtensions.unknown.rule')}</span>;
  }
  return (
    <>
      <span>{t(`targetDetail.piExtensions.origin.${row.origin}`, { rule: row.rule ?? '' })}</span>
      {row.globs && row.globs.length > 0 && <span className="font-mono">{row.globs.join(' ')}</span>}
      {canRemove && (
        <button type="button" className="inline-flex items-center gap-1 font-semibold text-ink-2 hover:text-ink" onClick={onDefault}>
          <RotateCcw size={12} />{t('targetDetail.piExtensions.removeRule')}
        </button>
      )}
    </>
  );
}

/** Only a selection a dot can't show (can't tell, nothing to load) is written out. */
function OddSelection({ value, t }: { value: PiSelection; t: T }) {
  if (value === 'loads' || value === 'skipped') return null;
  return <span className={`ss-st ${selTone[value]} shrink-0`}>{t(`targetDetail.piExtensions.sel.${value}`)}</span>;
}

/** Previews the pending changes, then applies exactly that preview's revision. */
function ReviewDialog({ name, view, changes, onClose, onApplied, t }: {
  name: string; view: PiExtensionsView; changes: PiExtensionChange[]; onClose: () => void; onApplied: (plan: PiExtensionsPlan) => void; t: T;
}) {
  const queryClient = useQueryClient();
  const preview = useQuery({ queryKey: ['pi-extensions-preview', name, changes], queryFn: () => piExtensionsApi.preview(name, changes), retry: false, gcTime: 0, staleTime: Infinity });
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState<ApiError | Error | null>(null);
  const plan = preview.data;
  const project = view.scope === 'project';
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.piExtensions(name) });
  const apply = async () => {
    if (!plan) return;
    setApplying(true);
    setError(null);
    try {
      const done = await piExtensionsApi.apply(name, changes, plan.revision);
      await Promise.all([refresh(), queryClient.invalidateQueries({ queryKey: queryKeys.plugins })]);
      onApplied(done);
    } catch (err) {
      setError(err as Error);
      setApplying(false);
    }
  };
  const failure = error ?? preview.error;
  const code = failure instanceof ApiError ? failure.code : undefined;
  const stale = code === 'pi_extensions_stale';
  const title = t(project ? 'targetDetail.piExtensions.dialog.titleProject' : 'targetDetail.piExtensions.dialog.title', { name });
  const pkgName = (scope: Scope | undefined, index: number, identity?: string) => {
    const p = view.packages.find((x) => x.scope === (scope ?? 'global') && x.index === index);
    return identity || p?.identity || p?.source || String(index);
  };
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={applying} ariaLabel={title} className="!max-w-[680px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t(project ? 'targetDetail.piExtensions.dialog.subtitleProject' : 'targetDetail.piExtensions.dialog.subtitle', { path: shortenHome(view.settingsPath) })}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={applying}><X size={16} /></button>
      </div>
      <div className="db !gap-3 text-[13.5px]">
        {!plan && !failure && <p className="text-ink-3">{t('targetDetail.piExtensions.dialog.loading')}</p>}
        {plan && (
          <>
            <ul className="flex flex-col gap-1">
              {plan.rows.map((r) => (
                <li key={keyOf(r.scope ?? 'global', r.index, r.path)} className="flex flex-wrap items-center gap-x-2">
                  <span className="font-mono text-[13px] font-semibold">{r.path}</span>
                  <span className="text-[12px] text-ink-3">{pkgName(r.scope, r.index)}</span>
                  <span className="flex-1" />
                  <span>{t(`targetDetail.piExtensions.sel.${r.before}`)} → <span className="font-semibold">{t(`targetDetail.piExtensions.sel.${r.after}`)}</span></span>
                </li>
              ))}
            </ul>
            {plan.entries.map((e) => (
              <EntryDiff key={`${e.scope ?? 'global'}:${e.index}`} entry={e} pkg={pkgName(e.scope, e.index, e.identity)} t={t} />
            ))}
            <p className="text-[12.5px] text-ink-2">{t('targetDetail.piExtensions.dialog.kept')}</p>
            {project && <p className="text-[12.5px] text-ink-2">{t('targetDetail.piExtensions.dialog.mayNotLoad')}</p>}
            <p className="text-[12.5px] text-ink-3">{t('targetDetail.piExtensions.dialog.revision', { revision: plan.revision.slice(0, 7) })}</p>
          </>
        )}
        {failure && <ReviewFailure failure={failure} code={code} t={t} />}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={applying}>{t('common.cancel')}</Button>
        {stale
          ? <Button variant="secondary" onClick={() => { void refresh(); onClose(); }}>{t('targetDetail.piExtensions.reviewAgain')}</Button>
          : <Button variant="primary" onClick={apply} loading={applying} disabled={!plan || Boolean(preview.error)}>{t(project ? 'targetDetail.piExtensions.dialog.applyProject' : 'targetDetail.piExtensions.dialog.apply')}</Button>}
      </div>
    </DialogShell>
  );
}

/** One settings entry the apply writes: its extensions list before and after. */
function EntryDiff({ entry: e, pkg, t }: { entry: PiExtensionsPlan['entries'][number]; pkg: string; t: T }) {
  const list = (rules?: string[] | null) => (rules ? JSON.stringify(rules) : t('targetDetail.piExtensions.dialog.noList'));
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-[12.5px] text-ink-2">{t('targetDetail.piExtensions.dialog.entry', { pkg })}</span>
      <div role="region" aria-label={t('targetDetail.piExtensions.dialog.entry', { pkg })} className="rounded-lg border border-line py-1.5 font-mono text-[12px] leading-[1.7]">
        {!e.created && <div className="bg-diff-del-bg px-3.5"><span className="text-diff-del">− </span>"extensions": {list(e.before)}</div>}
        {!e.removed && <div className="bg-diff-add-bg px-3.5"><span className="text-diff-add">+ </span>"extensions": {list(e.after)}</div>}
      </div>
      {e.created && <span className="text-[12.5px] text-ink-2">{t('targetDetail.piExtensions.dialog.created', { pkg, reference: e.reference ?? '' })}</span>}
      {e.removed && <span className="text-[12.5px] text-ink-2">{t('targetDetail.piExtensions.dialog.removed')}</span>}
      {e.converted && <span className="text-[12.5px] text-ink-3">{t('targetDetail.piExtensions.dialog.converted')}</span>}
      {!e.created && e.keptKeys.length > 0 && <span className="text-[12.5px] text-ink-3">{t('targetDetail.piExtensions.dialog.keptKeys', { keys: e.keptKeys.join(', ') })}</span>}
    </div>
  );
}

function ReviewFailure({ failure, code, t }: { failure: Error; code?: string; t: T }) {
  const stale = code === 'pi_extensions_stale';
  const busy = code === 'pi_extensions_busy';
  return (
    <div className={`ss-note ${stale || busy ? 'warn' : 'bad'}`} role="alert">
      <AlertCircle size={16} />
      <span className="flex-1">{stale ? t('targetDetail.piExtensions.stale') : busy ? t('targetDetail.piExtensions.busy') : failure.message}</span>
    </div>
  );
}
