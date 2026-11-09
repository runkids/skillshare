/** A finding parsed from the text of a blocked install or update, or from an install warning. */
export type Finding = { severity: string; message: string; where?: string; snippet?: string; name?: string; lowered?: boolean };

/** ss-sev modifier per severity. */
export const SEV: Record<string, string> = { CRITICAL: 'c', HIGH: 'h', MEDIUM: 'md', LOW: 'l', INFO: 'n' };

// Matches `HIGH: message (file:line)` from blocked installs and `name: audit HIGH: message (file:line)` from warnings.
const FINDING = /(?:^|: )(?:audit )?(CRITICAL|HIGH|MEDIUM|LOW|INFO): (.*?)(?: \(([^()]+:\d+)\))?$/;
// The Go side appends this note to a finding lowered on an inert Python line; the dashboard shows its own translation.
const LOWERED = / \(lowered from CRITICAL[^()]*\)$/;

/** A line of an install's warnings that is not a finding. Batch installs prefix it with the skill name. */
export type StatusNote = { name?: string; text: string; key?: 'forced' | 'accepted' | 'belowThreshold' | 'skipped'; value?: string };

// The audit status lines internal/install writes, so the dashboard can show its own translation.
const STATUS: [RegExp, NonNullable<StatusNote['key']>][] = [
  [/^(?:(.+?): )?audit findings at\/above block threshold \((\w+)\); proceeding due to --force$/, 'forced'],
  [/^(?:(.+?): )?recorded (\d+) accepted finding\(s\); future updates won't block on them$/, 'accepted'],
  [/^(?:(.+?): )?audit findings detected, but none at\/above block threshold \((\w+)\)$/, 'belowThreshold'],
  [/^(?:(.+?): )?audit skipped \(--skip-audit\)$/, 'skipped'],
];

export function statusNote(line: string): StatusNote {
  for (const [re, key] of STATUS) {
    const m = line.match(re);
    if (m) return { name: m[1], text: line, key, value: m[2] };
  }
  return { text: line };
}

export const isAuditBlock = (msg: string) => msg.includes('security audit failed');
export const thresholdOf = (msg: string) => msg.match(/at\/above (\w+)/)?.[1] ?? '';

/** Audit lines, each optionally followed by a Go-quoted snippet line. `keepOther` keeps lines that are not findings. */
export function parseFindings(lines: string[], keepOther: boolean): Finding[] {
  const out: Finding[] = [];
  for (const raw of lines) {
    const line = raw.trim();
    const m = line.match(FINDING);
    if (m) {
      const lowered = LOWERED.test(m[2]);
      const message = lowered ? m[2].replace(LOWERED, '') : m[2];
      out.push({ severity: m[1], message, where: m[3], name: line.slice(0, m.index) || undefined, ...(lowered && { lowered }) });
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
