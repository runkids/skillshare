import { ChevronLeft, ChevronRight } from 'lucide-react';
import SegmentedControl from './SegmentedControl';
import { radius } from '../design';

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

export default function Pagination({
  page,
  totalPages,
  onPageChange,
  rangeText,
  pageSize,
}: PaginationProps) {
  return (
    <div className="flex items-center justify-between pt-4 mt-4 border-t-2 border-dashed border-muted">
      <div className="flex items-center gap-2 text-sm text-pencil-light">
        {pageSize && (
          <>
            <span>Show</span>
            <SegmentedControl
              value={String(pageSize.value)}
              onChange={(v) => pageSize.onChange(Number(v))}
              options={pageSize.options.map((s) => ({ value: String(s), label: String(s) }))}
              size="sm"
            />
          </>
        )}
        {rangeText && <span className="ml-1">{rangeText}</span>}
      </div>

      <div className="flex items-center gap-1">
        <button
          onClick={() => onPageChange(Math.max(0, page - 1))}
          disabled={page === 0}
          className={`p-1.5 border-2 transition-all duration-150 cursor-pointer ${
            page === 0
              ? 'border-transparent text-muted-dark cursor-not-allowed'
              : 'border-transparent text-pencil hover:bg-paper-warm hover:border-muted-dark'
          }`}
          style={{ borderRadius: radius.sm }}
        >
          <ChevronLeft size={20} />
        </button>
        <span className="text-sm text-pencil px-2">
          {page + 1} / {totalPages}
        </span>
        <button
          onClick={() => onPageChange(Math.min(totalPages - 1, page + 1))}
          disabled={page >= totalPages - 1}
          className={`p-1.5 border-2 transition-all duration-150 cursor-pointer ${
            page >= totalPages - 1
              ? 'border-transparent text-muted-dark cursor-not-allowed'
              : 'border-transparent text-pencil hover:bg-paper-warm hover:border-muted-dark'
          }`}
          style={{ borderRadius: radius.sm }}
        >
          <ChevronRight size={20} />
        </button>
      </div>
    </div>
  );
}
