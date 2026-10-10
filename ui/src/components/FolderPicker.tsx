import { useState } from 'react';
import { isValidIntoPath } from '../lib/moveFolders';
import { useT } from '../i18n';
import { Input } from './Input';
import { Select } from './Select';

// Not a folder path: paths cannot contain ':'.
const NEW = ':new';

interface FolderPickerProps {
  label: string;
  /** Shown under the field, like the other install options. */
  hint?: string;
  /** '' is the source root. */
  value: string;
  onChange: (path: string) => void;
  folders: string[];
  /** A folder that cannot be picked, e.g. where a skill already is. */
  disabledFolder?: string;
}

/** Pick the source root, an existing folder, or type a new one. */
export default function FolderPicker({ label, hint, value, onChange, folders, disabledFolder }: FolderPickerProps) {
  const t = useT();
  // A value outside the list can only have been typed, so it shows as a new folder too.
  const [typing, setTyping] = useState(false);
  const creating = typing || (value !== '' && !folders.includes(value));
  const invalid = creating && value !== '' && !isValidIntoPath(value);
  const options = [
    { value: '', label: t('folderPicker.root'), disabled: disabledFolder === '', description: disabledFolder === '' ? t('folderPicker.here') : undefined },
    ...folders.map((f) => ({ value: f, label: f, disabled: f === disabledFolder, description: f === disabledFolder ? t('folderPicker.here') : undefined })),
    { value: NEW, label: t('folderPicker.new') },
  ];
  return (
    <div className="ss-fld min-w-0">
      <Select
        label={label}
        value={creating ? NEW : value}
        options={options}
        onChange={(v) => {
          setTyping(v === NEW);
          onChange(v === NEW ? '' : v);
        }}
      />
      {creating && (
        <Input
          autoFocus
          value={value}
          onChange={(e) => onChange(e.target.value.trim())}
          placeholder={t('folderPicker.newPlaceholder')}
          aria-label={t('folderPicker.new')}
          aria-invalid={invalid}
          className={invalid ? 'err' : ''}
        />
      )}
      {invalid && <span className="hp text-bad">{t('folderPicker.invalid')}</span>}
      {hint && <span className="hp font-mono">{hint}</span>}
    </div>
  );
}
