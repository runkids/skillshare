const CONTEXT = 3;

const changed = (line: string) => line.startsWith('+ ') || line.startsWith('- ');

/**
 * The part of a long textdiff worth reading first: each changed line with CONTEXT lines around
 * it, unchanged runs between them as one null (a gap), and at most max entries in all.
 */
export function diffExcerpt(lines: string[], max: number): (string | null)[] {
  const keep = lines.map((_, i) =>
    lines.slice(Math.max(0, i - CONTEXT), i + CONTEXT + 1).some(changed),
  );
  if (!keep.includes(true)) return lines.slice(0, max);
  const out: (string | null)[] = [];
  lines.forEach((line, i) => {
    if (keep[i]) out.push(line);
    else if (out[out.length - 1] !== null) out.push(null);
  });
  return out.slice(0, max);
}
