import { useState } from 'react';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { ConvertMethod, InstructionsChange, TargetInstructions } from '../../api/client';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import DialogHeader from '../DialogHeader';
import DialogShell from '../DialogShell';
import { Select } from '../Select';
import { useToast } from '../Toast';
import { useT, plural } from '../../i18n';
import { fileName, shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import {
  instructionsErrorMessage,
  defaultShareName,
  importLines,
  isOtherExtra,
  lineDiff,
  sharedNameProblem,
  takenName,
} from './instructionsView';
import { invalidate } from '../../lib/queryEvents';

const METHODS: ConvertMethod[] = ['import', 'rename', 'copy'];
// The share picker's choice for a new shared file; real names are extras names.
const NEW = '\0new';

/** ② Turn a target's own file (CLAUDE.md) into an AGENTS.md other tools read, with a preview first. */
export default function ConvertDialog({
  data,
  onClose,
}: {
  data: TargetInstructions;
  onClose: () => void;
}) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [method, setMethod] = useState<ConvertMethod>(data.convert[0]);
  const [keep, setKeep] = useState(true);
  // A user-level AGENTS.md no tool reads, so global mode shares by default.
  const [share, setShare] = useState(!data.project);
  // Unset until the user types: the target's own name, free among existing extras.
  const [typedName, setTypedName] = useState<string>();
  const [picked, setPicked] = useState<string>();
  const [busy, setBusy] = useState(false);
  const file = fileName(data.path ?? '');
  const tool = importLines(data.content).length;
  const sharing = share && method === 'import' && !data.project;
  const extras = useQuery({
    queryKey: queryKeys.extras,
    queryFn: () => api.listExtras(),
    enabled: !data.project,
  });
  const sharedList = useQuery({
    queryKey: queryKeys.instructions.shared,
    queryFn: () => api.listSharedInstructions(),
    enabled: !data.project,
  });
  // Files the target already uses would be imported twice.
  const available = (sharedList.data?.files ?? []).filter(
    (f) => !data.shared.some((a) => a.name === f.name),
  );
  // A new file named after the target unless the user picks one that exists.
  const choice = picked ?? NEW;
  const shareReady = sharing && Boolean(sharedList.data) && Boolean(extras.data);
  const shareInto = shareReady && choice !== NEW ? choice : undefined;
  const names = (extras.data?.extras ?? []).map((e) => e.name);
  const shareName = typedName ?? defaultShareName(data.target, names);
  const nameProblem = choice === NEW ? sharedNameProblem(shareName, names) : null;
  const taken = takenName(shareName, names) ?? shareName.trim();
  const nameError =
    nameProblem === 'invalid'
      ? t('instructions.shared.nameInvalid')
      : nameProblem === 'taken'
        ? t(
            isOtherExtra(taken, extras.data?.extras ?? [])
              ? 'instructions.shared.nameTakenFolder'
              : 'instructions.convert.nameTaken',
            { name: taken },
          )
        : '';
  const shareAs = shareReady && choice === NEW && !nameProblem ? shareName.trim() : undefined;
  const body = {
    method,
    keep_tool_lines: keep,
    ...(shareAs && { share_as: shareAs }),
    ...(shareInto && { share_into: shareInto }),
  };
  const shareMissing = sharing && !shareAs && !shareInto;
  // Rename cannot work at user level at all, so it is not offered there.
  const methods = METHODS.filter((m) => data.convert_blocked?.[m] !== 'global');
  const sharedDest = shareAs ?? shareInto;

  const preview = useQuery({
    queryKey: queryKeys.instructions.convert(data.target, body),
    queryFn: () => api.convertTargetInstructions(data.target, { ...body, apply: false }),
    // Off while applying: the refresh after success would re-plan against the
    // shared file that now exists.
    enabled: !busy && !shareMissing,
    placeholderData: keepPreviousData,
    retry: false,
  });

  const convert = async () => {
    setBusy(true);
    try {
      await api.convertTargetInstructions(data.target, { ...body, apply: true });
      void invalidate(queryClient, 'instructionsChanged');
      toast(t('instructions.convert.done', { file }), 'success');
      onClose();
    } catch (err) {
      toast(instructionsErrorMessage(err, t), 'error');
      setBusy(false);
    }
  };

  const title = t('instructions.convert.title');
  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      preventClose={busy}
      ariaLabel={title}
      className="!max-w-[980px]"
    >
      <DialogHeader
        title={title}
        subtitle={t('instructions.convert.subtitle', { path: shortenHome(data.path ?? '') })}
        onClose={onClose}
        closeDisabled={busy}
      />
      <div className="db">
        <div className="grid grid-cols-[300px_minmax(0,1fr)] items-start gap-7">
          <div className="flex flex-col gap-5">
            <div
              role="radiogroup"
              aria-label={t('instructions.convert.how')}
              className="flex flex-col gap-1"
            >
              <span className="mb-1 text-[12px] text-ink-3">{t('instructions.convert.how')}</span>
              {methods.map((m) => {
                const on = method === m;
                const blocked = data.convert_blocked?.[m];
                return (
                  <button
                    key={m}
                    type="button"
                    role="radio"
                    aria-checked={on}
                    onClick={() => setMethod(m)}
                    disabled={busy || !data.convert.includes(m)}
                    className={`flex gap-2.5 rounded-[var(--r-inner)] px-3 py-2.5 text-left disabled:opacity-60 ${on ? 'bg-sunken' : ''}`}
                  >
                    <span className={`ss-chk rad mt-0.5 shrink-0 ${on ? 'on' : ''}`} />
                    <span className="flex min-w-0 flex-col gap-0.5">
                      <span className={`text-[13px] ${on ? 'font-semibold' : ''}`}>
                        {t(`instructions.convert.method.${m}`, { file })}
                        {m === 'import' && (
                          <span className="text-[12px] font-normal text-ok">
                            {' '}
                            · {t('instructions.convert.recommended')}
                          </span>
                        )}
                      </span>
                      {on && (
                        <span className="text-[12px] leading-[1.55] text-ink-2">
                          {t(`instructions.convert.methodHint.${m}`, { file, name: data.target })}
                        </span>
                      )}
                      {blocked && (
                        <span className="text-[12px] text-ink-3">
                          {t(`instructions.convert.blocked.${blocked}`, {
                            file,
                            name: data.target,
                          })}
                        </span>
                      )}
                    </span>
                  </button>
                );
              })}
            </div>

            {sharing && sharedList.data && (
              <div className="flex flex-col gap-2">
                <label htmlFor="convert-share-name" className="text-[12px] text-ink-3">
                  {t('instructions.convert.nameLabel')}
                </label>
                <div className="flex gap-2">
                  {/* With no other shared file to pick, a new one is the only choice: just the name. */}
                  {available.length > 0 && (
                    <Select
                      ariaLabel={t('instructions.convert.shareInto')}
                      value={choice}
                      onChange={setPicked}
                      disabled={busy}
                      className={choice === NEW ? 'shrink-0' : 'min-w-0 flex-1'}
                      options={[
                        ...available.map((f) => ({ value: f.name, label: f.name })),
                        { value: NEW, label: t('instructions.convert.shareNew') },
                      ]}
                    />
                  )}
                  {choice === NEW && (
                    <span className={`ss-inp min-w-0 flex-1 ${nameProblem ? 'err' : ''}`}>
                      <input
                        id="convert-share-name"
                        className="font-mono"
                        value={shareName}
                        onChange={(e) => setTypedName(e.target.value)}
                        disabled={busy}
                        spellCheck={false}
                        autoComplete="off"
                      />
                    </span>
                  )}
                </div>
                <span className={`text-[12px] ${nameProblem ? 'text-bad' : 'text-ink-3'}`}>
                  {choice !== NEW
                    ? t('instructions.convert.shareIntoHint', { name: choice, file })
                    : nameError
                      ? nameError
                      : available.length > 0
                        ? t('instructions.convert.nameHint', { example: available[0].name })
                        : t('instructions.convert.shareHint')}
                </span>
              </div>
            )}

            {method === 'import' && (tool > 0 || !data.project) && (
              <div className="flex flex-col gap-2.5 border-t border-line-soft pt-3.5 text-[12.5px] text-ink-2">
                {tool > 0 && (
                  <Checkbox
                    size="sm"
                    label={t(plural('instructions.convert.keep', tool), { count: tool, file })}
                    checked={keep}
                    onChange={setKeep}
                    disabled={busy}
                  />
                )}
                {!data.project && (
                  <>
                    <Checkbox
                      size="sm"
                      label={t('instructions.convert.share')}
                      checked={share}
                      onChange={setShare}
                      disabled={busy}
                    />
                    {!share && (
                      <span className="pl-6 text-[12px] text-ink-3">
                        {t('instructions.convert.shareOff', { name: data.target })}
                      </span>
                    )}
                  </>
                )}
              </div>
            )}
          </div>

          <div className="flex min-w-0 flex-col gap-3">
            <span className="text-[12px] text-ink-3">{t('instructions.convert.after')}</span>
            {preview.error ? (
              <div className="ss-note bad">
                <span className="flex-1">{instructionsErrorMessage(preview.error, t)}</span>
              </div>
            ) : shareMissing ? (
              nameError ? (
                <div className="ss-note bad">
                  <span className="flex-1">{nameError}</span>
                </div>
              ) : (
                <p className="text-[13px] text-ink-3">{t('instructions.convert.nameFirst')}</p>
              )
            ) : (
              // The target's own file first, then the shared file it now imports.
              [...(preview.data?.changes ?? [])]
                .sort(
                  (a, b) =>
                    Number(Boolean(sharedDest && isSharedPath(a.path, sharedDest))) -
                    Number(Boolean(sharedDest && isSharedPath(b.path, sharedDest))),
                )
                .map((c) => (
                  <ChangePreview
                    key={c.path}
                    change={c}
                    label={
                      sharedDest && isSharedPath(c.path, sharedDest)
                        ? t('instructions.convert.sharedFile', { name: sharedDest })
                        : shortenHome(c.path)
                    }
                  />
                ))
            )}
          </div>
        </div>
      </div>
      <div className="df">
        <span className="flex-1 text-[12.5px] text-ink-3">
          {t('instructions.convert.backup', { file })}
        </span>
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          {t('common.cancel')}
        </Button>
        <Button
          variant="primary"
          onClick={convert}
          loading={busy}
          disabled={!preview.data || Boolean(preview.error) || preview.isFetching || shareMissing}
        >
          {t('instructions.convert.run')}
        </Button>
      </div>
    </DialogShell>
  );
}

/** Whether path is the AGENTS.md of the shared file name (extras/<name>/AGENTS.md). */
const isSharedPath = (path: string, name: string) =>
  /[\\/]extras[\\/]/.test(path) && path.replace(/\\/g, '/').endsWith(`/${name}/AGENTS.md`);

function ChangePreview({ change, label }: { change: InstructionsChange; label: string }) {
  const t = useT();
  const lines = lineDiff(change.before, change.after);
  const added = lines.filter((l) => l.kind === 'add').length;
  const removed = lines.filter((l) => l.kind === 'del').length;
  return (
    <div className="ss-list !shadow-none">
      <div className="ss-lh !normal-case">
        <span
          className="min-w-0 flex-1 truncate font-mono text-[12.5px] font-semibold text-ink"
          title={change.path}
        >
          {label}
        </span>
        <span className="text-[12px] text-ink-3">
          {t(`instructions.convert.status.${change.status}`)}
        </span>
        <span className="font-mono text-[12px]">
          {added > 0 && <span className="text-ok">+{added}</span>}{' '}
          {removed > 0 && <span className="text-bad">−{removed}</span>}
        </span>
      </div>
      <pre
        className="ss-code !max-h-[220px] !rounded-none !border-0 !overflow-auto !whitespace-pre-wrap"
        style={{ overflowWrap: 'anywhere' }}
      >
        {lines.map((l, i) => (
          <span key={i} className={l.kind === 'same' ? 'block' : l.kind}>
            {l.kind === 'add' ? '+ ' : l.kind === 'del' ? '− ' : '  '}
            {l.text || ' '}
          </span>
        ))}
      </pre>
    </div>
  );
}
