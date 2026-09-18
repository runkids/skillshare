import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { mcpApi, type MCPMutation } from '../../api/mcp';
import { X } from 'lucide-react';
import Button from '../Button';
import CopyButton from '../CopyButton';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import { Select } from '../Input';
import { useT } from '../../i18n';
import { targetLabel } from './mcpView';
import { queryKeys } from '../../lib/queryKeys';
import { shortenHome } from '../../lib/paths';

/**
 * What Sync would write for one server, per Agent. Read only on purpose: the source keeps
 * secrets as references, and an editable native view would invite pasting them back in as plain text.
 */
export default function MCPConfigView({ mutation }: { mutation: MCPMutation }) {
  const t = useT();
  const targets = mutation.server?.targets ?? [];
  const [picked, setPicked] = useState('');
  const shown = targets.includes(picked) ? picked : targets[0];
  // A local call that touches no file, so it is simply asked again whenever the server changes.
  const view = useQuery({ queryKey: [...queryKeys.mcp, 'render', JSON.stringify(mutation)], queryFn: () => mcpApi.render(mutation), placeholderData: (prev) => prev });
  const rendered = view.data?.rendered.find((r) => r.target === shown);
  return (
    <>
      <div className="flex items-center gap-2.5">
        <Select className="w-[200px] shrink-0" value={shown} onChange={setPicked} options={targets.map((x) => ({ value: x, label: targetLabel(x) }))} />
        {rendered && <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3" title={rendered.path}>{shortenHome(rendered.path)}</span>}
        {rendered?.content && <CopyButton value={rendered.content} />}
      </div>
      {view.error && <div className="ss-note bad"><span className="flex-1">{(view.error as Error).message}</span></div>}
      {rendered?.error && <div className="ss-note warn"><span className="flex-1">{rendered.error}</span></div>}
      {rendered?.content && <div className="ss-pre"><pre>{rendered.content}</pre></div>}
      <p className="text-xs text-ink-3">{t('mcp.viewConfigHint')}</p>
    </>
  );
}

/** The same view for a server as it is saved, reached from its row. */
export function MCPConfigDialog({ mutation, onClose }: { mutation: MCPMutation; onClose: () => void }) {
  const t = useT();
  return (
    <DialogShell open onClose={onClose} padding="none" ariaLabel={t('mcp.viewConfig')} className="!max-w-[720px]">
      <div className="dh">
        <div className="flex min-w-0 flex-col gap-1"><h2 className="ss-h2">{t('mcp.viewConfig')}</h2><p className="truncate font-mono text-xs text-ink-3">{mutation.name}</p></div>
        <IconButton icon={<X size={16} />} label={t('common.close')} onClick={onClose} />
      </div>
      <div className="db"><MCPConfigView mutation={mutation} /></div>
      <div className="df"><Button variant="ghost" onClick={onClose}>{t('common.close')}</Button></div>
    </DialogShell>
  );
}
