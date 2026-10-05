import { Fragment, useEffect, useState } from 'react';
import { pluginsApi, targetMap, type PluginBinding, type PluginDiscovery, type PluginPackage, type PluginRequest, type PluginTarget, type PluginTargetDefinition } from '../../api/plugins';
import { Check, ChevronDown, ChevronRight, Search, X } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import { agentReasons } from './agentReasons';
import SourceHint from './SourceHint';
import { pluginErrorMessage } from './pluginError';
import PluginDocsLink from './PluginDocsLink';
import Button from '../Button';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import Spinner from '../Spinner';
import { Input } from '../Input';
import { useT } from '../../i18n';
import { useAppContext } from '../../context/AppContext';
import { shortenPath } from '../../lib/paths';

interface Props {
  onClose: () => void;
  onPreview: (r: PluginRequest) => Promise<void>;
  initialSource?: string;
  initialName?: string;
  /** Targets ticked when they can take the plugin, such as the target page this was opened from. */
  initialTargets?: PluginTarget[];
  /** The page's Agents, for an npm: source, which has no discovery to list them. */
  definitions?: PluginTargetDefinition[];
  /** What the plugin is bound to already. With it the dialog adds Agents to that plugin: the source is known, so it opens on discovery's answer. */
  bound?: Partial<Record<PluginTarget, PluginBinding>>;
  /** What was recorded when the plugin was added, for a plugin that has no Agent yet. */
  recorded?: Pick<PluginPackage, 'sourceRef' | 'entry' | 'plugin'>;
}

// What pi.dev offers to copy, `pi install npm:<package>` or the page's own address, is an npm source.
function npmFromPi(value: string) {
  const command = value.match(/^\s*pi\s+install\s+(npm:\S+)\s*$/);
  const page = value.match(/^\s*https?:\/\/(?:www\.)?pi\.dev\/packages\/((?:@[^/\s?#]+\/)?[^/\s?#]+)/);
  if (command) return command[1];
  if (!page) return value;
  try { return `npm:${decodeURIComponent(page[1])}`; } catch { return value; }
}

// The name recorded when none is typed, as the server derives it (npmSpec): the package name
// without its version or @scope.
function npmDefaultName(source: string) {
  const spec = source.slice('npm:'.length).trim();
  const at = spec.indexOf('@', 1);
  const name = at > 0 ? spec.slice(0, at) : spec;
  return name.slice(name.lastIndexOf('/') + 1);
}

export default function PluginAddDialog({ onClose, onPreview, initialSource = '', initialName = '', initialTargets = [], definitions = [], bound, recorded }: Props) {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const known = [...Object.values(bound ?? {}), recorded];
  const extending = !!bound && !!initialSource;
  const [sourceRef, setSourceRef] = useState(known.find((b) => b?.sourceRef)?.sourceRef ?? '');
  const [entry, setEntry] = useState(known.find((b) => b?.entry)?.entry ?? '');
  const [advanced, setAdvanced] = useState(false);
  const [entryOpen, setEntryOpen] = useState(false);
  const [source, setSource] = useState(initialSource);
  const [discovery, setDiscovery] = useState<PluginDiscovery | null>(null);
  const [name, setName] = useState('');
  const [packageName, setPackageName] = useState(initialName);
  const [targets, setTargets] = useState<PluginTarget[]>([]);
  const [busy, setBusy] = useState(extending);
  const [error, setError] = useState('');
  // An npm: source is installed by Pi, which downloads it: there is nothing to discover first.
  const [npmSource, setNpmSource] = useState('');
  const npmTargets = definitions.filter((d) => d.npm && d.operations.includes('add') && (!isProjectMode || d.project));
  const pluginTargets = targetMap(discovery?.targetDefinitions);
  const selected = discovery?.candidates.find((c) => c.name === name);
  const reasons = agentReasons(selected, pluginTargets, isProjectMode, t);
  const usable = reasons.filter((r) => !r.reason);
  const blocked = reasons.filter((r) => r.reason);
  // A preselected target the chosen plugin can't use stays out of the request.
  const chosen = npmSource ? targets : targets.filter((target) => usable.some((u) => u.target === target));
  const submitSource = () => {
    const value = source.trim();
    if (!value.startsWith('npm:')) { void discover(); return; }
    setNpmSource(value); setError('');
    setTargets(initialTargets.filter((target) => npmTargets.some((d) => d.target === target)));
  };
  // `keep` is the second look at the same source, after an OpenCode entry was given: the choice made so far stays.
  const discover = async (keep = false) => {
    setBusy(true); setError('');
    try {
      const d = await pluginsApi.discover(source, sourceRef || undefined, entry || undefined);
      setDiscovery(d);
      const plugin = known.find((b) => b?.plugin)?.plugin;
      // A blocked row cannot be chosen, so it is never chosen for the user either.
      const open = d.candidates.filter((c) => !c.problem);
      if (!keep) { setName(open.find((c) => c.name === plugin)?.name ?? (open.length === 1 ? open[0].name : '')); setTargets(initialTargets); }
    } catch (e) { setError(pluginErrorMessage(e, t)); }
    finally { setBusy(false); }
  };
  // Asking for a click on a source that is already filled in would be a step with nothing to decide.
  useEffect(() => { if (extending) void discover(); }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const preview = async () => {
    setBusy(true); setError('');
    try {
      if (npmSource) await onPreview({ action: 'add', source: npmSource, name: packageName.trim() || undefined, targets: chosen });
      else await onPreview({ action: 'add', source: discovery!.source, sourceRef: discovery!.sourceRef || undefined, entry: entry || undefined, plugin: name, name: packageName.trim() || undefined, targets: chosen });
    }
    catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  };
  return (
    <DialogShell open onClose={onClose} preventClose={busy} ariaLabel={t(extending ? 'plugins.addAgents' : 'plugins.add')} maxWidth="2xl" padding="none">
      <div className="dh">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="ss-h2">{t(extending ? 'plugins.addAgents' : 'plugins.add')}</h2>
          {npmSource
            ? <p className="truncate font-mono text-xs text-ink-3" title={npmSource}>{npmSource}</p>
            : discovery
            ? <p className="truncate font-mono text-xs text-ink-3" title={discovery.source}>{shortenPath(discovery.source)}{discovery.commit && ` · ${discovery.commit.slice(0, 7)}`}</p>
            : <p className="text-[13px] text-ink-2">{extending && busy ? initialName : t('plugins.sourceHelp')}</p>}
        </div>
        <IconButton icon={<X size={16} />} label={t('common.close')} disabled={busy} onClick={onClose} />
      </div>
      <div className="db overflow-y-auto">
        {!discovery && extending && busy ? (
          <div className="flex flex-col gap-1.5">
            <p className="flex items-center gap-2 text-[13px] text-ink-2"><Spinner size="sm" />{t('plugins.discovering')}</p>
            <SourceHint active />
          </div>
        ) : npmSource ? (
          <>
            <p className="ss-note warn">{t('plugins.npmNotice')}</p>
            <Input label={t('resources.col.name')} value={packageName} placeholder={npmDefaultName(npmSource)} disabled={busy} onChange={(e) => setPackageName(e.target.value)} />
            <div className="ss-fld">
              <span className="text-[13px] font-semibold">{t('plugins.targets')}</span>
              {npmTargets.length === 0 ? <p className="text-[13px] text-ink-3">{t('plugins.npmNoTargets')}</p> : (
                <div className="grid grid-cols-3 gap-x-4 gap-y-3.5 pt-1">
                  {npmTargets.map((definition) => {
                    const on = targets.includes(definition.target);
                    return (
                      <button key={definition.target} type="button" role="checkbox" aria-checked={on} aria-label={definition.label} disabled={busy} className={`ss-tgl ${on ? 'on' : ''}`} onClick={() => setTargets((old) => (on ? old.filter((x) => x !== definition.target) : [...old, definition.target]))}>
                        <span className="ic"><AgentIcon target={definition.target} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                        {definition.label}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </>
        ) : !discovery ? (
          <form id="plugin-source" className="flex flex-col gap-3.5" onSubmit={(e) => { e.preventDefault(); if (source.trim()) submitSource(); }}>
            <Input label={t('plugins.source')} placeholder="owner/repo · npm:package" value={source} disabled={busy} autoFocus onChange={(e) => setSource(npmFromPi(e.target.value))} />
            <button type="button" className="ss-disc self-start" aria-expanded={advanced} onClick={() => setAdvanced(!advanced)}>
              {advanced ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
              {t('plugins.advanced')}
            </button>
            {advanced && <div className="ml-[22px]"><Input label={t('plugins.sourceRef')} placeholder="main" value={sourceRef} disabled={busy} onChange={(e) => setSourceRef(e.target.value)} /></div>}
            {(busy || error) && <SourceHint active={busy} />}
          </form>
        ) : (
          <>
            {discovery.warnings?.map((warning) => <p key={warning} className="ss-note warn">{warning}</p>)}
            <div className="ss-fld" role="radiogroup" aria-label={t('plugins.choose')}>
              <span className="text-[13px] font-semibold">{t('plugins.choose')}</span>
              <div className="flex flex-col gap-2">
                {discovery.candidates.map((c) => (
                  <button key={c.name} type="button" role="radio" aria-checked={name === c.name} disabled={busy || !!c.problem} className={`ss-pick text-left disabled:opacity-55 ${name === c.name ? 'on' : ''}`} onClick={() => { setName(c.name); setTargets(initialTargets); }}>
                    <span className={`ss-chk rad self-start mt-0.5 ${name === c.name ? 'on' : ''}`} />
                    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span className="flex items-center gap-2"><span className="font-mono font-semibold">{c.name}</span>{c.version && <span className="ss-tag">{c.version}</span>}</span>
                      {(c.problem || c.description) && <span className={`text-[13px] ${c.problem ? 'text-bad' : 'text-ink-2'}`}>{c.problemKey ? t(c.problemKey, c.problemArgs, c.problem) : c.problem || c.description}</span>}
                      {c.components.length > 0 && <span className="text-xs text-ink-3">{c.components.join(' · ')}</span>}
                    </span>
                    <span className="ss-stack" aria-hidden="true">{c.targets.map((target) => <span key={target} className="ss-at"><AgentIcon target={target} size={13} /></span>)}</span>
                  </button>
                ))}
              </div>
            </div>
            {selected && !extending && <Input label={t('resources.col.name')} value={packageName} placeholder={selected.name} disabled={busy} onChange={(e) => setPackageName(e.target.value)} />}
            {selected && !selected.problem && (
              <div className="ss-fld">
                <span className="flex items-baseline gap-2">
                  <span className="text-[13px] font-semibold">{t('plugins.targets')}</span>
                  <span className="text-xs text-ink-3">{t(reasons.length === 1 ? 'plugins.targetsUsable.one' : 'plugins.targetsUsable.other', { count: usable.length, total: reasons.length })}</span>
                </span>
                {/* A grid, not a wrap: every cell is the same height, so one long reason can no
                    longer set the height of a whole row and leave holes beside it. */}
                <div className="grid grid-cols-3 gap-x-4 gap-y-3.5 pt-1">
                  {usable.map(({ target, definition }) => {
                    // Already bound: shown ticked so the whole picture is here, locked because unticking belongs to the list.
                    const has = !!bound?.[target];
                    const on = has || targets.includes(target);
                    return (
                      <span key={target} className="flex min-w-0 flex-col gap-1"><button type="button" role="checkbox" aria-checked={on} aria-label={definition.label} disabled={busy || has} className={`ss-tgl ${on ? 'on' : ''}`} onClick={() => setTargets((old) => (on ? old.filter((x) => x !== target) : [...old, target]))}>
                        <span className="ic"><AgentIcon target={target} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                        {definition.label}
                      </button><span className="truncate text-xs text-ink-3">{selected.targetInfo?.[target]?.components.join(' · ')}</span></span>
                    );
                  })}
                </div>
                {blocked.length > 0 && (
                  <div className="mt-1.5 flex flex-col gap-1.5">
                    <span className="flex items-baseline gap-2"><span className="text-[13px] font-semibold text-ink-2">{t('plugins.targetsBlocked')}</span><span className="ss-cnt">{blocked.length}</span></span>
                    <div className="ss-list !shadow-none">
                      {blocked.map(({ target, definition, reason }) => (
                        <Fragment key={target}>
                          <div className="ss-r !min-h-9">
                            <span className="ss-at opacity-55"><AgentIcon target={target} size={15} /></span>
                            <span className="w-36 shrink-0 truncate text-[13px] text-ink-2">{definition.label}</span>
                            <span className="min-w-0 flex-1 text-xs text-ink-3">{reason}</span>
                            {target === 'opencode' && <button type="button" className="ss-more shrink-0 !text-xs" aria-expanded={entryOpen} onClick={() => setEntryOpen(!entryOpen)}>{t('plugins.entrySet')}</button>}
                            <PluginDocsLink target={target} label={definition.label} />
                          </div>
                          {/* Only OpenCode needs an entry, and only when it could not work one out, so it is asked here and not of every source. */}
                          {target === 'opencode' && entryOpen && (
                            <form className="ss-r fold !min-h-0 flex-col !items-stretch gap-1.5 !py-3" onSubmit={(e) => { e.preventDefault(); void discover(true); }}>
                              <div className="flex items-end gap-2">
                                <div className="min-w-0 flex-1"><Input label={t('plugins.entry')} placeholder="dist/index.js" value={entry} disabled={busy} autoFocus onChange={(e) => setEntry(e.target.value)} /></div>
                                <Button type="submit" variant="secondary" loading={busy}>{t('plugins.rediscover')}</Button>
                              </div>
                              <p className="text-xs text-ink-3">{t('plugins.entryHelp')}</p>
                            </form>
                          )}
                        </Fragment>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </>
        )}
        {error && <div role="alert" className="ss-note bad"><span className="flex-1">{error}</span></div>}
      </div>
      <div className="df">
        {(discovery || npmSource) && !extending && <Button variant="ghost" className="mr-auto" disabled={busy} onClick={() => { setDiscovery(null); setNpmSource(''); setError(''); setEntry(''); setEntryOpen(false); }}>{t('common.back')}</Button>}
        {/* Beside the button it explains, and only while it applies: nothing ticked adds the plugin without installing it. */}
        {selected && !selected.problem && !extending && !chosen.length && <span className="text-xs text-ink-3">{t('plugins.targetsOptional')}</span>}
        <Button variant="ghost" disabled={busy} onClick={onClose}>{t('common.cancel')}</Button>
        {npmSource
          ? <Button loading={busy} disabled={!chosen.length} onClick={() => void preview()}>{t('plugins.preview')}</Button>
          : discovery
          ? <Button loading={busy} disabled={!selected || !!selected.problem || (extending && !chosen.length)} onClick={() => void preview()}>{t('plugins.preview')}</Button>
          : <Button type="submit" form="plugin-source" loading={busy} disabled={!source.trim()}><Search size={15} />{t(source.trim().startsWith('npm:') ? 'plugins.npmContinue' : 'plugins.discover')}</Button>}
      </div>
    </DialogShell>
  );
}
