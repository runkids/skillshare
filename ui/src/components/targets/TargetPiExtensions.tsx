import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ArrowRight, ChevronDown, CircleCheck, Info, Puzzle, RotateCcw, X } from 'lucide-react';
import { ApiError } from '../../api/client';
import { piExtensionsApi, type PiExtensionAction, type PiExtensionChange, type PiExtensionPackage, type PiExtensionRow, type PiExtensionsPlan, type PiExtensionsView, type PiSelection } from '../../api/piExtensions';
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

      {rows === 0 && view.packages.length === 0 ? (
        <EmptyState icon={Puzzle} title={t('targetDetail.piExtensions.emptyTitle')} description={t('targetDetail.piExtensions.emptyDescription', { name })} action={<Link to="/plugins" className="ss-btn">{t('targetDetail.piExtensions.openPlugins')}<ArrowRight size={14} /></Link>} />
      ) : (
        <div className="ss-list !shadow-none">
          <div className="ss-lh">
            <span className="flex-1">{t('targetDetail.piExtensions.col.extension')}</span>
            <span className="w-[170px]">{t('targetDetail.piExtensions.col.configured')}</span>
          </div>
          {view.packages.map((p) => (
            <PackageRows key={`${p.scope}:${p.index}`} pkg={p} name={name} ownsRules={!project || p.scope === 'project'} pending={pending} set={set} t={t} />
          ))}
          {view.folders.map((f) => (
            <div key={`${f.scope ?? ''}:${f.path}`} className="contents">
              <div className="ss-gh">
                <span className="font-semibold">{t(f.kind === 'settings' ? 'targetDetail.piExtensions.folder.settings' : 'targetDetail.piExtensions.folder.title')}</span>
                {f.scope && <span className="ss-tag inf">{t(f.scope === 'global' ? 'targetDetail.piExtensions.shape.global' : 'targetDetail.piExtensions.shape.projectOnly')}</span>}
                <span className="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-3" title={f.path}>{shortenHome(f.path)}</span>
                <span className="ss-tag">{t('targetDetail.piExtensions.readOnlyTag')}</span>
              </div>
              <div className="ss-r !min-h-0 text-[13px] text-ink-2"><Info size={14} className="shrink-0 text-ink-3" />{t(f.kind === 'settings' ? 'targetDetail.piExtensions.folder.settingsHint' : 'targetDetail.piExtensions.folder.hint')}</div>
              {f.problem && <div className="ss-r !min-h-0 text-[13px] text-ink-2">{t(`targetDetail.piExtensions.problem.${f.problem}`)}</div>}
              {f.rows.length === 0 && !f.problem && <div className="ss-r !min-h-[42px] text-[13px] text-ink-3">{t('targetDetail.piExtensions.folder.empty')}</div>}
              {f.rows.map((r) => (
                <div key={r.path} className="ss-r !min-h-[42px]">
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="flex min-w-0 items-center gap-2">
                      <span className="truncate font-mono text-[13px] font-semibold" title={r.path}>{r.path}</span>
                      {r.file === 'missing' && <MissingBadge t={t} />}
                    </span>
                    <span className="text-[12px] text-ink-3">{r.provenance === 'extras' ? t('targetDetail.piExtensions.folder.extra', { name: r.extra ?? '' }) : t('targetDetail.piExtensions.folder.native')}</span>
                  </span>
                  <Selection value={r.selection} t={t} />
                </div>
              ))}
            </div>
          ))}
        </div>
      )}
      <p className="text-[13px] text-ink-3">
        {t('targetDetail.piExtensions.pluginsHint')} <Link to="/plugins" className="font-semibold text-ink-2 hover:text-ink">{t('targetDetail.piExtensions.openPlugins')}</Link>
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
  const values = { name: view.target, version: view.version || '?', verified: view.verifiedVersions.join(', ') };
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

/** ownsRules: the rules shown are in the file this view writes, so one can be removed. */
function PackageRows({ pkg, name, ownsRules, pending, set, t }: { pkg: PiExtensionPackage; name: string; ownsRules: boolean; pending: Record<string, PiExtensionAction>; set: SetAction; t: T }) {
  const [open, setOpen] = useState(false);
  const title = pkg.identity || pkg.source;
  const list = (rules: string[] | null | undefined) => (rules ? rules.join(', ') || '[]' : t('targetDetail.piExtensions.details.none'));
  return (
    <>
      <div className="ss-gh">
        <span className="ss-cat plugin !h-[26px] !w-[26px]" aria-hidden><Puzzle size={14} /></span>
        <span className="min-w-0 truncate font-mono text-[13px] font-semibold" title={pkg.source}>{title}</span>
        {pkg.kind && <span className="ss-tag">{pkg.kind}</span>}
        {pkg.shape && <span className="ss-tag inf">{t(`targetDetail.piExtensions.shape.${pkg.shape}`)}</span>}
        {pkg.managedBy && <Link to="/plugins" className="ss-tag hover:text-ink">{t('targetDetail.piExtensions.managedBy', { name: pkg.managedBy })}</Link>}
        <span className="flex-1" />
        <button type="button" className="inline-flex items-center gap-1 text-[12px] font-semibold text-ink-2 hover:text-ink" aria-expanded={open} onClick={() => setOpen(!open)}>
          {t('targetDetail.piExtensions.details.show')}<ChevronDown size={12} className={open ? 'rotate-180' : ''} />
        </button>
      </div>
      {open && (
        <div role="region" aria-label={t('targetDetail.piExtensions.details.label', { pkg: title })} className="ss-r !min-h-0 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-[12.5px]">
          <span className="text-ink-3">{t('targetDetail.piExtensions.details.source')}</span><span className="break-all font-mono">{pkg.source}</span>
          <span className="text-ink-3">{t('targetDetail.piExtensions.details.rules')}</span><span className="break-all font-mono">{list(pkg.rules)}</span>
          {pkg.globalRules && <><span className="text-ink-3">{t('targetDetail.piExtensions.details.globalRules')}</span><span className="break-all font-mono">{list(pkg.globalRules)}</span></>}
          <span className="text-ink-3">{t('targetDetail.piExtensions.details.otherKeys')}</span><span className="font-mono">{pkg.otherKeys.join(', ') || t('targetDetail.piExtensions.details.none')}</span>
        </div>
      )}
      {pkg.problem && <div className="ss-r !min-h-0 text-[13px] text-ink-2"><AlertCircle size={14} className="shrink-0 text-warn" />{t(`targetDetail.piExtensions.problem.${pkg.problem}`)}</div>}
      {pkg.readOnly && <div className="ss-r !min-h-0 text-[13px] text-ink-2"><Info size={14} className="shrink-0 text-ink-3" />{t(`targetDetail.piExtensions.packageReadOnly.${pkg.readOnly}`)}</div>}
      {pkg.rows.length === 0 && !pkg.problem && <div className="ss-r !min-h-[42px] text-[13px] text-ink-3">{t('targetDetail.piExtensions.noExtensions')}</div>}
      {pkg.rows.map((r) => (
        <ExtensionRow key={r.path} pkg={pkg} row={r} name={name} ownsRules={ownsRules} action={pending[keyOf(pkg.scope, pkg.index, r.path)]} set={set} t={t} />
      ))}
    </>
  );
}

function ExtensionRow({ pkg, row, name, ownsRules, action, set, t }: { pkg: PiExtensionPackage; row: PiExtensionRow; name: string; ownsRules: boolean; action?: PiExtensionAction; set: SetAction; t: T }) {
  const on = action === 'select' || (!action && row.selection === 'loads');
  const label = t('targetDetail.piExtensions.switchLabel', { path: row.path, pkg: pkg.identity || pkg.source, name });
  const change = (next?: PiExtensionAction) => set(pkg.scope, pkg.index, row.path, next);
  return (
    <div className={`ss-r !min-h-[46px] ${action ? 'sel' : ''}`}>
      <RowSwitch shown={row.editable && row.file === 'present' && action !== 'default'} on={on} label={label} onToggle={() => change(action ? undefined : row.selection === 'loads' ? 'exclude' : 'select')} />
      <span className="flex min-w-0 flex-1 flex-col">
        <RowPath row={row} t={t} />
        <span className="flex flex-wrap items-center gap-x-2 text-[12px] text-ink-3">
          <RowOrigin row={row} action={action} canRemove={ownsRules && row.editable && Boolean(row.rule) && (row.origin === 'rule' || row.origin === 'project')} onDefault={() => change('default')} onKeep={() => change()} t={t} />
        </span>
      </span>
      {action ? <PendingSelection from={row.selection} action={action} t={t} /> : <Selection value={row.selection} t={t} />}
    </div>
  );
}

function RowSwitch({ shown, on, label, onToggle }: { shown: boolean; on: boolean; label: string; onToggle: () => void }) {
  if (!shown) return <span className="w-[38px] shrink-0" aria-hidden />;
  return (
    <button type="button" role="switch" aria-checked={on} aria-label={label} className="grid h-7 shrink-0 place-items-center" onClick={onToggle}>
      <span className={`ss-sw ${on ? 'on' : ''}`}><i /></span>
    </button>
  );
}

/** The file's path; a file the settings name but the package lacks is marked. */
function RowPath({ row, t }: { row: PiExtensionRow; t: T }) {
  const missing = row.file === 'missing';
  return (
    <span className="flex min-w-0 items-center gap-2">
      <span className={`truncate font-mono text-[13px] font-semibold ${missing ? 'text-ink-3 line-through' : ''}`} title={row.path}>{row.path}</span>
      {missing && <MissingBadge t={t} />}
    </span>
  );
}

function MissingBadge({ t }: { t: T }) {
  return <span className="ss-st bad shrink-0">{t('targetDetail.piExtensions.file.missing')}</span>;
}

/** The selection a pending change leads to. */
function PendingSelection({ from, action, t }: { from: PiSelection; action: PiExtensionAction; t: T }) {
  // Without its rule the file follows the rules that remain; the preview computes that.
  const to = action === 'default' ? 'afterReview' : action === 'select' ? 'loads' : 'skipped';
  return <span className="w-[170px] shrink-0 text-[13px] font-semibold text-link">{t(`targetDetail.piExtensions.sel.${from}`)} → {t(`targetDetail.piExtensions.sel.${to}`)}</span>;
}

/** Where the row's selection comes from, and the button that removes or keeps its exact rule. */
function RowOrigin({ row, action, canRemove, onDefault, onKeep, t }: { row: PiExtensionRow; action?: PiExtensionAction; canRemove: boolean; onDefault: () => void; onKeep: () => void; t: T }) {
  if (action === 'default') {
    return (
      <>
        <span>{t('targetDetail.piExtensions.origin.pendingDefault')}</span>
        <button type="button" className="font-semibold text-ink-2 hover:text-ink" onClick={onKeep}>{t('targetDetail.piExtensions.keepRule')}</button>
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

function Selection({ value, t }: { value: PiSelection; t: T }) {
  return <span className="w-[170px] shrink-0"><span className={`ss-st ${selTone[value]}`}>{t(`targetDetail.piExtensions.sel.${value}`)}</span></span>;
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
