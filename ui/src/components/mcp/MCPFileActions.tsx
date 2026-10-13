import { Copy, ExternalLink, FolderOpen } from 'lucide-react';
import { mcpApi } from '../../api/mcp';
import { useT } from '../../i18n';
import { useToast } from '../Toast';
import { baseName } from '../projects/projectView';
import { describeError, targetLabel } from './mcpView';

/** One Agent file's name and buttons: open its folder, or the file once it exists, where the dashboard runs, and copy its path. */
export default function MCPFileActions({ target, path, local, openable, project }: { target: string; path: string; local: boolean; openable: boolean; project?: string }) {
  const t = useT();
  const { toast } = useToast();
  const label = targetLabel(target);
  const fail = (e: unknown) => toast(describeError(t, (e as Error).message), 'error');
  return (
    <>
      {/* The file name is enough to recognise; the full path is one hover or one copy away. */}
      <span className="max-w-[130px] truncate font-mono text-xs text-ink-3" title={path}>{baseName(path)}</span>
      <span className="flex">
        {local && <button type="button" className="ss-ib" aria-label={`${t('mcp.openFolder')} · ${label}`} onClick={() => mcpApi.open(target, { project, folder: true }).then(() => toast(t('mcp.openedFolder'), 'info'), fail)}><FolderOpen size={14} /></button>}
        {openable && <button type="button" className="ss-ib" aria-label={`${t('resourceDetail.actions.openInEditor')} · ${label}`} onClick={() => mcpApi.open(target, { project }).then((r) => toast(t('resourceDetail.toast.openedIn', { editor: r.editor }), 'info'), fail)}><ExternalLink size={14} /></button>}
        <button type="button" className="ss-ib" aria-label={`${t('mcp.copyPath')} · ${label}`} onClick={() => { void navigator.clipboard?.writeText(path); toast(t('mcp.copied'), 'success'); }}><Copy size={14} /></button>
      </span>
    </>
  );
}
