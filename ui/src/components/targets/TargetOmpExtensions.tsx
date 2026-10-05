import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { AlertCircle, ArrowRight, ChevronDown, CircleCheck, FileDiff, Folder, Info, Lock, Package, Puzzle, X } from 'lucide-react';
import { ApiError } from '../../api/client';
import { ompExtensionsApi, type OmpExtensionChange, type OmpExtensionRow, type OmpExtensionsPlan, type OmpExtensionsView } from '../../api/ompExtensions';
import Button from '../Button';
import DialogShell from '../DialogShell';
import EmptyState from '../EmptyState';
import Tooltip from '../Tooltip';
import { PageSkeleton } from '../Skeleton';
import { messagesByLocale, useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import { fileName, shortenHome } from '../../lib/paths';

type T = ReturnType<typeof useT>;
type Pending = Record<string, boolean>;

/**
 * The extension files an Oh My Pi target's settings select. A switch changes only what the
 * settings say for that one file, after a preview of exactly that change; nothing here is
 * executed, and omp loads a selected file at its next start without asking for trust.
 */
export default function TargetOmpExtensions({ name }: { name: string }) {
  const t = useT();
  const query = useQuery({ queryKey: queryKeys.ompExtensions(name), queryFn: () => ompExtensionsApi.get(name) });
  // Drafts live outside the view, so a refresh after a stale preview keeps them; rows the new
  // view no longer offers drop out when the changes are computed.
  const [pending, setPending] = useState<Pending>({});
  const [applied, setApplied] = useState('');
  if (query.error) return <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{query.error.message}</span></div>;
  if (!query.data) return <PageSkeleton />;
  return <ExtensionsView name={name} view={query.data} pending={pending} setPending={setPending} applied={applied} setApplied={setApplied} t={t} />;
}

const selTone: Record<OmpExtensionRow['selection'], string> = { selected: 'ok', disabled: 'off', shadowed: 'warn', unknown: 'off' };
const selOf = (on: boolean) => (on ? 'selected' : 'disabled');

// Localize Skillshare's fixed inventory guidance; keep unknown diagnostics verbatim.
const guidanceKeys = new Map(Object.entries(messagesByLocale.en)
  .filter(([key]) => key.startsWith('targetDetail.ompExtensions.guidance.') || key === 'targetDetail.ompExtensions.pluginReadOnly')
  .map(([key, text]) => [text, key]));
const guidance = (text: string, t: T) => {
  if (text === 'Managed by Hooks; review changes in the Hooks page.') return t('targetDetail.ompExtensions.guidance.hooksOwned');
  if (text === 'Hooks ownership record exists but the file has drifted; resolve it in Hooks before changing extension selection.') return t('targetDetail.ompExtensions.guidance.hooksDrift');
  const key = guidanceKeys.get(text);
  if (key) return t(key);
  // Only canonical guidance templates are translated; paths and unknown diagnostics stay verbatim.
  for (const [template, key] of guidanceKeys) {
    const marker = template.indexOf('{path}');
    if (marker < 0 || template.includes('{name}')) continue;
    const prefix = template.slice(0, marker), suffix = template.slice(marker + 6);
    if (text.startsWith(prefix) && text.endsWith(suffix)) return t(key, { path: text.slice(prefix.length, suffix ? -suffix.length : undefined) });
  }
  const unsupported = text.match(/^Unsupported (.+) (list|entry) in (.+)$/s);
  if (unsupported) return t(`targetDetail.ompExtensions.guidance.unsupported${unsupported[2] === 'list' ? 'List' : 'Entry'}`, { name: unsupported[1], path: unsupported[3] });
  return text;
};

function ExtensionsView({ name, view, pending, setPending, applied, setApplied, t }: { name: string; view: OmpExtensionsView; pending: Pending; setPending: (p: Pending) => void; applied: string; setApplied: (p: string) => void; t: T }) {
  const [reviewing, setReviewing] = useState(false);
  const switchable = (row: OmpExtensionRow) => !view.readOnly && row.selectable;
  // Only a draft that still differs from a switchable row is a change.
  const drafts = view.rows.filter((r) => switchable(r) && r.key in pending && pending[r.key] !== r.enabled);
  const changes: OmpExtensionChange[] = drafts.map((r) => ({ key: r.key, enabled: pending[r.key] }));
  const pendingLabel = t(changes.length === 1 ? 'targetDetail.ompExtensions.pending.one' : 'targetDetail.ompExtensions.pending.other', { count: changes.length });
  const flip = (row: OmpExtensionRow) => {
    setApplied('');
    const next = { ...pending };
    if (row.key in next) delete next[row.key];
    else next[row.key] = !row.enabled;
    setPending(next);
  };
  return (
    <div className="flex flex-col gap-5">
      {applied && <div className="ss-note ok" role="status"><CircleCheck size={16} /><span className="flex-1">{t('targetDetail.ompExtensions.applied', { path: shortenHome(applied) })}</span></div>}
      {view.readOnly && (
        <div className="ss-note warn">
          <Lock size={16} />
          <div className="min-w-0 flex-1">
            {t('targetDetail.ompExtensions.readOnly', { name })}
            {view.reasons.length > 0 && <ul className="mt-1 list-disc pl-4">{view.reasons.map((r) => <li key={r}>{guidance(r, t)}</li>)}</ul>}
          </div>
        </div>
      )}
      {view.warnings.map((w) => <div key={w} className="ss-note warn" role="alert"><AlertCircle size={16} /><span className="flex-1">{guidance(w, t)}</span></div>)}
      <p className="flex items-center gap-1.5 text-[13px] text-ink-2">
        <span>{t('targetDetail.ompExtensions.hint', { name, path: shortenHome(view.settingsPath) })}</span>
        <Tooltip content={<span className="flex flex-col gap-2"><span>{t('targetDetail.ompExtensions.switchHint')}</span><span>{t('targetDetail.ompExtensions.footer')}</span></span>}>
          <button type="button" className="ss-ib !h-6 !w-6 !text-inherit" aria-label={t('targetDetail.piExtensions.info.label')}><Info size={14} /></button>
        </Tooltip>
      </p>

      {view.rows.length === 0 ? (
        <EmptyState icon={Puzzle} title={t('targetDetail.ompExtensions.emptyTitle')} description={t('targetDetail.ompExtensions.emptyDescription', { name })} />
      ) : (
        <div className="flex flex-col gap-4" aria-label={t('targetDetail.ompExtensions.listLabel', { name })}>
          {extensionGroups(view.rows).map((group) => (
            <ExtensionGroup key={group.key} group={group} name={name} readOnly={view.readOnly} pending={pending} onFlip={flip} t={t} />
          ))}
        </div>
      )}

      {changes.length > 0 && (
        <div className="ss-bulk" role="toolbar" aria-label={pendingLabel}>
          <b>{pendingLabel}</b>
          <span className="text-ink-3 max-md:hidden">{t('targetDetail.ompExtensions.pending.nothingYet')}</span>
          <span className="dv" />
          <Button variant="secondary" size="sm" onClick={() => setReviewing(true)}><FileDiff size={15} />{t('targetDetail.ompExtensions.review')}</Button>
          <span className="dv" />
          <button type="button" className="ss-ib" aria-label={t('targetDetail.ompExtensions.discard')} title={t('targetDetail.ompExtensions.discard')} onClick={() => setPending({})}><X size={16} /></button>
        </div>
      )}
      {reviewing && (
        <ReviewDialog name={name} view={view} changes={changes} onClose={() => setReviewing(false)}
          onApplied={(plan) => { setReviewing(false); setPending({}); setApplied(plan.settingsPath); }} t={t} />
      )}
    </div>
  );
}

type ExtensionGroupData = { key: string; dir: string; rows: OmpExtensionRow[] };
const folderOf = (path: string) => path.slice(0, Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')) + 1);

// Group plugins by their inspected root; other sources stay grouped by filesystem folder.
function extensionGroups(rows: OmpExtensionRow[]): ExtensionGroupData[] {
  const groups = new Map<string, ExtensionGroupData>();
  for (const row of rows) {
    const dir = folderOf(row.path);
    const key = JSON.stringify([row.source, row.scope, row.pluginRoot || dir]);
    if (!groups.has(key)) groups.set(key, { key, dir, rows: [] });
    const group = groups.get(key)!;
    group.rows.push(row);
    if (!row.path.startsWith(group.dir)) {
      const rootDir = row.pluginRoot ? row.pluginRoot + (row.pluginRoot.includes('\\') ? '\\' : '/') : '';
      group.dir = group.rows.every((r) => r.path.startsWith(rootDir)) ? rootDir : '';
    }
  }
  return [...groups.values()];
}

function ExtensionGroup({ group, name, readOnly, pending, onFlip, t }: { group: ExtensionGroupData; name: string; readOnly: boolean; pending: Pending; onFlip: (row: OmpExtensionRow) => void; t: T }) {
  const [details, setDetails] = useState(false);
  const first = group.rows[0];
  const rootDir = first.pluginRoot ? first.pluginRoot + (first.pluginRoot.includes('\\') ? '\\' : '/') : '';
  const dirLabel = rootDir && group.dir.startsWith(rootDir) ? group.dir.slice(rootDir.length) : shortenHome(group.dir);
  const title = first.source === 'plugin'
    ? [t('plugins.title'), first.pluginName ? fileName(first.pluginName) : group.dir && fileName(group.dir.slice(0, -1))].filter(Boolean).join(' · ')
    : t(`targetDetail.ompExtensions.group.${first.source}`);
  const locked = readOnly || group.rows.every((r) => !r.selectable);
  const on = group.rows.filter((r) => (!readOnly && r.selectable ? pending[r.key] ?? r.enabled : r.enabled) === true).length;
  const Icon = first.source === 'plugin' ? Package : Folder;
  const notes = [...new Set(group.rows.flatMap((r) => [
    ...(r.owner === 'native' ? [t('targetDetail.ompExtensions.ownerNative')] : []),
    ...(r.owner !== 'hooks' ? r.notes : []),
  ].filter((n) => n !== r.readOnlyReason && (!r.pluginName || n !== `Installed plugin: ${r.pluginName}`))))];
  return (
    <section className="ss-list !shadow-none" aria-label={title}>
      <div className="ss-r !min-h-[60px]">
        <span className="ss-cat bg-sunken text-ink-2" aria-hidden><Icon size={first.source === 'plugin' ? 26 : 16} /></span>
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="nm m truncate" title={first.pluginName}>{title}</span>
          <span className="flex flex-wrap items-center gap-1.5">
            {first.source !== 'plugin' && <span className="ss-tag">{first.source}</span>}
            {first.pluginVersion && <span className="ss-tag">{first.pluginVersion}</span>}
            <span className="ss-tag inf">{first.scope}</span>
            {first.source === 'plugin' && <Link to="/plugins" className="ss-ib !h-6 !w-6" aria-label={t('targetDetail.piExtensions.openPlugins')} title={t('targetDetail.piExtensions.openPlugins')}><ArrowRight size={14} /></Link>}
          </span>
        </span>
        <span className="shrink-0 text-[13px] text-ink-2">{t('targetDetail.piExtensions.summary', { on, total: group.rows.length })}</span>
        {locked && <span className="ss-tag shrink-0"><Lock size={11} />{t('targetDetail.ompExtensions.readOnlyTag')}</span>}
        <button type="button" className="inline-flex shrink-0 items-center gap-1 text-[12px] font-semibold text-ink-2 hover:text-ink" aria-expanded={details} onClick={() => setDetails(!details)}>
          {t('targetDetail.piExtensions.details.show')}<ChevronDown size={12} className={details ? 'rotate-180' : ''} />
        </button>
      </div>
      {details && (
        <div role="region" aria-label={t('targetDetail.piExtensions.details.label', { pkg: title })} className="ss-r !min-h-0 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-[12.5px]">
          <span className="text-ink-3">{t('targetDetail.piExtensions.details.source')}</span><span className="break-all font-mono">{first.pluginRoot || group.dir}</span>
          {first.pluginName && <><span className="text-ink-3">{t('resources.col.name')}</span><span className="break-all font-mono">{first.pluginName}</span></>}
          <span className="text-ink-3">{t('targetDetail.ompExtensions.details.identities')}</span>
          <span className="break-all font-mono">{group.rows.map((r) => <span key={r.key} className="block">{r.path.slice(group.dir.length)} · {r.derivedId}</span>)}</span>
          <span className="text-ink-3">{t('targetDetail.ompExtensions.details.notes')}</span>
          <span className="break-words">{notes.length ? notes.map((n) => <span key={n} className="block">{guidance(n, t)}</span>) : t('targetDetail.piExtensions.details.none')}</span>
        </div>
      )}
      <div className="ss-r !block !min-h-0 !p-0">
        {dirLabel && <div className="break-all px-4 pt-3 font-mono text-[12px] text-ink-3">{dirLabel}</div>}
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,440px),1fr))] gap-x-4 px-2 pb-2 pt-1">
          {group.rows.map((r) => {
            const switchable = !readOnly && r.selectable;
            return <Row key={r.key} row={r} dir={group.dir} name={name} switchable={switchable} showLock={!locked && !r.selectable} draft={switchable && r.key in pending && pending[r.key] !== r.enabled ? pending[r.key] : undefined} onFlip={() => onFlip(r)} t={t} />;
          })}
        </ul>
      </div>
    </section>
  );
}

function Row({ row, dir, name, switchable, showLock, draft, onFlip, t }: { row: OmpExtensionRow; dir: string; name: string; switchable: boolean; showLock: boolean; draft?: boolean; onFlip: () => void; t: T }) {
  const hooksLink = row.owner === 'hooks' && row.hooksTarget ? `/targets/${encodeURIComponent(row.hooksTarget)}?tab=hooks` : '';
  const on = draft ?? row.enabled === true;
  return (
    <li className={`flex min-h-[46px] min-w-0 items-center gap-2.5 rounded-[var(--r-ctl)] px-2 py-1.5 ${draft !== undefined ? 'bg-link-bg' : ''}`}>
      {switchable
        ? <button type="button" role="switch" aria-checked={on} aria-label={t('targetDetail.ompExtensions.switchLabel', { path: row.path, name })} className="grid h-8 w-[38px] shrink-0 place-items-center" onClick={onFlip}><span className={`ss-sw ${on ? 'on' : ''}`}><i /></span></button>
        : <span className={`ss-st ${selTone[row.selection]} w-[38px] shrink-0 justify-center`} aria-hidden />}
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="flex min-w-0 flex-wrap items-center gap-2">
          <span title={row.path} className="truncate font-mono text-[13px] font-semibold">{row.path.slice(dir.length) || guidance(row.name, t)}</span>
          {hooksLink && <Link to={hooksLink} className="ss-tag hover:text-ink">{t('targetDetail.ompExtensions.managedInHooks')}</Link>}
          {showLock && <span className="ss-tag"><Lock size={11} />{t('targetDetail.ompExtensions.readOnlyTag')}</span>}
        </span>
        {!switchable && row.readOnlyReason && <span className="text-[12px] text-ink-3">{guidance(row.readOnlyReason, t)}</span>}
      </span>
      {draft !== undefined
        ? <span className="shrink-0 text-[13px] font-semibold text-link">{t(`targetDetail.ompExtensions.sel.${selOf(!draft)}`)} → {t(`targetDetail.ompExtensions.sel.${selOf(draft)}`)}</span>
        : !switchable && <span className="shrink-0 text-[13px] text-ink-2">{t(`targetDetail.ompExtensions.sel.${row.selection}`)}</span>}
    </li>
  );
}

/** Previews the drafts against the view's revision, then applies exactly that plan. */
function ReviewDialog({ name, view, changes, onClose, onApplied, t }: { name: string; view: OmpExtensionsView; changes: OmpExtensionChange[]; onClose: () => void; onApplied: (plan: OmpExtensionsPlan) => void; t: T }) {
  const queryClient = useQueryClient();
  const preview = useQuery({ queryKey: ['omp-extensions-preview', name, view.revision, changes], queryFn: () => ompExtensionsApi.preview(name, changes, view.revision), retry: false, gcTime: 0, staleTime: Infinity });
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const plan = preview.data;
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.ompExtensions(name) });
  const apply = async () => {
    if (!plan) return;
    setApplying(true);
    setError(null);
    try {
      const done = await ompExtensionsApi.apply(name, changes, plan.revision);
      await Promise.all([refresh(), queryClient.invalidateQueries({ queryKey: queryKeys.plugins })]);
      onApplied(done);
    } catch (err) {
      setError(err as Error);
      setApplying(false);
    }
  };
  const failure = error ?? preview.error;
  const code = failure instanceof ApiError ? failure.code : undefined;
  const stale = code === 'omp_extensions_stale';
  const title = t('targetDetail.ompExtensions.dialog.title', { name });
  const rowOf = (key: string) => view.rows.find((r) => r.key === key);
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={applying} ariaLabel={title} className="!max-w-[680px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t('targetDetail.ompExtensions.dialog.subtitle', { path: shortenHome(view.settingsPath) })}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={applying}><X size={16} /></button>
      </div>
      <div className="db !gap-3 text-[13.5px]">
        {!plan && !failure && <p className="text-ink-3">{t('targetDetail.ompExtensions.dialog.loading')}</p>}
        {plan && (
          <>
            <ul className="flex flex-col gap-1.5">
              {plan.rows.map((r) => (
                <li key={r.key} className="flex flex-wrap items-center gap-x-2">
                  <span className="flex min-w-0 flex-col">
                    <span className="font-mono text-[13px] font-semibold">{rowOf(r.key)?.name ?? r.derivedId}</span>
                    <span className="truncate font-mono text-[12px] text-ink-3">{rowOf(r.key)?.path ?? r.key}</span>
                  </span>
                  <span className="flex-1" />
                  <span>{t(`targetDetail.ompExtensions.sel.${selOf(r.before)}`)} → <span className="font-semibold">{t(`targetDetail.ompExtensions.sel.${selOf(r.after)}`)}</span></span>
                </li>
              ))}
            </ul>
            {plan.warnings.map((w) => <div key={w} className="ss-note warn" role="alert"><AlertCircle size={16} /><span className="flex-1">{guidance(w, t)}</span></div>)}
            <p className="text-[12.5px] text-ink-2">{t('targetDetail.ompExtensions.dialog.kept')}</p>
            <p className="text-[12.5px] text-ink-3">{t('targetDetail.ompExtensions.dialog.revision', { revision: plan.revision.slice(0, 7) })}</p>
          </>
        )}
        {failure && (
          <div className={`ss-note ${stale || code === 'omp_extensions_busy' ? 'warn' : 'bad'}`} role="alert">
            <AlertCircle size={16} />
            <span className="flex-1">{stale ? t('targetDetail.ompExtensions.stale') : code === 'omp_extensions_busy' ? t('targetDetail.ompExtensions.busy') : failure.message}</span>
          </div>
        )}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={applying}>{t('common.cancel')}</Button>
        {stale
          ? <Button variant="secondary" onClick={() => { void refresh(); onClose(); }}>{t('targetDetail.ompExtensions.reviewAgain')}</Button>
          : <Button variant="primary" onClick={apply} loading={applying} disabled={!plan || Boolean(preview.error)}>{t('targetDetail.ompExtensions.dialog.apply')}</Button>}
      </div>
    </DialogShell>
  );
}
