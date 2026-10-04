import { AlertTriangle } from 'lucide-react';
import type { SkillfollowResponse } from '../../api/client';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';

// followed and not-link pass doctor; every other declared state pauses cleanup.
const stateTone = (state: string) => (state === 'followed' ? 'ok' : state === 'not-link' ? '' : 'warn');

/** The declared entries with the states discovery gave them, read-only. */
export function SkillfollowStates({ data }: { data?: SkillfollowResponse }) {
  const t = useT();
  const entries = data?.entries ?? [];
  // An entry's state warning repeats its row, so only the others (such as
  // rejected declaration lines) are listed with the prune pauses.
  const rowWarnings = new Set(entries.map((e) => `${e.name}: ${e.state}: ${e.reason}`));
  const notes = [...(data?.warnings ?? []).filter((w) => !rowWarnings.has(w)), ...(data?.prune_paused ?? [])];
  return (
    <section className="flex flex-col gap-3">
      <div className="ss-sec !mb-0">
        <h2>{t('config.skillfollow.entries')}</h2>
        <span className="ss-cnt">{entries.length}</span>
      </div>
      {entries.length === 0 ? (
        <p className="ss-empty">{t('config.skillfollow.noEntries')}</p>
      ) : (
        <div className="ss-tbl">
          <table>
            <thead>
              <tr>
                <th>{t('config.skillfollow.col.name')}</th>
                <th>{t('config.skillfollow.col.state')}</th>
                <th>{t('config.skillfollow.col.target')}</th>
                <th>{t('config.skillfollow.col.reason')}</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => (
                <tr key={entry.name}>
                  <td className="whitespace-nowrap font-mono">{entry.name}</td>
                  <td><span className={`ss-tag ${stateTone(entry.state)}`}>{entry.state}</span></td>
                  <td className="font-mono text-ink-2 [overflow-wrap:anywhere]">{entry.resolved_target ? shortenHome(entry.resolved_target) : '—'}</td>
                  <td className="text-ink-2 [overflow-wrap:anywhere]">{entry.reason}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {notes.length > 0 && (
        <div className="ss-note warn">
          <AlertTriangle size={16} className="mt-0.5 shrink-0" />
          <ul className="flex min-w-0 flex-1 flex-col gap-1 [overflow-wrap:anywhere]">
            {notes.map((note) => <li key={note}>{note}</li>)}
          </ul>
        </div>
      )}
    </section>
  );
}

/** The panel beside the editor: what the file holds and what saving does not do. */
export function SkillfollowPanel() {
  const t = useT();
  return (
    <div className="ss-box flex flex-col gap-3 !p-3.5 text-[13px] text-ink-2">
      <p>{t('config.skillfollow.localHint')}</p>
      <p>{t('config.skillfollow.ignoreReminder')}</p>
      <p className="text-xs text-ink-3">{t('config.skillfollow.applyNote')}</p>
    </div>
  );
}
