import { scalarText } from '../../lib/scalarText';
import { useState } from 'react';
import { TriangleAlert, X } from 'lucide-react';
import { api, ApiError } from '../../api/client';
import type { InstructionsWarning } from '../../api/client';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import DialogShell from '../DialogShell';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import type { SharedMode } from './instructionsView';
import {
  instructionsErrorMessage,
  locationFile,
  locationModeOptions,
  projectLocationFile,
} from './instructionsView';

type Problem = { text: string; field?: 'folder' | 'file'; tone: 'bad' | 'warn' };

/** Puts a shared file in any folder, under its own name or another, and syncs it there. */
export default function AddLocationDialog({
  name,
  file,
  fileLinks,
  project = false,
  onClose,
  onAdded,
}: {
  name: string;
  /** The shared file's own name, e.g. AGENTS.md. */
  file: string;
  fileLinks: boolean;
  /** The folder is relative to the project root, and so is the file shown. */
  project?: boolean;
  onClose: () => void;
  onAdded: (path: string, warnings?: InstructionsWarning[]) => void;
}) {
  const t = useT();
  const [folder, setFolder] = useState('');
  const [as, setAs] = useState('');
  const [mode, setMode] = useState<SharedMode>(fileLinks ? 'symlink' : 'copy');
  // import only helps where the reading tool follows @import lines; the user has to say so.
  const [supportsImport, setSupportsImport] = useState(false);
  const [saving, setSaving] = useState(false);
  const [problem, setProblem] = useState<Problem | null>(null);
  const title = t('instructions.locations.dialog.title');
  const target = project ? projectLocationFile(folder, as, file) : locationFile(folder, as, file);
  const targetName = as.trim() || file;

  const add = async () => {
    setSaving(true);
    setProblem(null);
    try {
      const res = await api.addInstructionLocation(name, {
        path: folder.trim(),
        mode,
        ...(as.trim() && { as: as.trim() }),
      });
      onAdded(target, res.warnings);
    } catch (err) {
      const code = err instanceof ApiError ? err.code : undefined;
      // The server names the held file by its path here, not by a target, so it gets its own sentence.
      const held =
        err instanceof ApiError && code === 'instructions_target_held'
          ? t('instructions.locations.held', {
              name: scalarText(err.params?.name),
              target: shortenHome(scalarText(err.params?.target)),
            })
          : null;
      setProblem({
        text: held ?? instructionsErrorMessage(err, t),
        field:
          code === 'instructions_location_directory' || code === 'instructions_invalid_file_name'
            ? 'file'
            : code === 'instructions_location_outside_project'
              ? 'folder'
              : undefined,
        tone:
          code === 'instructions_location_is_target' || code === 'instructions_location_exists'
            ? 'warn'
            : 'bad',
      });
      setSaving(false);
    }
  };

  const pickImport = (on: boolean) => {
    setSupportsImport(on);
    if (!on && mode === 'import') setMode(fileLinks ? 'symlink' : 'copy');
  };

  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      preventClose={saving}
      ariaLabel={title}
      className="!max-w-[600px]"
    >
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">
            {t(
              project
                ? 'instructions.locations.dialog.projectSubtitle'
                : 'instructions.locations.dialog.subtitle',
              { name, file },
            )}
          </p>
        </div>
        <button
          type="button"
          className="ss-ib"
          aria-label={t('common.close')}
          onClick={onClose}
          disabled={saving}
        >
          <X size={16} />
        </button>
      </div>
      <div className="db flex flex-col gap-4">
        <div className="grid grid-cols-2 gap-3.5">
          <div className="ss-fld">
            <label htmlFor="location-folder">{t('instructions.locations.dialog.folder')}</label>
            <span className={`ss-inp ${problem?.field === 'folder' ? 'err' : ''}`}>
              <input
                id="location-folder"
                className="font-mono"
                autoFocus
                value={folder}
                onChange={(e) => {
                  setFolder(e.target.value);
                  setProblem(null);
                }}
                placeholder={project ? 'docs/ai' : '~/work/notes'}
                disabled={saving}
                spellCheck={false}
                autoComplete="off"
                aria-invalid={problem?.field === 'folder' || undefined}
              />
            </span>
            <span className={`hp ${problem?.field === 'folder' ? 'text-bad' : ''}`}>
              {problem?.field === 'folder'
                ? problem.text
                : t(
                    project
                      ? 'instructions.locations.dialog.projectFolderHint'
                      : 'instructions.locations.dialog.folderHint',
                  )}
            </span>
          </div>
          <div className="ss-fld">
            <label htmlFor="location-as">{t('instructions.locations.dialog.fileName')}</label>
            <span className={`ss-inp ${problem?.field === 'file' ? 'err' : ''}`}>
              <input
                id="location-as"
                className="font-mono"
                value={as}
                onChange={(e) => {
                  setAs(e.target.value);
                  setProblem(null);
                }}
                placeholder={file}
                disabled={saving}
                spellCheck={false}
                autoComplete="off"
                aria-invalid={problem?.field === 'file' || undefined}
              />
            </span>
            <span className={`hp ${problem?.field === 'file' ? 'text-bad' : ''}`}>
              {problem?.field === 'file'
                ? problem.text
                : t('instructions.locations.dialog.fileNameHint', { file })}
            </span>
          </div>
        </div>

        {folder.trim() && (
          <div className="flex flex-col gap-1 rounded-[9px] bg-sunken px-3 py-2.5">
            <span className="text-[12px] text-ink-3">
              {t('instructions.locations.dialog.writesTo')}
            </span>
            <span className="font-mono text-[13px] font-semibold break-all">{target}</span>
          </div>
        )}

        <div
          role="radiogroup"
          aria-label={t('instructions.locations.dialog.how', { name })}
          className="flex flex-col gap-1"
        >
          <span className="mb-1 text-[13px] font-semibold">
            {t('instructions.locations.dialog.how', { name })}
          </span>
          {locationModeOptions(fileLinks).map((o) => {
            const on = mode === o.mode;
            const disabled =
              saving || Boolean(o.blocked) || (o.mode === 'import' && !supportsImport);
            return (
              <div
                key={o.mode}
                className={`flex flex-col gap-2 rounded-[10px] ${on ? 'bg-sunken' : ''}`}
              >
                <button
                  type="button"
                  role="radio"
                  aria-checked={on}
                  onClick={() => setMode(o.mode)}
                  disabled={disabled}
                  className="flex gap-2.5 px-3 py-2.5 text-left disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <span className={`ss-chk rad mt-0.5 shrink-0 ${on ? 'on' : ''}`} />
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span className="font-mono text-[13px] font-semibold">
                      {o.mode}
                      {o.isDefault && (
                        <span className="font-sans font-normal text-ink-3">
                          {' '}
                          {t('instructions.mode.default')}
                        </span>
                      )}
                    </span>
                    <span
                      className={`text-[12px] leading-[1.55] ${o.blocked ? 'text-warn' : 'text-ink-2'}`}
                    >
                      {o.blocked
                        ? t(`instructions.mode.blocked.${o.blocked}`, { target: targetName })
                        : t(`instructions.mode.${o.mode}`, { name, file: targetName })}
                    </span>
                  </span>
                </button>
                {o.mode === 'import' && (
                  <Checkbox
                    size="sm"
                    className="!items-start pr-3 pb-2.5 pl-[38px] text-[12.5px] leading-[1.45] text-ink-2"
                    label={t('instructions.locations.dialog.importSupported')}
                    checked={supportsImport}
                    onChange={pickImport}
                    disabled={saving}
                  />
                )}
              </div>
            );
          })}
        </div>

        {problem && !problem.field && (
          <div className={`ss-note ${problem.tone}`} role="alert">
            <TriangleAlert size={16} />
            <span className="flex-1">{problem.text}</span>
          </div>
        )}
      </div>
      <div className="df">
        <span className="flex-1 text-[12.5px] text-ink-3">
          {t(
            project
              ? 'instructions.locations.dialog.projectFooter'
              : 'instructions.locations.dialog.footer',
          )}
        </span>
        <Button variant="ghost" onClick={onClose} disabled={saving}>
          {t('common.cancel')}
        </Button>
        <Button variant="primary" onClick={add} loading={saving} disabled={!folder.trim()}>
          {t('instructions.locations.dialog.submit')}
        </Button>
      </div>
    </DialogShell>
  );
}
