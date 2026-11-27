import { useId, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
import type { TargetFileRefusal } from '../../api/client';
import Button from '../Button';
import DialogHeader from '../DialogHeader';
import DialogShell from '../DialogShell';
import { useToast } from '../Toast';
import { targetLabel } from '../mcp/mcpView';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { invalidate } from '../../lib/queryEvents';

/** The root with the separator it uses at the end, as the fixed start of the path. */
const withSep = (root: string) =>
  /[\\/]$/.test(root) ? root : root + (root.lastIndexOf('\\') > root.lastIndexOf('/') ? '\\' : '/');

/** Adds a tab for another file the target's tool reads, somewhere under root. onAdded gets its path as listed. */
export default function AddFileDialog({
  target,
  root,
  onClose,
  onAdded,
}: {
  target: string;
  root: string;
  onClose: () => void;
  onAdded: (path: string) => void;
}) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const id = useId();
  const [path, setPath] = useState('');
  const [saving, setSaving] = useState(false);
  // Why the server refused the last try; the path is checked only on Add.
  const [failure, setFailure] = useState('');
  const title = t('targetFiles.add');
  const prefix = withSep(shortenHome(root));
  const reasonText = (reason: TargetFileRefusal) =>
    reason === 'outside' || reason === 'absolute'
      ? t('targetFiles.check.outside', { root: prefix })
      : reason === 'empty'
        ? ''
        : t(`targetFiles.check.${reason}`);

  const add = async () => {
    const rel = path.trim();
    if (!rel || saving) return;
    setSaving(true);
    setFailure('');
    try {
      const res = await api.addTargetFile(target, rel);
      await invalidate(queryClient, 'targetFilesChanged', target);
      // The new entry comes last, in the clean form the list uses.
      const added = res.files[res.files.length - 1]?.path ?? rel;
      toast(t('targetFiles.added', { file: added }), 'success');
      onAdded(added);
    } catch (err) {
      const reason =
        err instanceof ApiError && err.code === 'target_file_invalid_path'
          ? (err.params?.reason as TargetFileRefusal | undefined)
          : undefined;
      setFailure((reason && reasonText(reason)) || (err as Error).message);
      setSaving(false);
    }
  };

  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      preventClose={saving}
      ariaLabel={title}
      className="!max-w-[540px]"
    >
      <DialogHeader
        title={title}
        subtitle={t('targetFiles.addDialog.description', { tool: targetLabel(target) })}
        onClose={onClose}
        closeDisabled={saving}
      />
      <div className="db flex flex-col gap-4">
        <div className="ss-fld">
          <label htmlFor={id}>{t('targetFiles.addDialog.label')}</label>
          <span className={`ss-inp !gap-0 font-mono ${failure ? 'err' : ''}`}>
            <span className="max-w-[55%] shrink-0 truncate text-ink-3" title={root}>
              {prefix}
            </span>
            <input
              id={id}
              autoFocus
              value={path}
              onChange={(e) => {
                setPath(e.target.value);
                setFailure('');
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void add();
              }}
              placeholder="SYSTEM.md"
              spellCheck={false}
              autoComplete="off"
              aria-invalid={Boolean(failure)}
              disabled={saving}
            />
          </span>
          {failure && <span className="hp !text-bad">{failure}</span>}
        </div>
      </div>
      <div className="df">
        <span className="flex-1" />
        <Button variant="ghost" size="sm" onClick={onClose} disabled={saving}>
          {t('common.cancel')}
        </Button>
        <Button variant="primary" size="sm" onClick={add} loading={saving} disabled={!path.trim()}>
          {t('targetFiles.addDialog.submit')}
        </Button>
      </div>
    </DialogShell>
  );
}
