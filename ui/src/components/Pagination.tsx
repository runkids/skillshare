import { ChevronLeft, ChevronRight } from 'lucide-react';
import SegmentedControl from './SegmentedControl';
import { useT } from '../i18n';

interface PageSizeConfig {
  value: number;
  options: readonly number[];
  onChange: (size: number) => void;
}

interface PaginationProps {
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  /** Range text, e.g. "1–25 of 100" */
  rangeText?: string;
  /** Page size options and current selection */
  pageSize?: PageSizeConfig;
}

export default function Pagination({ page, totalPages, onPageChange, rangeText, pageSize }: PaginationProps) {
  const t = useT();
  return (
    <div className="flex items-center justify-between pt-4 mt-4 [border-top:var(--sep)]">
      <div className="flex items-center gap-2 text-[13px] text-ink-2">
        {pageSize && (
          <>
            <span>{t('pagination.show')}</span>
            <SegmentedControl
              value={String(pageSize.value)}
              onChange={(v) => pageSize.onChange(Number(v))}
              options={pageSize.options.map((s) => ({ value: String(s), label: String(s) }))}
            />
          </>
        )}
        {rangeText && <span className="ml-1">{rangeText}</span>}
      </div>

      <div className="ss-pager">
        <button type="button" className="ss-ib" onClick={() => onPageChange(Math.max(0, page - 1))} disabled={page === 0} aria-label={t('pagination.previous')}>
          <ChevronLeft size={16} />
        </button>
        <span className="!cursor-default font-mono">{page + 1} / {totalPages}</span>
        <button type="button" className="ss-ib" onClick={() => onPageChange(Math.min(totalPages - 1, page + 1))} disabled={page >= totalPages - 1} aria-label={t('pagination.next')}>
          <ChevronRight size={16} />
        </button>
      </div>
    </div>
  );
}
