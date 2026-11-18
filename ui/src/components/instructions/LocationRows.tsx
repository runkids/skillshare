import { useState } from 'react';
import { Folder, TriangleAlert } from 'lucide-react';
import { api } from '../../api/client';
import type { InstructionLocation, InstructionsWarning } from '../../api/client';
import Button from '../Button';
import { Select } from '../Select';
import { useToast } from '../Toast';
import { useT } from '../../i18n';
import { fileName } from '../../lib/paths';
import RestorePreviewDialog from './RestorePreviewDialog';
import type { LocationHint } from './instructionsView';
import {
  blockMode,
  locationHint,
  locationLabel,
  locationModeOptions,
  pickedMode,
  statusTone,
} from './instructionsView';

/**
 * The rows of a shared file's other locations: mode, status, removal after a restore
 * preview, and collect or reapply for an edited file. The caller wraps them in a list.
 */
export default function LocationRows({
  name,
  locations,
  fileLinks,
  busy,
  project = false,
  act,
  warn,
  onResolve,
}: {
  name: string;
  locations: InstructionLocation[];
  fileLinks: boolean;
  busy: boolean;
  /** Paths are shown relative to the project root. */
  project?: boolean;
  /** Runs one change with the caller busy, reporting failures and refetching afterwards. */
  act: (fn: () => Promise<void>) => Promise<void>;
  warn: (warnings?: InstructionsWarning[]) => void;
  /** Asks before collecting an edited location into the shared file, or overwriting it; block when only its content block is touched. */
  onResolve: (label: string, path: string, action: 'collect' | 'reapply', block: boolean) => void;
}) {
  const t = useT();
  const { toast } = useToast();
  const [removing, setRemoving] = useState<InstructionLocation | null>(null);

  const setMode = (l: InstructionLocation, mode: string) =>
    act(async () => {
      warn((await api.setInstructionLocationMode(name, l.path, mode)).warnings);
      toast(
        t('instructions.mode.changed', { target: locationLabel(l, project), name, mode }),
        'success',
      );
    });
  const remove = (l: InstructionLocation) =>
    act(async () => {
      warn((await api.removeInstructionLocation(name, l.path)).warnings);
      toast(
        t('instructions.locations.removed', { path: locationLabel(l, project), name }),
        'success',
      );
    });

  const hintText = (h: LocationHint) => {
    switch (h) {
      case 'folderLink':
        return t('instructions.hint.folderLink');
      case 'directory':
        return t('instructions.hint.directory');
      case 'noSource':
        return t('instructions.hint.noSource', { name });
      case 'notSynced':
        return t('instructions.locations.hint.notSynced');
      case 'drift':
        return t('instructions.locations.hint.drift', { name });
      case 'driftImport':
        return t('instructions.locations.hint.driftImport');
      case 'import':
        return t('instructions.locations.hint.import');
    }
  };

  return (
    <>
      {locations.map((l) => {
        const hint = locationHint(l);
        const path = locationLabel(l, project);
        return (
          <div key={l.path} className="ss-r !block !p-0">
            <div className="flex min-h-[56px] items-center gap-3 px-4 py-2">
              <span className="ss-at">
                <Folder size={15} className="text-ink-3" />
              </span>
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="truncate font-mono text-[13px] font-semibold" title={l.file}>
                  {path}
                </span>
                {hint && (
                  <span className={`text-[12px] ${hint === 'import' ? 'text-ink-3' : 'text-warn'}`}>
                    {hintText(hint)}
                  </span>
                )}
              </span>
              <Select
                size="sm"
                align="end"
                className="w-[104px] shrink-0 font-mono"
                ariaLabel={t('instructions.mode.label', { target: path, name })}
                value={pickedMode(l.mode)}
                disabled={busy}
                onChange={(m) => {
                  if (m !== pickedMode(l.mode)) void setMode(l, m);
                }}
                options={locationModeOptions(fileLinks).map((o) => ({
                  value: o.mode,
                  label: o.mode,
                  note: o.isDefault ? t('instructions.mode.default') : undefined,
                  disabled: Boolean(o.blocked),
                  description: o.blocked
                    ? t(`instructions.mode.blocked.${o.blocked}`, { target: path })
                    : t(`instructions.mode.${o.mode}`, { name, file: fileName(l.file) }),
                }))}
              />
              <span className="w-[90px] shrink-0">
                <span className={`ss-st ${statusTone(l.status)}`}>{l.status}</span>
              </span>
              <Button
                variant="ghost"
                size="sm"
                disabled={busy}
                aria-label={t('instructions.locations.removeLabel', { path })}
                onClick={() => setRemoving(l)}
              >
                {t('instructions.locations.remove')}
              </Button>
            </div>
            {l.status === 'modified' && (
              <div className="ss-note warn mr-4 mb-3 ml-[58px] !items-center">
                <TriangleAlert size={16} className="!mt-0" />
                <span className="flex-1">
                  {t(
                    blockMode(l.mode)
                      ? 'instructions.row.modifiedBlock'
                      : 'instructions.row.modified',
                    { name },
                  )}
                </span>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={busy}
                  onClick={() => onResolve(path, l.path, 'collect', blockMode(l.mode))}
                >
                  {t(
                    blockMode(l.mode)
                      ? 'instructions.resolve.collect.itemBlock'
                      : 'instructions.resolve.collect.item',
                    { name },
                  )}
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={busy}
                  onClick={() => onResolve(path, l.path, 'reapply', blockMode(l.mode))}
                >
                  {t(
                    blockMode(l.mode)
                      ? 'instructions.resolve.reapply.itemBlock'
                      : 'instructions.resolve.reapply.item',
                    { name },
                  )}
                </Button>
              </div>
            )}
          </div>
        );
      })}
      {removing && (
        <RestorePreviewDialog
          location
          name={name}
          target={removing.path}
          label={locationLabel(removing, project)}
          mode={removing.mode}
          busy={busy}
          onClose={() => setRemoving(null)}
          // Closed first: the refetch after removing would ask for the preview of a location that is gone.
          onConfirm={() => {
            setRemoving(null);
            void remove(removing);
          }}
        />
      )}
    </>
  );
}
