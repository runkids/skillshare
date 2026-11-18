import { useState } from 'react';
import type { ProjectRow } from './projectView';
import { hasSharedCopy, settingsLeft } from './projectActions';
import ConfirmDialog from '../ConfirmDialog';
import RadioOption from './RadioOption';
import { useT } from '../../i18n';

interface Props {
  project: ProjectRow;
  /** The project also has entries under mcp.projects or hooks.projects. */
  keeps: { mcp: boolean; hooks: boolean };
  loading: boolean;
  onCancel: () => void;
  /** `alsoShared`: config.yaml's copy goes too. */
  onConfirm: (alsoShared: boolean) => void;
}

/** Mount it only while open, so the choice starts at "this machine's copy" each time. */
export default function RemoveProjectDialog({
  project,
  keeps,
  loading,
  onCancel,
  onConfirm,
}: Props) {
  const t = useT();
  const [alsoShared, setAlsoShared] = useState(false);
  const twoCopies = hasSharedCopy(project);
  const left = settingsLeft(project, keeps, alsoShared);
  const body = twoCopies ? (
    <div className="flex flex-col gap-2">
      <p className="mb-1">{t('projects.removeBoth.message', { name: project.name })}</p>
      <div
        role="radiogroup"
        aria-label={t('projects.removeTitle', { name: project.name })}
        className="flex flex-col gap-2"
      >
        <RadioOption
          checked={!alsoShared}
          onSelect={() => setAlsoShared(false)}
          disabled={loading}
          title={t('projects.removeBoth.local')}
          hint={t('projects.removeBoth.localHint', { name: project.name })}
        />
        <RadioOption
          checked={alsoShared}
          onSelect={() => setAlsoShared(true)}
          disabled={loading}
          title={t('projects.removeBoth.shared')}
          hint={t('projects.removeBoth.sharedHint', { name: project.name })}
        />
      </div>
      <p className="mt-1 text-[12.5px] text-ink-3">{t('projects.removeBoth.note')}</p>
    </div>
  ) : (
    <p>
      {t(
        !project.declared
          ? 'projects.removeSharedMessage'
          : project.origin === 'shared'
            ? 'projects.removeConfigMessage'
            : 'projects.removeLocalMessage',
      )}
    </p>
  );
  const message = (
    <>
      {body}
      {left && <p className="mt-2 text-[12.5px] text-ink-3">{t(`projects.removeKeeps.${left}`)}</p>}
    </>
  );
  return (
    <ConfirmDialog
      open
      loading={loading}
      variant="danger"
      title={t('projects.removeTitle', { name: project.name })}
      message={message}
      confirmText={t('projects.remove')}
      onCancel={onCancel}
      onConfirm={() => onConfirm(alsoShared)}
    />
  );
}
