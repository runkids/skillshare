import { useState } from 'react';
import { Copy, X } from 'lucide-react';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import { useToast } from '../Toast';
import { useT } from '../../i18n';

interface Props {
  /** Every plugin that can be shared, with the command that adds it elsewhere. */
  plugins: { name: string; command: string }[];
  /** The plugins ticked when the dialog opens. */
  initial: string[];
  onClose: () => void;
}

export default function PluginShareDialog({ plugins, initial, onClose }: Props) {
  const t = useT();
  const { toast } = useToast();
  const [picked, setPicked] = useState(() => new Set(initial));
  // Without the picker each plugin is added to Skillshare only, ready to tick Agents for on this page.
  const [ask, setAsk] = useState(false);
  const chosen = plugins.filter((p) => picked.has(p.name));
  // One line, so a pasted list runs in order: on separate lines, the first add's Agent picker would read the rest as keystrokes.
  const command = chosen.map((p) => (ask ? p.command : `${p.command} --no-tui`)).join(' && ');
  const toggle = (name: string) => setPicked((s) => { const next = new Set(s); if (!next.delete(name)) next.add(name); return next; });
  const copy = () => void navigator.clipboard.writeText(command).then(
    () => toast(t('plugins.shareCopied'), 'success'),
    () => toast(t('plugins.shareCopyFailed', { command }), 'error'),
  );
  return (
    <DialogShell open onClose={onClose} ariaLabel={t('plugins.share')} maxWidth="2xl" padding="none">
      <div className="dh">
        <div className="flex flex-col gap-1"><h2 className="ss-h2">{t('plugins.share')}</h2><p className="text-[13px] text-ink-2">{t('plugins.shareHelp')}</p></div>
        <IconButton icon={<X size={16} />} label={t('common.close')} onClick={onClose} />
      </div>
      <div className="db overflow-y-auto">
        <div className="ss-list max-h-[320px] overflow-y-auto !shadow-none">
          <div className="ss-lh !px-4">
            <Checkbox
              label={t('plugins.shareAll')}
              checked={chosen.length === plugins.length}
              indeterminate={chosen.length > 0 && chosen.length < plugins.length}
              onChange={(on) => setPicked(new Set(on ? plugins.map((p) => p.name) : []))}
            />
          </div>
          {plugins.map((p) => (
            <div key={p.name} className={`ss-r !min-h-11 ${picked.has(p.name) ? 'sel' : ''}`}>
              <Checkbox label={p.name} hideLabel checked={picked.has(p.name)} onChange={() => toggle(p.name)} />
              <span className="min-w-0 flex-1 truncate font-mono text-[13px] font-semibold" title={p.command}>{p.name}</span>
            </div>
          ))}
        </div>
        <Checkbox size="sm" label={t('plugins.shareAsk')} checked={ask} onChange={setAsk} />
        {command && <code className="ss-code block !whitespace-pre-wrap break-all !py-2">{command}</code>}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose}>{t('common.close')}</Button>
        <Button disabled={!command} onClick={copy}><Copy size={15} />{t('plugins.copyShare')}</Button>
      </div>
    </DialogShell>
  );
}
