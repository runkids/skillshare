import { useState } from 'react';
import { pluginsApi, pluginTargets, type PluginDiscovery, type PluginRequest, type PluginTarget } from '../../api/plugins';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { Input, Checkbox } from '../Input';
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
  return <DialogShell open onClose={onClose} preventClose={busy} ariaLabel={t('plugins.add')} maxWidth="2xl">
    <div className="space-y-5">
      <h2 className="text-xl font-semibold">{t('plugins.add')}</h2>
      {!discovery ? <>
        <Input label={t('plugins.source')} placeholder="owner/repo" value={source} disabled={busy} onChange={(e) => setSource(e.target.value)} />
        <p className="text-sm text-pencil-light">{t('plugins.sourceHelp')}</p>
        <Button onClick={() => void discover()} loading={busy} disabled={!source.trim()}>{t('plugins.discover')}</Button>
      </> : <>
        <p className="text-sm break-all text-pencil-light">{discovery.source}</p>
        <fieldset className="space-y-2">
          <legend className="font-medium mb-2">{t('plugins.choose')}</legend>
          {discovery.candidates.map((c) => <label key={c.name} className="flex gap-3 rounded-lg border border-muted p-3">
            <input type="radio" name="plugin" value={c.name} disabled={busy || !!c.problem} checked={name === c.name} onChange={() => { setName(c.name); setTargets([]); }} />
            <span><span className="font-medium">{c.name}</span><span className="block text-sm text-pencil-light">{c.problem || c.description}</span><span className="block text-xs text-muted-dark">{c.components.join(' · ')}</span></span>
          </label>)}
        </fieldset>
        {selected && !selected.problem && <fieldset className="space-y-2">
          <legend className="font-medium mb-2">{t('plugins.targets')}</legend>
          {(Object.keys(pluginTargets) as PluginTarget[]).map((target) => {
            const compatible = selected.targets.includes(target) && !(isProjectMode && !pluginTargets[target].project);
            return <div key={target} className="space-y-1"><Checkbox label={pluginTargets[target].label} checked={targets.includes(target)} disabled={busy || !compatible} onChange={(on) => setTargets((old) => on ? [...old, target] : old.filter((x) => x !== target))} />
              {!compatible && <p className="text-xs text-pencil-light">{t('plugins.unsupported')}</p>}</div>;
          })}
        </fieldset>}
        {selected && <Input label={t('resources.col.name')} value={packageName} placeholder={selected.name} disabled={busy} onChange={(e) => setPackageName(e.target.value)} />}
        <div className="flex gap-2"><Button variant="ghost" disabled={busy} onClick={() => { setDiscovery(null); setError(''); }}>{t('common.back')}</Button><Button loading={busy} disabled={!targets.length || !!selected?.problem} onClick={() => void preview()}>{t('plugins.preview')}</Button></div>
      </>}
      {error && <p role="alert" className="text-danger text-sm">{error}</p>}
      <Button variant="ghost" disabled={busy} onClick={onClose}>{t('common.cancel')}</Button>
    </div>
  </DialogShell>;
}
