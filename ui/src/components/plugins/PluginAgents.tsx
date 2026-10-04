import { Link } from 'react-router-dom';
import { RefreshCw } from 'lucide-react';
import { targetMap, type PluginInventory } from '../../api/plugins';
import { isPiTarget } from '../../api/piExtensions';
import IconButton from '../IconButton';
import { RailGroup, RailRow, RailSection } from '../StatusRail';
import PluginDocsLink from './PluginDocsLink';
import { useT } from '../../i18n';
import { useSlow } from '../../hooks/useSlow';
import { useSyncedTargetsQuery } from '../../hooks/useSharedQueries';

interface Props {
  inventory: PluginInventory;
  /** False while the Agents' CLIs are still being asked: their names are known, their state is not. */
  ready: boolean;
  refreshing: boolean;
  disabled: boolean;
  onRefresh: () => void;
}

/** Agents grouped by what you would do about them: use it, open it, leave it, or install its CLI. */
export default function PluginAgents({ inventory, ready, refreshing, disabled, onRefresh }: Props) {
  const t = useT();
  // One Agent's CLI can hang on its own network call; say so instead of spinning in silence.
  const slow = useSlow(!ready, 10000);
  const definitions = inventory.targetDefinitions ?? [];
  const labels = targetMap(definitions);
  const label = (target: string) => labels[target]?.label ?? target;
  // The backend keys its fixed sentences; a message it assembled at runtime has no key
  // and is shown as it came, which is also what the CLI prints.
  const message = (key: string | undefined, text: string | undefined, args?: Record<string, string>) => (key ? t(key, args, text) : text ?? '');
  // Native registrations need not be managed by Skillshare; extension selection stays on each Pi target.
  const piTargets = new Set((useSyncedTargetsQuery().data?.targets ?? []).filter(isPiTarget).map((x) => x.name));
  const manual = inventory.hosts.filter((h) => labels[h.target]?.operations.length === 0);
  const byStatus = (status: string) => inventory.hosts.filter((h) => h.status === status && !manual.includes(h));
  const reasoned = (hosts: PluginInventory['hosts']) => hosts.map((h) => (
    <RailRow key={h.target} target={h.target} label={label(h.target)} dim sub={message(h.errorKey, h.error, h.errorArgs)}
      detail={<><span>{message(h.errorKey, h.error, h.errorArgs)}</span><PluginDocsLink target={h.target} label={label(h.target)} /></>} />
  ));

  return (
    <RailSection title={t('plugins.hostsTitle')} count={definitions.length} action={<IconButton icon={<RefreshCw size={15} className={refreshing ? 'animate-spin' : ''} />} label={t('plugins.refresh')} disabled={disabled || refreshing} onClick={onRefresh} />}>
      <p className="text-xs leading-normal text-ink-3">{t('plugins.hostsHelp')}</p>
      {!ready ? (
        <RailGroup label={t('plugins.hostsAsking')} foot={slow ? t('plugins.hostsSlow') : undefined}>
          {definitions.map((d) => <RailRow key={d.target} target={d.target} label={d.label} dim right={<span className="ss-skel w-11 shrink-0" />} />)}
        </RailGroup>
      ) : (
        <>
          {byStatus('ready').length > 0 && (
            <RailGroup label={t('plugins.hostReady')} count={byStatus('ready').length}>
              {byStatus('ready').map((h) => (
                <RailRow key={h.target} target={h.target} label={label(h.target)}
                  right={h.installed.length > 0 && <span className="shrink-0 text-xs text-ink-3">{t(h.installed.length === 1 ? 'plugins.hostRegistered.one' : 'plugins.hostRegistered.other', { count: h.installed.length })}</span>}
                  detail={<>
                    <span>{h.target === 'grok' ? t('plugins.reason.grok') : message(h.noteKey || 'plugins.note.native', h.note)}</span>
                    {h.version && <span className="break-all font-mono text-xs text-ink-3">{h.version}</span>}
                    {piTargets.has(h.target) && <Link to={`/targets/${encodeURIComponent(h.target)}?tab=extensions`} className="text-xs font-semibold text-ink-2 hover:text-ink">{t('plugins.piExtensions', { name: label(h.target) })}</Link>}
                    <PluginDocsLink target={h.target} label={label(h.target)} />
                  </>} />
              ))}
            </RailGroup>
          )}
          {byStatus('blocked').length > 0 && <RailGroup label={t('plugins.hostBlocked')} count={byStatus('blocked').length}>{reasoned(byStatus('blocked'))}</RailGroup>}
          {manual.length > 0 && <RailGroup label={t('plugins.hostManual')} count={manual.length} foot={t('plugins.hostManualHelp')}>{reasoned(manual)}</RailGroup>}
          {/* One cause shared by every Agent here, so it is stated once instead of per row. */}
          {byStatus('missing').length > 0 && (
            <RailGroup label={t('plugins.hostMissing')} count={byStatus('missing').length} foot={t('plugins.hostMissingHelp')}>
              {byStatus('missing').map((h) => h.errorArgs
                ? reasoned([h])[0]
                : <RailRow key={h.target} target={h.target} label={label(h.target)} dim right={<PluginDocsLink target={h.target} label={label(h.target)} />} />)}
            </RailGroup>
          )}
        </>
      )}
    </RailSection>
  );
}
