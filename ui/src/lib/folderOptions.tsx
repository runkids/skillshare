import type { ReactNode } from 'react';
import { Folder, PanelTop } from 'lucide-react';
import type { SelectOption } from '../components/Select';
import type { useT } from '../i18n';
import type { FolderOption } from './moveFolders';
import { countLabel } from './resourceGrouping';

type Kind = 'skill' | 'agent';

/** The source root, then the existing folders, as options. A path in `disabledPaths` is blocked with everything below it. */
export function folderOptions(t: ReturnType<typeof useT>, kind: Kind, folders: FolderOption[], rootCount: number, disabledPaths: string[] = []): SelectOption[] {
  const blocked = (path: string) => disabledPaths.some((d) => path === d || (d !== '' && path.startsWith(`${d}/`)));
  const row = (path: string, icon: ReactNode, trailing: string): SelectOption => {
    const off = blocked(path);
    return { value: path, label: path || t('folderPicker.root'), mono: path !== '', icon, trailing, disabled: off, description: off ? t('folderPicker.here') : undefined };
  };
  return [
    row('', <PanelTop size={14} />, countLabel(t, kind, rootCount)),
    ...folders.map((f) => row(f.path, <Folder size={14} />, String(f.count))),
  ];
}
