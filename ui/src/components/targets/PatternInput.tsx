import { X } from 'lucide-react';
import { useT } from '../../i18n';
import { usePatternDraft } from './usePatternDraft';

/** Include/exclude patterns as removable tags, typed into one field. */
export default function PatternInput({
  id,
  patterns,
  onChange,
  disabled,
  placeholder,
}: {
  id: string;
  patterns: string[];
  onChange: (patterns: string[]) => void;
  disabled?: boolean;
  placeholder?: string;
}) {
  const t = useT();
  const field = usePatternDraft(patterns, onChange);
  return (
    <span className="ss-inp !h-auto min-h-[38px] flex-wrap !gap-1.5 !py-1.5">
      {patterns.map((p) => (
        <span key={p} className="ss-tag !h-[22px] !text-[12px]">
          {p}
          <button
            type="button"
            className="text-ink-3 hover:text-ink"
            aria-label={t('targetDetail.removePattern', { pattern: p })}
            onClick={() => onChange(patterns.filter((x) => x !== p))}
            disabled={disabled}
          >
            <X size={11} />
          </button>
        </span>
      ))}
      <input
        id={id}
        className="min-w-[160px] !h-[24px]"
        {...field}
        placeholder={placeholder ?? t('targetDetail.addPattern')}
        disabled={disabled}
      />
    </span>
  );
}
