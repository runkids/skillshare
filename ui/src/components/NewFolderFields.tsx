import { useT } from '../i18n';
import { folderOptions } from '../lib/folderOptions';
import { linkPrefix, newFolderPath, type FolderOption } from '../lib/moveFolders';
import { Input } from './Input';
import { Select } from './Select';

interface NewFolderFieldsProps {
  kind: 'skill' | 'agent';
  folders: FolderOption[];
  rootCount: number;
  /** Paths the new folder cannot go in, with everything below them; same meaning as on FolderPicker. */
  disabledPaths?: string[];
  /** The folder the new one goes in; '' is the root. */
  parent: string;
  name: string;
  /** The skill the links are named after, when known. */
  skill: string | null;
  onParent: (parent: string) => void;
  onName: (name: string) => void;
  /** Enter in the name field with a usable name. */
  onSubmit: () => void;
}

/** The new-folder step of the folder picker: where, what name, and the path and link names that come out. */
export default function NewFolderFields({ kind, folders, rootCount, disabledPaths, parent, name, skill, onParent, onName, onSubmit }: NewFolderFieldsProps) {
  const t = useT();
  const path = newFolderPath(parent, name);
  const invalid = name !== '' && path === null;
  return (
    <div className="flex flex-col gap-[18px]">
      <Select label={t('folderPicker.inside')} value={parent} onChange={onParent} options={folderOptions(t, kind, folders, rootCount, disabledPaths)} />
      <div className="flex flex-col gap-1.5">
        <Input
          autoFocus
          label={t('folderPicker.name')}
          className={`font-mono ${invalid ? 'err' : ''}`}
          value={name}
          onChange={(e) => onName(e.target.value.trim())}
          onKeyDown={(e) => e.key === 'Enter' && path && onSubmit()}
          placeholder={t('folderPicker.newPlaceholder')}
          aria-invalid={invalid}
          aria-describedby="folder-name-rule"
        />
        <span id="folder-name-rule" className={`text-xs ${invalid ? 'text-bad' : 'text-ink-3'}`}>{t('folderPicker.nameRule')}</span>
      </div>
      <div className="ss-list !shadow-none">
        <div className="ss-r text-xs font-semibold text-ink-3">{t('folderPicker.result')}</div>
        <div className="ss-r">
          <span className="w-[88px] shrink-0 text-[13px] text-ink-2">{t('folderPicker.resultFolder')}</span>
          <span className="min-w-0 truncate font-mono text-[13px] font-semibold">{path ?? '—'}</span>
        </div>
        <div className="ss-r">
          <span className="w-[88px] shrink-0 text-[13px] text-ink-2">{t('folderPicker.resultLink')}</span>
          <span className="min-w-0 truncate font-mono text-[13px]">
            {path ? linkPrefix(path) : '—'}
            {path && (skill ?? <span className="text-ink-3">&lt;skill&gt;</span>)}
          </span>
        </div>
      </div>
    </div>
  );
}
