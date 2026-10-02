import { useMemo, useState } from 'react';
import type { GitConflictVersion, GitPullConflict, GitPullResolution } from '../../api/types/git';
import { useT } from '../../i18n';
import Button from '../Button';
import CodeView from '../CodeView';
import DialogShell from '../DialogShell';
import { conflictMarks } from './gitView';

interface Props {
  conflict: GitPullConflict;
  onCancel: () => void;
  onConfirm: (resolution: GitPullResolution) => void;
}

export default function PullConflictDialog({ conflict, onCancel, onConfirm }: Props) {
  const t = useT();
  const [choices, setChoices] = useState<GitPullResolution['choices']>({});
  const title = t('gitSync.conflict.title');
  const complete = conflict.files.every((file) => choices[file.path]);
  // Only two text previews can be compared; a deleted or unpreviewable side has nothing to mark
  const marks = useMemo(() => conflict.files.map((file) => {
    const comparable = [file.local, file.remote].every((version) => !version.deleted && !version.noPreview);
    return comparable ? conflictMarks(file.local.content, file.remote.content) : undefined;
  }), [conflict.files]);
  const preview = (version: GitConflictVersion, path: string, lines?: number[]) => version.deleted
    ? <div className="ss-note warn">{t('gitSync.conflict.deleted')}</div>
    : version.noPreview
      ? <div className="ss-note inf">{t('gitSync.conflict.noPreview')}</div>
      : <CodeView content={version.content} lang={path} marks={lines} className="min-h-[40vh] max-h-[55vh]" />;

  return (
    <DialogShell open onClose={onCancel} maxWidth="5xl" padding="none" ariaLabel={title}>
      <div className="dh sep"><h2 className="ss-h2">{title}</h2></div>
      <div className="db flex flex-col gap-5">
        <div className="ss-note warn">{t('gitSync.conflict.hint')}</div>
        {conflict.files.map((file, index) => (
          <section key={file.path} className="flex min-w-0 flex-col gap-3">
            <h3 className="break-all font-mono text-[13px] font-semibold">{file.path}</h3>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {(['local', 'remote'] as const).map((side) => (
                <div key={side} className="flex min-w-0 flex-col gap-2">
                  <div className="flex items-center justify-between gap-2">
                    <span className="flex items-baseline gap-2">
                      <span className="text-[13px] font-semibold">{t(`gitSync.conflict.${side}`)}</span>
                      {!!marks[index]?.[side].length && <span className="text-[12px] text-warn">{t(marks[index][side].length === 1 ? 'gitSync.conflict.diffLine' : 'gitSync.conflict.diffLines', { count: marks[index][side].length })}</span>}
                    </span>
                    <Button size="sm" variant={choices[file.path] === side ? 'primary' : 'secondary'} aria-pressed={choices[file.path] === side} aria-label={t(`gitSync.conflict.choose.${side}`, { path: file.path })} onClick={() => setChoices((previous) => ({ ...previous, [file.path]: side }))}>
                      {t(choices[file.path] === side ? 'gitSync.conflict.selected' : 'gitSync.conflict.choose')}
                    </Button>
                  </div>
                  {preview(file[side], file.path, marks[index]?.[side])}
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>
      <div className="df flex-wrap">
        <span className="flex-1 text-[13px] text-ink-2">{t('gitSync.conflict.progress', { chosen: Object.keys(choices).length, total: conflict.files.length })}</span>
        <Button variant="ghost" onClick={onCancel}>{t('common.cancel')}</Button>
        <Button variant="primary" disabled={!complete} onClick={() => onConfirm({ localHash: conflict.localHash, remoteHash: conflict.remoteHash, choices })}>{t('gitSync.conflict.apply')}</Button>
      </div>
    </DialogShell>
  );
}
