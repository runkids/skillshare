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

const GO_SPECIFIC_RE = /\\x\{|^\(\?[a-zA-Z]+\)/;

function isGoSpecificRegex(pattern: string): boolean {
  return GO_SPECIFIC_RE.test(pattern);
}

export function computeRegexMatches(
  pattern: string,
  testInput: string,
  excludePattern?: string,
): RegexMatchResult {
  if (!pattern) return { matches: [], error: null, isGoSpecific: false };

  if (isGoSpecificRegex(pattern)) {
    return { matches: [], error: 'Go-specific regex syntax — cannot test in browser', isGoSpecific: true };
  }

  let re: RegExp;
  try {
    re = new RegExp(pattern);
  } catch (e) {
    return { matches: [], error: (e as Error).message, isGoSpecific: false };
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
