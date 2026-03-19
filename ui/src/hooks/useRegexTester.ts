import { useState, useEffect, useRef } from 'react';

export interface LineMatch {
  line: number;
  content: string;
  matched: boolean;
  matchStart?: number;
  matchEnd?: number;
  excluded: boolean;
}

export interface RegexMatchResult {
  matches: LineMatch[];
  error: string | null;
  isGoSpecific: boolean;
}

// Truly Go-only syntax that can't be converted to JS
const GO_ONLY_RE = /\\x\{/;

/** Try to convert Go inline flags to JS RegExp flags.
 *  (?i)pattern → new RegExp("pattern", "i")
 *  (?im)pattern → Go-specific (m means different things in Go vs JS for \b)
 *  Returns { pattern, flags } or null if unconvertible. */
function convertGoFlags(pattern: string): { pattern: string; flags: string } | null {
  const m = pattern.match(/^\(\?([imsu]+)\)(.*)/s);
  if (!m) return null;
  const goFlags = m[1];
  const rest = m[2];
  // Only convert single 'i' flag — 'm' and 's' have different semantics in Go
  if (goFlags === 'i') return { pattern: rest, flags: 'i' };
  return null; // multi-flag or non-i flags → can't safely convert
}

export function computeRegexMatches(
  pattern: string,
  testInput: string,
  excludePattern?: string,
): RegexMatchResult {
  if (!pattern) return { matches: [], error: null, isGoSpecific: false };

  if (GO_ONLY_RE.test(pattern)) {
    return { matches: [], error: 'Go-specific regex syntax — cannot test in browser', isGoSpecific: true };
  }

  let re: RegExp;
  try {
    // Try direct compilation first
    re = new RegExp(pattern);
  } catch {
    // Try converting Go inline flags (e.g. (?i)pattern → /pattern/i)
    const converted = convertGoFlags(pattern);
    if (converted) {
      try {
        re = new RegExp(converted.pattern, converted.flags);
      } catch (e2) {
        return { matches: [], error: (e2 as Error).message, isGoSpecific: false };
      }
    } else {
      // Check if it looks like Go inline flags we can't convert
      if (/^\(\?[a-zA-Z]+\)/.test(pattern)) {
        return { matches: [], error: 'Go-specific regex syntax — cannot test in browser', isGoSpecific: true };
      }
      return { matches: [], error: 'Invalid regex', isGoSpecific: false };
    }
  }

  let excludeRe: RegExp | null = null;
  if (excludePattern) {
    try { excludeRe = new RegExp(excludePattern); } catch { /* ignore */ }
  }

  const lines = testInput.split('\n');
  const matches: LineMatch[] = lines.map((content, i) => {
    const match = re.exec(content);
    if (!match) return { line: i + 1, content, matched: false, excluded: false };
    const excluded = excludeRe ? excludeRe.test(content) : false;
    return {
      line: i + 1, content, matched: true,
      matchStart: match.index, matchEnd: match.index + match[0].length,
      excluded,
    };
  });

  return { matches, error: null, isGoSpecific: false };
}

export function useRegexTester(pattern: string, testInput: string, excludePattern?: string) {
  const [result, setResult] = useState<RegexMatchResult>({ matches: [], error: null, isGoSpecific: false });
  const timerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      setResult(computeRegexMatches(pattern, testInput, excludePattern));
    }, 100);
    return () => clearTimeout(timerRef.current);
  }, [pattern, testInput, excludePattern]);

  return result;
}
