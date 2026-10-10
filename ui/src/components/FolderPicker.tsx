import type { ReactNode } from 'react';
import { Folder, FolderPlus } from 'lucide-react';
import { folderOptions } from '../lib/folderOptions';
import type { FolderOption } from '../lib/moveFolders';
import { useT } from '../i18n';
import { Select, type SelectOption } from './Select';

// Not a folder path: paths cannot contain ':'.
const NEW = ':new';

type Kind = 'skill' | 'agent';

interface FolderPickerProps {
  label: string;
  kind: Kind;
  /** '' is the source root. A path outside `folders` shows as a new folder. */
  value: string;
  onChange: (path: string) => void;
  folders: FolderOption[];
  /** Skills at and below the root, shown beside "Root". */
  rootCount: number;
  /** Paths that cannot be picked, with everything below them, e.g. where a skill already is. */
  disabledPaths?: string[];
  /** "New folder…" was chosen. `parent` is the existing folder that was selected ('' for the root or a folder not made yet); the caller shows the step that makes the name. */
  onNewFolder: (parent: string) => void;
  /** Under the field, e.g. where the install lands. */
  caption?: ReactNode;
  /** Under the caption, like the other install options. */
  hint?: string;
}

/** Pick the source root or an existing folder; "New folder…" hands over to the caller's new-folder step. */
export default function FolderPicker({ label, kind, value, onChange, folders, rootCount, disabledPaths, onNewFolder, caption, hint }: FolderPickerProps) {
  const t = useT();
  const isNew = value !== '' && !folders.some((f) => f.path === value);
  const options: SelectOption[] = [
    ...folderOptions(t, kind, folders, rootCount, disabledPaths),
    ...(isNew ? [{ value, label: value, mono: true, icon: <Folder size={14} />, trailing: t('folderPicker.newTag') }] : []),
    { value: NEW, label: t('folderPicker.new'), icon: <FolderPlus size={14} />, separated: true },
  ];
  return (
    <div className="ss-fld min-w-0">
      <Select label={label} value={value} options={options} onChange={(v) => (v === NEW ? onNewFolder(isNew ? '' : value) : onChange(v))} />
      {caption && <span className="hp">{caption}</span>}
      {hint && <span className="hp font-mono">{hint}</span>}
    </div>
  );
}
