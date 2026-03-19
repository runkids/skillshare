import { useState, useEffect, useRef } from 'react';

export interface DiffLine {
  type: 'add' | 'remove';
  content: string;
  oldLine?: number;
  newLine?: number;
}

export interface DiffResult {
  lines: DiffLine[];
  changeCount: number;
}

export function computeLineDiff(oldStr: string, newStr: string): DiffResult {
  if (oldStr === newStr) return { lines: [], changeCount: 0 };

  const oldLines = oldStr.split('\n');
  const newLines = newStr.split('\n');
  const lcs = buildLCS(oldLines, newLines);
  const lines: DiffLine[] = [];
  let oi = 0, ni = 0, li = 0;

  while (oi < oldLines.length || ni < newLines.length) {
    if (li < lcs.length && oi < oldLines.length && ni < newLines.length && oldLines[oi] === lcs[li] && newLines[ni] === lcs[li]) {
      oi++; ni++; li++;
    } else if (oi < oldLines.length && (li >= lcs.length || oldLines[oi] !== lcs[li])) {
      lines.push({ type: 'remove', content: oldLines[oi], oldLine: oi + 1 });
      oi++;
    } else if (ni < newLines.length && (li >= lcs.length || newLines[ni] !== lcs[li])) {
      lines.push({ type: 'add', content: newLines[ni], newLine: ni + 1 });
      ni++;
    }
  }

  return { lines, changeCount: lines.length };
}

function buildLCS(a: string[], b: string[]): string[] {
  const m = a.length, n = b.length;
  const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0));
  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      dp[i][j] = a[i - 1] === b[j - 1] ? dp[i - 1][j - 1] + 1 : Math.max(dp[i - 1][j], dp[i][j - 1]);
    }
  }
  const result: string[] = [];
  let i = m, j = n;
  while (i > 0 && j > 0) {
    if (a[i - 1] === b[j - 1]) { result.unshift(a[i - 1]); i--; j--; }
    else if (dp[i - 1][j] > dp[i][j - 1]) { i--; }
    else { j--; }
  }
  return result;
}

export function useLineDiff(oldStr: string, newStr: string, enabled: boolean = true) {
  const [diff, setDiff] = useState<DiffResult>({ lines: [], changeCount: 0 });
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    if (!enabled) return;
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      setDiff(computeLineDiff(oldStr, newStr));
    }, 500);
    return () => clearTimeout(timerRef.current);
  }, [oldStr, newStr, enabled]);

  const changeCount = !enabled || oldStr === newStr ? 0 : computeSimpleChangeCount(oldStr, newStr);
  return { diff, changeCount };
}

export function computeSimpleChangeCount(a: string, b: string): number {
  const aLines = a.split('\n');
  const bLines = b.split('\n');
  let changes = 0;
  const maxLen = Math.max(aLines.length, bLines.length);
  for (let i = 0; i < maxLen; i++) {
    if (aLines[i] !== bLines[i]) changes++;
  }
  return changes;
}
