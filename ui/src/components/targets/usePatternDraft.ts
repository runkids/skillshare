import { useState, type KeyboardEvent } from 'react';

/** The typing half of a pattern field: Enter, comma or blur adds; Backspace on an empty field removes the last. */
export function usePatternDraft(patterns: string[], onChange: (patterns: string[]) => void) {
  const [draft, setDraft] = useState('');
  const commit = () => {
    const value = draft.trim();
    if (value && !patterns.includes(value)) onChange([...patterns, value]);
    setDraft('');
  };
  return {
    value: draft,
    onChange: (e: { target: { value: string } }) => setDraft(e.target.value),
    onKeyDown: (e: KeyboardEvent<HTMLInputElement>) => {
      if ((e.key === 'Enter' || e.key === ',') && draft.trim()) {
        e.preventDefault();
        commit();
      } else if (e.key === 'Backspace' && !draft && patterns.length) {
        onChange(patterns.slice(0, -1));
      }
    },
    onBlur: commit,
  };
}
