import { useState } from 'react';

/**
 * Progress of a job on its status card. A faint runner shows work is going on even before the
 * first item finishes; without a `value` the runner is all there is. When the job ends the fill
 * is given time to arrive, then the bar fades and its slot closes, so it never needs unmounting.
 * `className` sets `--bar-gap` to the parent's gap, which the closed slot then takes back.
 */
export default function ProgressBar({ active, value, max = 100, label, className = '' }: {
  active: boolean; value?: number; max?: number; label?: string; className?: string;
}) {
  // A new run gets a new bar, so its fill starts empty instead of sliding back from the last run.
  const [run, setRun] = useState(0);
  const [was, setWas] = useState(active);
  if (was !== active) {
    setWas(active);
    if (active) setRun(run + 1);
  }
  // What kind of bar it was is kept while it leaves; a caller often drops `value` in the same render.
  const [counted, setCounted] = useState(value !== undefined);
  if (active && counted !== (value !== undefined)) setCounted(value !== undefined);
  const part = counted && max > 0 ? Math.min(Math.max((value ?? 0) / max, 0), 1) : 0;
  return (
    <span className={`ss-barslot ${active ? '' : 'shut'} ${className}`} aria-hidden={!active || undefined}>
      <span>
        <span key={run} className={`ss-bar ${counted ? '' : 'indet'} ${active ? '' : 'end'}`} role="progressbar"
          aria-label={label} aria-valuemin={0} aria-valuemax={max} aria-valuenow={counted ? value : undefined}>
          {counted && <i style={{ transform: `translateX(${(part - 1) * 100}%)` }} />}
        </span>
      </span>
    </span>
  );
}
