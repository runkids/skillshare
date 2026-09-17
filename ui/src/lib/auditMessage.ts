/** A finding parsed from the text of a blocked install or update, or from an install warning. */
export type Finding = { severity: string; message: string; where?: string; snippet?: string; name?: string };

/** ss-sev modifier per severity. */
export const SEV: Record<string, string> = { CRITICAL: 'c', HIGH: 'h', MEDIUM: 'md', LOW: 'l', INFO: 'n' };

// Matches `HIGH: message (file:line)` from blocked installs and `name: audit HIGH: message (file:line)` from warnings.
const FINDING = /(?:^|: )(?:audit )?(CRITICAL|HIGH|MEDIUM|LOW|INFO): (.*?)(?: \(([^()]+:\d+)\))?$/;

export const isAuditBlock = (msg: string) => msg.includes('security audit failed');
export const thresholdOf = (msg: string) => msg.match(/at\/above (\w+)/)?.[1] ?? '';

/** Audit lines, each optionally followed by a Go-quoted snippet line. `keepOther` keeps lines that are not findings. */
export function parseFindings(lines: string[], keepOther: boolean): Finding[] {
  const out: Finding[] = [];
  for (const raw of lines) {
    const line = raw.trim();
    const m = line.match(FINDING);
    if (m) {
      out.push({ severity: m[1], message: m[2], where: m[3], name: line.slice(0, m.index) || undefined });
    } else if (line.startsWith('"') && out.length > 0) {
      out[out.length - 1].snippet = unquote(line);
    } else if (keepOther && line) {
      out.push({ severity: '', message: line });
    }
  }
  return out;
}

function unquote(s: string) {
  try {
    const v: unknown = JSON.parse(s);
    return typeof v === 'string' ? v : s;
  } catch {
    return s;
  }
}
