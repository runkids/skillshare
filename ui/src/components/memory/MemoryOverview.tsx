import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { BookOpen, ChevronRight, FileText, Plus } from 'lucide-react';
import { api } from '../../api/client';
import type { MemoryNotesResponse } from '../../api/memory';
import { formatDateTime, formatRelativeTime, plural, useI18n, useT } from '../../i18n';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import { invalidate } from '../../lib/queryEvents';
import Button from '../Button';
import EmptyState from '../EmptyState';
import Skeleton from '../Skeleton';
import { useToast } from '../Toast';
import { updatedNotes } from './memoryView';
import MemoryTools from './MemoryTools';

export function MemorySummary({ data }: { data: MemoryNotesResponse }) {
  const t = useT();
  const { locale } = useI18n();
  const { data: guidance } = useQuery({ queryKey: queryKeys.memory.guidance, queryFn: api.getMemoryGuidance, enabled: data.initialized, staleTime: staleTimes.extras });
  const latest = updatedNotes(data.notes)[0]?.modified_at;
  return <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[13px] text-ink-3">
    <span>{t(plural('memory.noteCount', data.notes.length), { count: data.notes.length })}</span>
    {latest && <span title={formatDateTime(latest, locale)}>{t('memory.updated', { time: formatRelativeTime(latest, locale) })}</span>}
    {guidance && <span>{t('memory.connectedCount', { count: guidance.targets.filter((target) => target.state === 'configured').length, total: guidance.targets.length })}</span>}
  </div>;
}

export default function MemoryOverview() {
  const t = useT();
  const { locale } = useI18n();
  const client = useQueryClient();
  const { toast } = useToast();
  const [busy, setBusy] = useState(false);
  const { data, isPending, error } = useQuery({ queryKey: queryKeys.memory.overview, queryFn: api.getMemoryOverview, staleTime: staleTimes.extras });
  const { data: guidance, error: guidanceError } = useQuery({ queryKey: queryKeys.memory.guidance, queryFn: api.getMemoryGuidance, enabled: !!data?.initialized, staleTime: staleTimes.extras });
  const notes = data?.notes ?? [];
  const onlyStarters = notes.length > 0 && notes.every((note) => note.starter);
  const initialize = async () => {
    setBusy(true);
    try { await api.initMemory(); await invalidate(client, 'memoryNotesChanged'); }
    catch (err) { toast((err as Error).message, 'error'); }
    finally { setBusy(false); }
  };
  return <section aria-label={t('memory.title')}>
    <div className="ss-sec">
      <h2>{t('memory.title')}</h2>
      {data && <span className="text-[13px] text-ink-3">{t(plural('memory.noteCount', data.count), { count: data.count })}</span>}
      {data?.initialized && <MemoryTools root={data.root} disabled={false} />}
      <Link to="/extras?tab=memory" className="more">{t('memory.open')}</Link>
    </div>
    {isPending ? <Skeleton variant="card" /> : error ? <div className="ss-note bad">{error.message}</div> : data && <>
      {!data.initialized && data.count === 0 ? <EmptyState icon={BookOpen} title={t('memory.empty')} description={t('memory.startHint')}
        action={<Button variant="primary" loading={busy} onClick={() => void initialize()}><Plus size={15} />{t('memory.init')}</Button>} /> : <div className="ss-list">
        {onlyStarters ? <div className="ss-r text-[13px] text-ink-2">{t('memory.starterHint')}</div>
          : <div className="ss-lh">{t('memory.recentlyUpdated')}</div>}
        {notes.length === 0 && <div className="ss-r text-[13px] text-ink-3">{t(data.count ? 'memory.invalid' : 'memory.empty')}</div>}
        {notes.map((note) => <Link key={note.path} className="ss-r link !min-h-[64px] flex-wrap" to={`/extras?tab=memory&note=${encodeURIComponent(note.path)}`}>
          <span className="ss-cat extra sm"><FileText size={16} /></span>
          <span className="min-w-0 flex-1"><span className="block truncate font-mono text-[13px] font-semibold">{note.path}</span><span className="block truncate text-[13px] text-ink-3">{note.title}</span></span>
          {!note.starter && note.modified_at && <time dateTime={note.modified_at} title={formatDateTime(note.modified_at, locale)} className="text-[12.5px] text-ink-3">{formatRelativeTime(note.modified_at, locale)}</time>}
          <ChevronRight size={15} className="text-ink-3" />
        </Link>)}
        <div className="ss-r !min-h-[52px] flex-wrap bg-sunken text-[12.5px] gap-y-2">
          {guidance && <span className={`ss-st ${guidance.targets.some((target) => target.state === 'configured') ? 'ok' : 'off'}`}>{t('memory.connectedCount', { count: guidance.targets.filter((target) => target.state === 'configured').length, total: guidance.targets.length })}</span>}
          <span className="min-w-0 flex-1 text-ink-3">{guidanceError ? guidanceError.message : t(data.initialized ? 'memory.configuredHint' : 'memory.startHint')}</span>
          {!data.initialized && <Button variant="primary" loading={busy} onClick={() => void initialize()}><Plus size={15} />{t('memory.init')}</Button>}
        </div>
      </div>}
    </>}
  </section>;
}
