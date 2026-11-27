import { useState } from 'react';
import { Copy, Eye, ExternalLink, FolderOpen } from 'lucide-react';
import { mcpApi } from '../../api/mcp';
import FilesDialog from '../FilesDialog';
import { useT } from '../../i18n';
import { useToast } from '../Toast';
import { queryKeys } from '../../lib/queryKeys';
import { baseName } from '../projects/projectView';
import { describeError, targetLabel } from './mcpView';

/** One Agent file's name and buttons: once it exists, show it in the page or open it where the dashboard runs; open its folder; copy its path. */
export default function MCPFileActions({
  target,
  path,
  local,
  openable,
  project,
}: {
  target: string;
  path: string;
  local: boolean;
  openable: boolean;
  project?: string;
}) {
  const t = useT();
  const { toast } = useToast();
  const label = targetLabel(target);
  const fail = (e: unknown) => toast(describeError(t, (e as Error).message), 'error');
  const [viewing, setViewing] = useState(false);
  return (
    <>
      {/* The file name is enough to recognise; the full path is one hover or one copy away. */}
      <span className="max-w-[130px] truncate font-mono text-xs text-ink-3" title={path}>
        {baseName(path)}
      </span>
      <span className="flex">
        {/* Works where nothing can open on the dashboard's machine, such as WSL cut off from Windows. */}
        {openable && (
          <button
            type="button"
            className="ss-ib"
            aria-label={`${t('mcp.viewFile')} · ${label}`}
            onClick={() => setViewing(true)}
          >
            <Eye size={14} />
          </button>
        )}
        {local && (
          <button
            type="button"
            className="ss-ib"
            aria-label={`${t('mcp.openFolder')} · ${label}`}
            onClick={() =>
              mcpApi
                .open(target, { project, folder: true })
                .then(() => toast(t('mcp.openedFolder'), 'info'), fail)
            }
          >
            <FolderOpen size={14} />
          </button>
        )}
        {openable && (
          <button
            type="button"
            className="ss-ib"
            aria-label={`${t('resourceDetail.actions.openInEditor')} · ${label}`}
            onClick={() =>
              mcpApi
                .open(target, { project })
                .then(
                  (r) => toast(t('resourceDetail.toast.openedIn', { editor: r.editor }), 'info'),
                  fail,
                )
            }
          >
            <ExternalLink size={14} />
          </button>
        )}
        <button
          type="button"
          className="ss-ib"
          aria-label={`${t('mcp.copyPath')} · ${label}`}
          onClick={() => {
            void navigator.clipboard?.writeText(path);
            toast(t('mcp.copied'), 'success');
          }}
        >
          <Copy size={14} />
        </button>
      </span>
      {viewing && (
        <FilesDialog
          title={baseName(path)}
          subtitle={path}
          ariaLabel={`${t('mcp.viewFile')} · ${label}`}
          files={[baseName(path)]}
          file={() => ({
            key: queryKeys.mcpFile(target, project),
            read: () => mcpApi.file(target, project).then((r) => r.content),
          })}
          fresh
          onClose={() => setViewing(false)}
        />
      )}
    </>
  );
}
