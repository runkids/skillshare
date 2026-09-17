import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, type SyncResult } from '../../api/client';
import { mcpApi, type MCPPlan } from '../../api/mcp';
import { useT } from '../../i18n';
import { invalidateAfterSync } from '../../lib/sync';
import { queryKeys } from '../../lib/queryKeys';
import Button from '../Button';
import DialogShell from '../DialogShell';
import SyncResultList from '../SyncResultList';
import MCPPreview from './MCPPreview';

// Revisions move on any config write, including the resource sync; only the reviewed changes must hold.
const changeKey = (plan: MCPPlan) => plan.changes.filter(c => c.action !== 'unchanged').map(c => JSON.stringify([c.target, c.name, c.action])).sort().join();

export default function MCPSyncAll() {
  const t = useT();
  const cache = useQueryClient();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState<{ mcp: MCPPlan; resources: SyncResult[]; extras: string[] } | null>(null);
  const [error, setError] = useState('');
  const [completed, setCompleted] = useState<string[]>([]);
  const [finished, setFinished] = useState(false);
  const [warnings, setWarnings] = useState<string[]>([]);

  const inspect = async () => {
    setOpen(true); setBusy(true); setError(''); setCompleted([]); setPreview(null); setFinished(false); setWarnings([]);
    try {
      const mcp = await mcpApi.preview();
      const [resources, extras] = await Promise.all([api.sync({ dryRun: true }), api.syncExtras({ dry_run: true })]);
      const failed = extras.extras.flatMap(e => e.targets).find(e => e.error || e.errors?.length);
      if (failed) throw new Error(failed.error || failed.errors?.join('; '));
      setPreview({ mcp, resources: resources.results, extras: extras.extras.map(e => e.name) });
      setWarnings(resources.warnings ?? []);
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  };
  const apply = async () => {
    if (!preview) return;
    setBusy(true); setError('');
    const recheck = async () => {
      const fresh = await mcpApi.preview();
      if (fresh.blocked || changeKey(fresh) !== changeKey(preview.mcp)) throw new Error(t('mcp.previewAgain'));
      return fresh.revision;
    };
    try {
      // Recheck MCP before changing any resource and again after; native apply checks the fresh revision.
      await recheck();
      const resources = await api.sync({}); setCompleted(['Skills / Agents']); setWarnings(resources.warnings ?? []);
      const extras = await api.syncExtras();
      const failed = extras.extras.flatMap(e => e.targets).find(e => e.error || e.errors?.length);
      if (failed) throw new Error(failed.error || failed.errors?.join('; '));
      setCompleted(['Skills / Agents', 'Extras']);
      const result = await mcpApi.configure({}, await recheck(), true);
      setCompleted(['Skills / Agents', 'Extras', 'MCP', ...result.backupIds.map(id => `${t('mcp.backupId')}: ${id}`)]);
      setFinished(true);
    } catch (e) { setError((e as Error).message); }
    finally {
      setBusy(false);
      invalidateAfterSync(cache);
      void cache.invalidateQueries({ queryKey: queryKeys.mcp });
      void cache.invalidateQueries({ queryKey: queryKeys.extras });
      void cache.invalidateQueries({ queryKey: ['extras-diff'] });
    }
  };

  return <>
    <Button variant="secondary" loading={busy} onClick={inspect}>{t('mcp.syncAll')}</Button>
    <DialogShell open={open} onClose={() => setOpen(false)} preventClose={busy} maxWidth="2xl" ariaLabel={t('mcp.syncAll')}>
      <div className="space-y-4 max-h-[80vh] overflow-auto">
        <h2 className="text-xl font-semibold">{t('mcp.syncAll')}</h2>
        <p className="text-sm text-pencil-light">{t('mcp.allHint')}</p>
        {error ? <p role="alert" className="text-danger">{error}</p> : null}
        {warnings.map(warning => <p key={warning} className="text-warning">{warning}</p>)}
        {completed.length ? <ul className="text-success list-disc ps-5">{completed.map(item => <li key={item}>{item}</li>)}</ul> : null}
        {preview && !finished ? <>
          <SyncResultList results={preview.resources} />
          <p>Extras: {preview.extras.join(', ') || '—'}</p>
          <MCPPreview plan={preview.mcp} />
        </> : null}
        <div className="flex gap-2">
          <Button variant="ghost" disabled={busy} onClick={() => setOpen(false)}>{t('common.close')}</Button>
          {!finished ? <Button loading={busy} disabled={!preview || preview.mcp.blocked || Boolean(error)} onClick={apply}>{t('mcp.syncAll')}</Button> : null}
        </div>
      </div>
    </DialogShell>
  </>;
}
