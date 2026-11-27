import { pluginsApi } from '../../api/plugins';
import FilesDialog from '../FilesDialog';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';

/** The files of a plugin, read only, from the local copy Skillshare reviewed: the same two panes as a skill's Files tab. */
export default function PluginFilesDialog({
  name,
  source,
  onClose,
}: {
  name: string;
  source?: string;
  onClose: () => void;
}) {
  const t = useT();
  return (
    <FilesDialog
      title={name}
      subtitle={source}
      ariaLabel={t('plugins.viewFiles')}
      emptyText={t('plugins.noFiles')}
      files={{
        key: queryKeys.pluginFiles(name),
        list: () => pluginsApi.files(name).then((r) => r.files),
      }}
      file={(path) => ({
        key: queryKeys.pluginFile(name, path),
        read: () => pluginsApi.file(name, path).then((r) => r.content),
      })}
      onClose={onClose}
    />
  );
}
