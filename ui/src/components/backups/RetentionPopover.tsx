import { useEffect, useRef, useState } from 'react';
import { SlidersHorizontal } from 'lucide-react';
import type { BackupRetention } from '../../api/client';
import Button from '../Button';
import { useI18n, useT } from '../../i18n';
import { retentionSummary } from './backupView';

const COUNTS = [5, 10, 20, 50, 0];
const SIZES = [100, 500, 1000, 5000, 0];

/** The choices, with a value set by hand in config.yaml kept in its place. */
const choices = (list: number[], current: number) => (list.includes(current) ? list : [...list.slice(0, -1), current].sort((a, b) => a - b).concat(0));

/** The retention summary in the backup footer; in global mode it opens a panel to change the count and size limits. */
export default function RetentionPopover({ retention, editable, saving, onSave }: {
  retention: BackupRetention;
  editable: boolean;
  saving: boolean;
  onSave: (limits: { maxCount: number; maxSizeMB: number }) => Promise<unknown>;
}) {
  const t = useT();
  const { locale } = useI18n();
  const [open, setOpen] = useState(false);
  const [count, setCount] = useState(retention.maxCount);
  const [size, setSize] = useState(retention.maxSizeMB);
  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  const num = (n: number) => new Intl.NumberFormat(locale).format(n);
  const countLabel = (n: number) => (n === 0 ? t('backup.retention.unlimited') : num(n));
  const sizeLabel = (n: number) => (n === 0 ? t('backup.retention.unlimited') : `${num(n)} MB`);
  const summary = retentionSummary(t, retention, locale);

  const close = () => { setOpen(false); triggerRef.current?.focus(); };
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => { if (!containerRef.current?.contains(e.target as Node)) setOpen(false); };
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') { setOpen(false); triggerRef.current?.focus(); } };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onKey); };
  }, [open]);

  if (!editable) return <span>{summary}</span>;

  const group = (label: string, values: number[], value: number, set: (n: number) => void, text: (n: number) => string) => (
    <div className="flex flex-col gap-1.5">
      <span className="text-xs font-semibold text-ink-3">{label}</span>
      <div role="radiogroup" aria-label={label} className="ss-seg !flex">
        {values.map((n) => (
          <button key={n} type="button" role="radio" aria-checked={value === n} className={`flex-1 justify-center whitespace-nowrap ${value === n ? 'on' : ''}`} onClick={() => set(n)}>{text(n)}</button>
        ))}
      </div>
    </div>
  );

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={triggerRef}
        type="button"
        className="ss-btn sm secondary"
        aria-expanded={open}
        onClick={() => { setCount(retention.maxCount); setSize(retention.maxSizeMB); setOpen(!open); }}
      >
        <SlidersHorizontal size={14} />{summary}
      </button>
      {open && (
        <div role="dialog" aria-label={t('backup.retention.title')} className="ss-menu absolute right-0 top-full z-50 mt-2 !w-[480px] gap-4 !p-4 animate-dropdown-in">
          <div className="flex flex-col gap-1">
            <span className="text-sm font-semibold text-ink">{t('backup.retention.title')}</span>
            <span className="text-xs text-ink-3">{t('backup.retention.hint')}</span>
          </div>
          {group(t('backup.retention.maxCount'), choices(COUNTS, retention.maxCount), count, setCount, countLabel)}
          {group(t('backup.retention.maxSize'), choices(SIZES, retention.maxSizeMB), size, setSize, sizeLabel)}
          <div className="flex items-center justify-between text-[13px] text-ink-2">
            <span>{t('backup.retention.maxAge')}</span>
            <span>{t('backup.retention.days', { days: num(retention.maxAgeDays) })}</span>
          </div>
          <div className="ss-note !py-2 text-xs">{t('backup.retention.applies')}</div>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={close} disabled={saving}>{t('common.cancel')}</Button>
            <Button variant="primary" size="sm" loading={saving} onClick={() => { void onSave({ maxCount: count, maxSizeMB: size }).then(close, () => undefined); }}>{t('common.save')}</Button>
          </div>
        </div>
      )}
    </div>
  );
}
