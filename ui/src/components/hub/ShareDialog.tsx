import { scalarText } from '../../lib/scalarText';
import { useState } from 'react';
import type { ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Check, Copy, Download } from 'lucide-react';
import { ApiError } from '../../api/client';
import { hubAddCommand, hubDrafts } from '../../api/hubDrafts';
import type { DraftResponse } from '../../api/hubDrafts';
import Button from '../Button';
import DialogHeader from '../DialogHeader';
import DialogShell from '../DialogShell';
import { blockedEntries, downloadIndex, publishable } from './hubShared';
import { queryKeys } from '../../lib/queryKeys';
import { useT, plural } from '../../i18n';
import { invalidate } from '../../lib/queryEvents';

function Step({ n, children }: { n: number; children: ReactNode }) {
  return (
    <div className="flex gap-3.5">
      <span className="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-pri text-xs font-bold text-on-pri">
        {n}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-2">{children}</div>
    </div>
  );
}

/** Three steps from an own hub to one the team can add: download, host, hand out the command. */
export default function ShareDialog({
  response,
  onClose,
}: {
  response: DraftResponse;
  onClose: () => void;
}) {
  const t = useT();
  const queryClient = useQueryClient();
  const { draft, problems } = response;
  const saved = scalarText(draft.fields?.publishUrl);
  const [location, setLocation] = useState(saved);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);
  const blocked = blockedEntries(problems).size;
  const empty = draft.entries.length === 0;
  const name = draft.name || t('hubBuilder.untitled');

  const fail = (err: unknown) =>
    setError(
      err instanceof ApiError && err.status === 409
        ? t('hubs.conflict')
        : err instanceof Error
          ? err.message
          : String(err),
    );

  /** The hosted URL belongs to the hub, so a reopened hub remembers it. */
  async function commit() {
    if (location.trim() === saved.trim()) return true;
    setBusy(true);
    setError('');
    try {
      const res = await hubDrafts.save({
        ...draft,
        fields: { ...draft.fields, publishUrl: location.trim() },
      });
      queryClient.setQueryData(queryKeys.hub.draft(res.draft.id), res);
      void invalidate(queryClient, 'hubDraftsChanged');
      return true;
    } catch (err) {
      fail(err);
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function download() {
    setBusy(true);
    setError('');
    try {
      downloadIndex(await hubDrafts.export(draft));
    } catch (err) {
      fail(err);
    } finally {
      setBusy(false);
    }
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(hubAddCommand(location, name));
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Ignore clipboard failures; the command stays visible to copy by hand.
    }
  }

  const close = async () => {
    if (await commit()) onClose();
  };

  return (
    <DialogShell
      open
      onClose={() => void close()}
      maxWidth="xl"
      padding="none"
      preventClose={busy}
      ariaLabel={t('hubs.share.title', { name })}
    >
      <DialogHeader
        title={t('hubs.share.title', { name })}
        subtitle={
          empty
            ? t('hubBuilder.problem.empty')
            : blocked > 0
              ? t(plural('hubBuilder.blocked', blocked), { count: blocked })
              : t(plural('hubs.share.ready', draft.entries.length), {
                  count: draft.entries.length,
                })
        }
        onClose={() => void close()}
        closeDisabled={busy}
      />
      <div className="db !gap-5">
        {error && (
          <div role="alert" className="ss-note bad">
            <span className="flex-1 break-words">{error}</span>
          </div>
        )}

        <Step n={1}>
          <span className="text-sm font-semibold">{t('hubs.share.step1')}</span>
          <Button
            variant="secondary"
            className="self-start"
            disabled={busy || blocked > 0 || empty}
            onClick={() => void download()}
          >
            <Download size={14} />
            {t('hubs.share.download')}
          </Button>
          {(blocked > 0 || empty) && (
            <span className="text-[12.5px] text-ink-3">{t('hubs.share.fixFirst')}</span>
          )}
        </Step>

        <Step n={2}>
          <label htmlFor="hub-share-location" className="text-sm font-semibold">
            {t('hubs.share.step2')}
          </label>
          <span className="ss-inp font-mono">
            <input
              id="hub-share-location"
              placeholder="https://example.com/skillshare-hub.json"
              value={location}
              onChange={(e) => setLocation(e.target.value)}
              onBlur={() => void commit()}
            />
          </span>
          <span className="text-[12.5px] text-ink-3">{t('hubs.share.step2Hint')}</span>
          {/* The URL is only asked for to build this command, so the command sits right under it. */}
          <span className="mt-2 text-[13px] text-ink-2">{t('hubs.share.step3')}</span>
          {publishable(location) ? (
            <>
              <div className="ss-code flex items-center gap-2 !whitespace-normal !py-2.5 !pr-2.5 !pl-3">
                <code className="min-w-0 flex-1 break-all text-ink">
                  {hubAddCommand(location, name)}
                </code>
                <Button variant="secondary" size="sm" onClick={() => void copy()}>
                  {copied ? <Check size={13} className="text-ok" /> : <Copy size={13} />}
                  {t('common.copy')}
                </Button>
              </div>
              <span className="text-[12.5px] text-ink-3">{t('hubs.share.step3Hint')}</span>
            </>
          ) : (
            <span className="text-[12.5px] text-ink-3">{t('hubs.share.needURL')}</span>
          )}
        </Step>
      </div>
      <div className="df">
        <Button onClick={() => void close()} loading={busy}>
          {t('hubs.edit.done')}
        </Button>
      </div>
    </DialogShell>
  );
}
