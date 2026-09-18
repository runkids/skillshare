import { useState } from 'react';
import { pluginsApi, pluginTargets, type PluginDiscovery, type PluginRequest, type PluginTarget } from '../../api/plugins';
import { Check, Search, X } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import { Input } from '../Input';
import { useT } from '../../i18n';
import { useAppContext } from '../../context/AppContext';

export default function PluginAddDialog({ onClose, onPreview, initialSource = '', initialName = '' }: { onClose: () => void; onPreview: (r: PluginRequest) => Promise<void>; initialSource?: string; initialName?: string }) {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const [source, setSource] = useState(initialSource);
  const [discovery, setDiscovery] = useState<PluginDiscovery | null>(null);
  const [name, setName] = useState('');
  const [packageName, setPackageName] = useState(initialName);
  const [targets, setTargets] = useState<PluginTarget[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const selected = discovery?.candidates.find((c) => c.name === name);
  const discover = async () => {
    setBusy(true); setError('');
    try {
      const d = await pluginsApi.discover(source);
      setDiscovery(d); setName(d.candidates.length === 1 ? d.candidates[0].name : ''); setTargets([]);
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  };
  const preview = async () => {
    setBusy(true); setError('');
    try { await onPreview({ action: 'add', source: discovery!.source, plugin: name, name: packageName.trim() || undefined, targets }); }
    catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  };
  return (
    <DialogShell open onClose={onClose} preventClose={busy} ariaLabel={t('plugins.add')} maxWidth="2xl" padding="none">
      <div className="dh">
        <div className="flex flex-col gap-1"><h2 className="ss-h2">{t('plugins.add')}</h2><p className="text-[13px] text-ink-2">{t('plugins.sourceHelp')}</p></div>
        <IconButton icon={<X size={16} />} label={t('common.close')} disabled={busy} onClick={onClose} />
      </div>
      <div className="db overflow-y-auto">
        {!discovery ? (
          <form className="flex items-end gap-2" onSubmit={(e) => { e.preventDefault(); if (source.trim()) void discover(); }}>
            <div className="min-w-0 flex-1"><Input label={t('plugins.source')} placeholder="owner/repo" value={source} disabled={busy} autoFocus onChange={(e) => setSource(e.target.value)} /></div>
            <Button type="submit" variant="secondary" loading={busy} disabled={!source.trim()}><Search size={15} />{t('plugins.discover')}</Button>
          </form>
        ) : (
          <>
            <p className="truncate font-mono text-xs text-ink-3" title={discovery.source}>{discovery.source}</p>
            <div className="ss-fld" role="radiogroup" aria-label={t('plugins.choose')}>
              <span className="text-[13px] font-semibold">{t('plugins.choose')}</span>
              <div className="flex flex-col gap-2">
                {discovery.candidates.map((c) => (
                  <button key={c.name} type="button" role="radio" aria-checked={name === c.name} disabled={busy || !!c.problem} className={`ss-pick text-left disabled:opacity-55 ${name === c.name ? 'on' : ''}`} onClick={() => { setName(c.name); setTargets([]); }}>
                    <span className={`ss-chk rad ${name === c.name ? 'on' : ''}`} />
                    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span className="flex items-center gap-2"><span className="font-mono font-semibold">{c.name}</span>{c.version && <span className="ss-tag">{c.version}</span>}</span>
                      {(c.problem || c.description) && <span className={`text-[13px] ${c.problem ? 'text-bad' : 'text-ink-2'}`}>{c.problem || c.description}</span>}
                      {c.components.length > 0 && <span className="text-xs text-ink-3">{c.components.join(' · ')}</span>}
                    </span>
                    <span className="ss-stack" aria-hidden="true">{c.targets.map((target) => <span key={target} className="ss-at"><AgentIcon target={target} size={13} /></span>)}</span>
                  </button>
                ))}
              </div>
            </div>
            {selected && !selected.problem && (
              <div className="ss-fld">
                <span className="text-[13px] font-semibold">{t('plugins.targets')}</span>
                <div className="flex flex-wrap gap-x-6 gap-y-3.5 pt-1">
                  {(Object.keys(pluginTargets) as PluginTarget[]).map((target) => {
                    const compatible = selected.targets.includes(target) && !(isProjectMode && !pluginTargets[target].project);
                    const on = targets.includes(target);
                    return (
                      <button key={target} type="button" role="checkbox" aria-checked={on} aria-label={pluginTargets[target].label} title={compatible ? undefined : t('plugins.unsupported')} disabled={busy || !compatible} className={`ss-tgl disabled:opacity-50 ${on ? 'on' : ''}`} onClick={() => setTargets((old) => (on ? old.filter((x) => x !== target) : [...old, target]))}>
                        <span className="ic"><AgentIcon target={target} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                        {pluginTargets[target].label}
                      </button>
                    );
                  })}
                </div>
                {(Object.keys(pluginTargets) as PluginTarget[]).some((target) => !selected.targets.includes(target) || (isProjectMode && !pluginTargets[target].project)) && <span className="hp">{t('plugins.unsupported')}</span>}
              </div>
            )}
            {selected && <Input label={t('resources.col.name')} value={packageName} placeholder={selected.name} disabled={busy} onChange={(e) => setPackageName(e.target.value)} />}
          </>
        )}
        {error && <div role="alert" className="ss-note bad"><span className="flex-1">{error}</span></div>}
      </div>
      <div className="df">
        {discovery && <Button variant="ghost" className="mr-auto" disabled={busy} onClick={() => { setDiscovery(null); setError(''); }}>{t('common.back')}</Button>}
        <Button variant="ghost" disabled={busy} onClick={onClose}>{t('common.cancel')}</Button>
        {discovery && <Button loading={busy} disabled={!targets.length || !!selected?.problem} onClick={() => void preview()}>{t('plugins.preview')}</Button>}
      </div>
    </DialogShell>
  );
}
