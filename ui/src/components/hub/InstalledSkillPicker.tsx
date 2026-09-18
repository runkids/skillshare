import { useState } from 'react';
import type { HubEntry } from '../../api/hubDrafts';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { Checkbox, Input } from '../Input';
import { useT } from '../../i18n';

interface Props { candidates: HubEntry[]; onClose: () => void; onAdd: (entries: HubEntry[]) => void }
export default function InstalledSkillPicker({ candidates, onClose, onAdd }: Props) {
  const t = useT();
  const [query, setQuery] = useState('');
  const [selection, setSelection] = useState<string[]>([]);
  const selected = new Set(selection);
  return <DialogShell open onClose={onClose} ariaLabel={t('hubBuilder.fromInstalled')}>
    <div className="space-y-4">
      <h2 className="ss-h2">{t('hubBuilder.fromInstalled')}</h2>
      <Input label={t('hubBuilder.filter')} value={query} onChange={event => setQuery(event.target.value)} />
      <div className="max-h-80 overflow-auto space-y-3">
        {candidates.length === 0 && <p>{t('hubBuilder.noInstalled')}</p>}
        {candidates.filter(entry => `${entry.data.name} ${entry.id}`.toLowerCase().includes(query.toLowerCase())).map(entry => <div key={entry.id}>
          <Checkbox label={entry.data.name || entry.id} checked={selected.has(entry.id)} onChange={checked => setSelection(current => checked ? [...current, entry.id] : current.filter(id => id !== entry.id))} />
          <p className="text-xs text-ink-2 break-all ms-6">{entry.id}</p>
        </div>)}
      </div>
      <Button disabled={selection.length === 0} onClick={() => onAdd(candidates.filter(entry => selected.has(entry.id)))}>{t('hubBuilder.addSelected', { count: selection.length })}</Button>
    </div>
  </DialogShell>;
}
