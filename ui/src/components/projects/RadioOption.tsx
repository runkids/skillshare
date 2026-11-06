import type { ReactNode } from 'react';

interface Props {
  checked: boolean;
  title: ReactNode;
  hint: ReactNode;
  onSelect: () => void;
  disabled?: boolean;
}

/** One choice of a radio group, drawn as an `ss-pick`. The group supplies `role="radiogroup"`. */
export default function RadioOption({ checked, title, hint, onSelect, disabled }: Props) {
  return (
    <button type="button" role="radio" aria-checked={checked} className={`ss-pick items-start text-left ${checked ? 'on' : ''}`} onClick={onSelect} disabled={disabled}>
      <span className={`ss-chk rad mt-0.5 ${checked ? 'on' : ''}`} />
      <span className="flex flex-col gap-0.5">
        <span className="font-semibold">{title}</span>
        <span className="text-[13px] text-ink-2">{hint}</span>
      </span>
    </button>
  );
}
