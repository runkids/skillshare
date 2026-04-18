// Tiny YAML frontmatter parser/serializer — handles the field set SKILL.md cares
// about (scalars, inline arrays, multiline block scalars). Not a general-purpose
// YAML parser; fidelity is "good enough for SKILL.md round-trips".

export type FrontmatterScalar = string | number | boolean | null;
export type FrontmatterNested = Record<string, FrontmatterScalar | FrontmatterScalar[]>;
export type FrontmatterValue =
  | FrontmatterScalar
  | FrontmatterScalar[]
  | FrontmatterNested;
export type Frontmatter = Record<string, FrontmatterValue>;

export interface ParsedSkillMarkdown {
  frontmatter: Frontmatter;
  rawFrontmatter: string;
  body: string;
  hasFrontmatter: boolean;
}

const FENCE = /^---\s*(?:\r?\n|$)/;

export function parseSkillMarkdown(content: string): ParsedSkillMarkdown {
  if (!content) {
    return { frontmatter: {}, rawFrontmatter: '', body: '', hasFrontmatter: false };
  }
  if (!FENCE.test(content)) {
    return { frontmatter: {}, rawFrontmatter: '', body: content, hasFrontmatter: false };
  }
  const rest = content.replace(FENCE, '');
  const closeIdx = rest.search(/^---\s*(?:\r?\n|$)/m);
  if (closeIdx === -1) {
    return { frontmatter: {}, rawFrontmatter: '', body: content, hasFrontmatter: false };
  }
  const rawFrontmatter = rest.slice(0, closeIdx).replace(/\r?\n$/, '');
  const body = rest.slice(closeIdx).replace(/^---\s*(?:\r?\n|$)/, '');
  return {
    frontmatter: parseYaml(rawFrontmatter),
    rawFrontmatter,
    body,
    hasFrontmatter: true,
  };
}

function parseYaml(text: string): Frontmatter {
  const out: Frontmatter = {};
  const lines = text.split(/\r?\n/);
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (!line || /^\s*#/.test(line) || !line.includes(':')) {
      i++;
      continue;
    }
    const colon = line.indexOf(':');
    const key = line.slice(0, colon).trim();
    const after = line.slice(colon + 1).trim();
    if (!key) {
      i++;
      continue;
    }
    if (after === '|' || after === '>') {
      const block: string[] = [];
      i++;
      while (i < lines.length && /^\s{2,}/.test(lines[i])) {
        block.push(lines[i].replace(/^\s{2}/, ''));
        i++;
      }
      out[key] = after === '|' ? block.join('\n') : block.join(' ').trim();
      continue;
    }
    if (after.startsWith('[') && after.endsWith(']')) {
      out[key] = after
        .slice(1, -1)
        .split(',')
        .map((s) => s.trim().replace(/^['"]|['"]$/g, ''))
        .filter(Boolean);
      i++;
      continue;
    }
    if (after === '') {
      // Could be either a YAML list ("- " lines) or a nested map (indented "key:" lines).
      const peek = lines[i + 1] ?? '';
      const isListNext = /^\s*-\s+/.test(peek);
      const isNestedNext = /^\s{2,}\S+\s*:/.test(peek);
      if (isListNext) {
        const arr: string[] = [];
        i++;
        while (i < lines.length && /^\s*-\s+/.test(lines[i])) {
          arr.push(lines[i].replace(/^\s*-\s+/, '').trim().replace(/^['"]|['"]$/g, ''));
          i++;
        }
        if (arr.length) {
          out[key] = arr;
          continue;
        }
      } else if (isNestedNext) {
        const nested: FrontmatterNested = {};
        i++;
        while (i < lines.length && /^\s{2,}\S/.test(lines[i])) {
          const subLine = lines[i].replace(/^\s+/, '');
          const subColon = subLine.indexOf(':');
          if (subColon === -1) { i++; continue; }
          const subKey = subLine.slice(0, subColon).trim();
          const subAfter = subLine.slice(subColon + 1).trim();
          if (subAfter.startsWith('[') && subAfter.endsWith(']')) {
            nested[subKey] = subAfter
              .slice(1, -1)
              .split(',')
              .map((s) => s.trim().replace(/^['"]|['"]$/g, ''))
              .filter(Boolean);
          } else {
            nested[subKey] = unquote(subAfter);
          }
          i++;
        }
        if (Object.keys(nested).length) {
          out[key] = nested;
          continue;
        }
      }
      out[key] = '';
      i++;
      continue;
    }
    out[key] = unquote(after);
    i++;
  }
  return out;
}

function unquote(s: string): string | number | boolean {
  if (/^-?\d+(\.\d+)?$/.test(s)) return Number(s);
  if (s === 'true') return true;
  if (s === 'false') return false;
  if ((s.startsWith('"') && s.endsWith('"')) || (s.startsWith("'") && s.endsWith("'"))) {
    return s.slice(1, -1);
  }
  return s;
}

export function serializeFrontmatter(fm: Frontmatter, keyOrder?: string[]): string {
  const lines: string[] = ['---'];
  const written = new Set<string>();
  const push = (key: string) => {
    if (written.has(key)) return;
    const value = fm[key];
    if (value == null || value === '' || (Array.isArray(value) && value.length === 0)) return;
    if (typeof value === 'object' && !Array.isArray(value)) {
      const entries = Object.entries(value).filter(
        ([, v]) => v != null && v !== '' && !(Array.isArray(v) && v.length === 0),
      );
      if (entries.length === 0) return;
      written.add(key);
      lines.push(`${key}:`);
      for (const [subK, subV] of entries) {
        if (Array.isArray(subV)) {
          lines.push(`  ${subK}: [${subV.map((v) => String(v)).join(', ')}]`);
        } else if (typeof subV === 'number' || typeof subV === 'boolean') {
          lines.push(`  ${subK}: ${subV}`);
        } else {
          lines.push(`  ${subK}: ${quote(String(subV))}`);
        }
      }
      return;
    }
    written.add(key);
    if (Array.isArray(value)) {
      const allScalar = value.every((v) => !/[,\s]/.test(String(v)));
      if (allScalar) {
        lines.push(`${key}: [${value.map((v) => String(v)).join(', ')}]`);
      } else {
        lines.push(`${key}:`);
        for (const v of value) lines.push(`  - ${quote(String(v))}`);
      }
      return;
    }
    const str = String(value);
    if (/\n/.test(str)) {
      lines.push(`${key}: |`);
      for (const line of str.split('\n')) lines.push('  ' + line);
      return;
    }
    if (typeof value === 'number' || typeof value === 'boolean') {
      lines.push(`${key}: ${value}`);
      return;
    }
    lines.push(`${key}: ${quote(str)}`);
  };
  const order = keyOrder ?? Object.keys(fm);
  for (const key of order) push(key);
  for (const key of Object.keys(fm)) push(key);
  lines.push('---');
  return lines.join('\n');
}

function quote(s: string): string {
  // Emit as a YAML plain scalar when safe. Quoting is only required for:
  //   - empty strings or leading/trailing whitespace
  //   - strings starting with a YAML indicator character
  //   - `-`, `?`, `:` followed by space/EOL
  //   - strings containing `: ` or ` #` (would be parsed as mapping / comment)
  //   - values that look like bool/null/number (avoid auto-coercion on re-parse)
  if (s === '' || s.trim() !== s) return `"${s.replace(/"/g, '\\"')}"`;
  if (/^[!&*[\]{}|>@`'"%#,]/.test(s)) return `"${s.replace(/"/g, '\\"')}"`;
  if (/^[-?:](\s|$)/.test(s)) return `"${s.replace(/"/g, '\\"')}"`;
  if (/:\s|\s#/.test(s)) return `"${s.replace(/"/g, '\\"')}"`;
  if (/^(true|false|null|~|yes|no|on|off)$/i.test(s)) return `"${s}"`;
  if (/^-?\d+(\.\d+)?$/.test(s)) return `"${s}"`;
  return s;
}

export function composeSkillMarkdown(fm: Frontmatter, body: string, keyOrder?: string[]): string {
  const fmText = serializeFrontmatter(fm, keyOrder);
  const bodyText = body.startsWith('\n') ? body : '\n' + body;
  return `${fmText}${bodyText}`;
}
