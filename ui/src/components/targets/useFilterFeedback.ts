import { useEffect, useRef, useState } from 'react';
import type { SyncMatrixEntry } from '../../api/client';
import { previewChange } from './targetView';

const NONE: ReadonlySet<string> = new Set();
// How long a flipped row stays tinted, and how long the card says what the change did.
const TINT_MS = 700;
const RESULT_MS = 2000;

export interface FilterFeedback {
  /** The rule under the pointer or focus; its rows light up. */
  lit: string;
  setLit: (pattern: string) => void;
  /** Rows the last rule change flipped. */
  changed: ReadonlySet<string>;
  /** What the last rule change did; kept while it fades out. */
  result: { skipped: number; synced: number; shown: boolean } | null;
}

/**
 * Ties the filter card to the preview, which sit apart on the page: the rule under the pointer, the rows the
 * last rule change flipped, and how many. `loadedKey` names the filters the shown rows were computed for,
 * null while they are stale or this kind is off; a new key is a rule change, so it is compared with the last.
 */
export function useFilterFeedback(
  entries: SyncMatrixEntry[],
  loadedKey: string | null,
): FilterFeedback {
  const [lit, setLit] = useState('');
  const [changed, setChanged] = useState(NONE);
  const [result, setResult] = useState<FilterFeedback['result']>(null);
  const last = useRef<{ key: string; entries: SyncMatrixEntry[] } | null>(null);
  const timers = useRef<number[]>([]);
  useEffect(() => () => timers.current.forEach(clearTimeout), []);
  useEffect(() => {
    // While the next preview loads, the last loaded one stays the one to compare with.
    if (loadedKey === null) return;
    const before = last.current;
    last.current = { key: loadedKey, entries };
    if (!before || before.key === loadedKey) return;
    const change = previewChange(before.entries, entries);
    timers.current.forEach(clearTimeout);
    setChanged(new Set(change.changed));
    setResult({ skipped: change.skipped, synced: change.synced, shown: true });
    timers.current = [
      window.setTimeout(() => setChanged(NONE), TINT_MS),
      window.setTimeout(() => setResult((r) => r && { ...r, shown: false }), RESULT_MS),
    ];
  }, [loadedKey, entries]);
  return { lit, setLit, changed, result };
}
