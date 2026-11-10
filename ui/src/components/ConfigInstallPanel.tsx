import type { CSSProperties, ReactNode } from 'react';
import { Check, CircleAlert, Download, FolderDown, GitBranch, Loader2, Puzzle, X } from 'lucide-react';
import Button from './Button';
import { parseRemoteURL } from '../lib/parseRemoteURL';
import { plural, useT } from '../i18n';
import { useInstallFromConfig } from '../hooks/useInstallFromConfig';

/** Rows finish one after another, so results read top to bottom. */
const stagger = (i: number): CSSProperties => ({ animationDelay: `${i * 50}ms`, animationFillMode: 'both' });

/** Install dialog section: what config records but the disk lacks, installed in one go. */
export default function ConfigInstallPanel() {
  const t = useT();
  const { entries, file, phase, done, failed, summary, contents, error, run, dismiss } = useInstallFromConfig();
  if (entries.length === 0) return null;

  let icon = <FolderDown size={16} />;
  let tone = '';
  let title = t(plural('install.fromConfig.title', entries.length), { count: entries.length, file });
  let sub = t('install.fromConfig.hint', { items: contents });
  let action: ReactNode = <Button variant="primary" size="sm" onClick={run}><Download size={14} />{t('install.fromConfig.installAll')}</Button>;
  if (phase === 'running') {
    icon = <Loader2 size={16} className="animate-spin" />;
    title = t('install.fromConfig.installing', { file });
    sub = t('install.audited');
    action = null;
  } else if (done) {
    icon = failed.size ? <CircleAlert size={16} className="ss-pop" /> : <Check size={16} strokeWidth={2.6} className="ss-pop" />;
    tone = failed.size ? 'warn' : 'ok';
    title = summary;
    sub = t(failed.size ? 'install.fromConfig.failedHint' : 'install.fromConfig.syncHint', { file });
    action = failed.size
      ? <Button variant="secondary" size="sm" onClick={run}>{t('install.preview.retry')}</Button>
      : <Button variant="secondary" size="sm" onClick={dismiss}>{t('install.done')}</Button>;
  } else if (error) {
    tone = 'bad';
    icon = <CircleAlert size={16} />;
    sub = error;
  }

  return (
    <div className={`ss-collapse ${phase === 'closing' ? 'shut' : ''}`}>
      <section className="flex flex-col gap-2" aria-live="polite">
        <div className={`ss-note live items-center ${tone}`}>
          {icon}
          <div key={phase} className="flex flex-1 min-w-0 flex-col gap-0.5 animate-fade-in">
            <b>{title}</b>
            <span className="text-ink-2">{sub}</span>
          </div>
          {action}
        </div>
        {phase === 'running' && <div className="ss-bar indet animate-fade-in" role="progressbar" aria-label={title}><span /></div>}
        <div className="ss-list !shadow-none">
          <div className="max-h-[164px] overflow-y-auto">
            {entries.map((e, i) => {
              const bad = done && failed.has(e.name);
              const row = phase === 'running' ? 'updating' : done && !bad ? 'flash' : '';
              return (
                <div key={e.name} className={`ss-r relative !min-h-10 ${row}`} style={done ? stagger(i) : undefined}>
                  {e.tracked
                    ? <span className="w-[22px] grid place-items-center text-ink-2"><GitBranch size={15} /></span>
                    : <span className="ss-cat sm skill"><Puzzle size={13} /></span>}
                  <span className="nm m truncate shrink-0 max-w-[55%]">{e.name}</span>
                  {e.tracked && <span className="ss-tag">tracked</span>}
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3">{parseRemoteURL(e.source)?.ownerRepo ?? e.source}</span>
                  <span className="w-5 grid place-items-center">
                    {phase === 'running' && <Loader2 size={14} className="animate-spin text-ink-3" />}
                    {done && (bad
                      ? <X size={15} strokeWidth={2.6} className="ss-pop text-bad" style={stagger(i)} />
                      : <Check size={15} strokeWidth={2.6} className="ss-pop text-ok" style={stagger(i)} />)}
                  </span>
                  {phase === 'running' && <span className="ss-rowbar" aria-hidden />}
                </div>
              );
            })}
          </div>
        </div>
      </section>
    </div>
  );
}
