import { useState } from 'react';
import { X } from 'lucide-react';
import { useT } from '../../i18n';

/** Include/exclude patterns as removable tags. Enter, comma or blur adds; Backspace on an empty field removes the last. */
export default function PatternInput({ id, patterns, onChange, disabled }: {
  id: string;
  patterns: string[];
  onChange: (patterns: string[]) => void;
  disabled?: boolean;
}) {
  const t = useT();
  const [draft, setDraft] = useState('');
  const commit = () => {
    const value = draft.trim();
    if (value && !patterns.includes(value)) onChange([...patterns, value]);
    setDraft('');
  };
  return (
    <span className="ss-inp !h-auto min-h-[38px] flex-wrap !gap-1.5 !py-1.5">
      {patterns.map((p) => (
        <span key={p} className="ss-tag !h-[22px] !text-[12px]">
          {p}
          <button type="button" className="text-ink-3 hover:text-ink" aria-label={t('targetDetail.removePattern', { pattern: p })} onClick={() => onChange(patterns.filter((x) => x !== p))} disabled={disabled}>
            <X size={11} />
          </button>
        </span>
      ))}
      <input
        id={id}
        className="min-w-[160px] !h-[24px]"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if ((e.key === 'Enter' || e.key === ',') && draft.trim()) {
            e.preventDefault();
            commit();
          } else if (e.key === 'Backspace' && !draft && patterns.length) {
            onChange(patterns.slice(0, -1));
          }
        }}
        onBlur={commit}
        placeholder={t('targetDetail.addPattern')}
        disabled={disabled}
      />
    </span>
  );
}
