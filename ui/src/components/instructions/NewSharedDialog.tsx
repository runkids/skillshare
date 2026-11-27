import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { SharedInstructionsTarget } from '../../api/client';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import CodeEditor from '../CodeEditor';
import DialogHeader from '../DialogHeader';
import DialogShell from '../DialogShell';
import { Select } from '../Select';
import { useToast } from '../Toast';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import { BoxHeader, InstructionsPreview } from './ViewTabs';
import {
  instructionsErrorMessage,
  isOtherExtra,
  isImportLine,
  sharedNameProblem,
  takenName,
} from './instructionsView';

/** Creates a shared instruction file, from content or by moving a target's current file into it. */
export default function NewSharedDialog({
  targets,
  project = false,
  onClose,
  onCreated,
}: {
  targets: SharedInstructionsTarget[];
  /** A project's shared file starts empty: there is no target file to move in. */
  project?: boolean;
  onClose: () => void;
  /** Resolves once the new file can be shown. */
  onCreated: (name: string) => Promise<void>;
}) {
  const t = useT();
  const { toast } = useToast();
  const [name, setName] = useState('');
  const [from, setFrom] = useState('');
  const [content, setContent] = useState('');
  const [saving, setSaving] = useState(false);
  const [view, setView] = useState<'edit' | 'preview'>('edit');
  const title = t(project ? 'instructions.projectShared.new' : 'instructions.shared.newTitle');
  // Extra names are one namespace: a folder extra takes the name too.
  const extras = useQuery({ queryKey: queryKeys.extras, queryFn: () => api.listExtras() });
  const names = (extras.data?.extras ?? []).map((e) => e.name);
  const problem = sharedNameProblem(name, names);
  const taken = takenName(name, names) ?? name.trim();
  // A target already on a shared file has nothing of its own to move in.
  const sources = targets.filter((tg) => tg.exists && !tg.same_as && tg.assigned.length === 0);

  const create = async () => {
    setSaving(true);
    try {
      await api.createSharedInstructions({
        name: name.trim(),
        ...(from ? { from_target: from } : { content }),
      });
      await onCreated(name.trim());
      toast(t('instructions.shared.created', { name: name.trim() }), 'success');
    } catch (err) {
      toast(instructionsErrorMessage(err, t), 'error');
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
      className="!max-w-[720px]"
    >
      <DialogHeader
        title={title}
        subtitle={t(
          project ? 'instructions.projectShared.newSubtitle' : 'instructions.shared.newSubtitle',
        )}
        onClose={onClose}
        closeDisabled={saving}
      />
      <div className="db">
        <div className="grid grid-cols-2 gap-3.5">
          <div className="ss-fld">
            <label htmlFor="shared-name">{t('instructions.shared.name')}</label>
            <span className={`ss-inp ${problem ? 'err' : ''}`}>
              <input
                id="shared-name"
                autoFocus
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="personal"
                disabled={saving}
              />
            </span>
            <span className={`hp ${problem ? 'text-bad' : ''}`}>
              {problem === 'invalid'
                ? t('instructions.shared.nameInvalid')
                : problem === 'taken'
                  ? t(
                      isOtherExtra(taken, extras.data?.extras ?? [])
                        ? 'instructions.shared.nameTakenFolder'
                        : 'instructions.convert.nameTaken',
                      { name: taken },
                    )
                  : t('instructions.shared.nameHint')}
            </span>
          </div>
          {!project && (
            <div className="ss-fld">
              <span className="text-[13px] font-semibold">
                {t('instructions.shared.startFrom')}
              </span>
              <Select
                value={from}
                onChange={setFrom}
                options={[
                  { value: '', label: t('instructions.shared.startEmpty') },
                  ...sources.map((tg) => ({
                    value: tg.name,
                    label: t('instructions.shared.startCopy', { name: tg.name }),
                    icon: <AgentIcon target={tg.name} size={14} />,
                    description: shortenHome(tg.path),
                  })),
                ]}
                disabled={saving}
              />
              <span className="hp">
                {t(
                  from ? 'instructions.shared.startCopyHint' : 'instructions.shared.startEmptyHint',
                  { name: from },
                )}
              </span>
            </div>
          )}
        </div>
        {!from && (
          <div className="ss-code flex h-[300px] flex-col !overflow-hidden !p-0 !whitespace-normal">
            <BoxHeader content={content} view={view} onChange={setView} />
            {view === 'edit' ? (
              <CodeEditor
                value={content}
                onChange={setContent}
                ariaLabel={t('instructions.shared.content')}
                markLine={isImportLine}
                disabled={saving}
                wrap
                fill
                className="min-h-0 flex-1 !rounded-none !border-0"
                placeholder={t('instructions.shared.contentPlaceholder')}
              />
            ) : (
              <InstructionsPreview content={content} names={[]} />
            )}
          </div>
        )}
      </div>
      <div className="df">
        <span className="flex-1" />
        <Button variant="ghost" onClick={onClose} disabled={saving}>
          {t('common.cancel')}
        </Button>
        <Button
          variant="primary"
          onClick={create}
          loading={saving}
          disabled={!name.trim() || Boolean(problem)}
        >
          {t('instructions.shared.create')}
        </Button>
      </div>
    </DialogShell>
  );
}
