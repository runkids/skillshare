import { Document, isScalar, isSeq, parse, parseDocument, Scalar, visit } from 'yaml';

export type FrontmatterScalar = string | number | boolean | null;
export type FrontmatterNested = { [key: string]: FrontmatterValue };
export type FrontmatterValue = FrontmatterScalar | FrontmatterValue[] | FrontmatterNested;
export type Frontmatter = Record<string, FrontmatterValue>;

export interface ParsedSkillMarkdown {
  bom: string; // '\uFEFF' when the file starts with one, so a save can put it back
  frontmatter: Frontmatter;
  rawFrontmatter: string;
  body: string;
  hasFrontmatter: boolean;
}

const FENCE = /^---\s*(?:\r?\n|$)/;

export function parseSkillMarkdown(content: string): ParsedSkillMarkdown {
  // Windows editors write a BOM before the opening fence; the Go readers skip it too.
  const bom = content.startsWith('\uFEFF') ? '\uFEFF' : '';
  return { bom, ...parseFenced(content.slice(bom.length)) };
}

function parseFenced(content: string): Omit<ParsedSkillMarkdown, 'bom'> {
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
  try {
    const value = parse(text + '\n');
    return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
  } catch {
    // Keep older, non-YAML skill headers editable (e.g. unquoted description colons).
    return parseLegacyYaml(text);
  }
}

function parseLegacyYaml(text: string): Frontmatter {
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
          arr.push(
            lines[i]
              .replace(/^\s*-\s+/, '')
              .trim()
              .replace(/^['"]|['"]$/g, ''),
          );
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
          if (subColon === -1) {
            i++;
            continue;
          }
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

// An untouched decimal such as 1.10 keeps its written node, so a save does not drop its trailing zeros.
class Decimal {
  readonly node: Scalar<number>;
  constructor(node: Scalar<number>) {
    this.node = node;
  }
}

export function serializeFrontmatter(
  fm: Frontmatter,
  keyOrder?: string[],
  originalYaml?: string,
): string {
  const doc = new Document();
  let original: unknown;
  if (originalYaml) {
    try {
      // YAML resolves aliases with a shared cache. BigInts stay transient;
      // editor and JSON values remain ordinary JavaScript numbers.
      const doc = parseDocument(originalYaml, { intAsBigInt: true });
      if (doc.errors.length > 0) throw doc.errors[0];
      visit(doc, {
        Scalar(_, node) {
          // JS prints 0.00000010 as 1e-7; padding that with zeros would not be a number.
          if (
            typeof node.value === 'number' &&
            node.minFractionDigits &&
            !String(node.value).includes('e')
          ) {
            const written = new Scalar(node.value);
            written.minFractionDigits = node.minFractionDigits;
            node.value = new Decimal(written);
          }
        },
      });
      original = doc.toJS();
    } catch {
      // Malformed legacy headers continue through the ordinary serializer.
    }
  }
  const seen = new WeakMap<object, unknown>();
  const preserve = (value: FrontmatterValue, old: unknown): unknown => {
    if (old instanceof Decimal) return Object.is(value, old.node.value) ? old.node : value;
    if (typeof old === 'bigint' && typeof value === 'number' && Object.is(value, Number(old))) {
      return old;
    }
    if (!value || !old || typeof value !== 'object' || typeof old !== 'object') return value;
    const cached = seen.get(value);
    if (cached) return cached;
    if (Array.isArray(value) && Array.isArray(old)) {
      const result: unknown[] = [];
      seen.set(value, result);
      value.forEach((item, i) => result.push(preserve(item, old[i])));
      return result;
    }
    if (!Array.isArray(value) && !Array.isArray(old)) {
      const result: Record<string, unknown> = {};
      const previous = old as Record<string, unknown>;
      seen.set(value, result);
      for (const key of Object.keys(value)) {
        Object.defineProperty(result, key, {
          value: preserve(value[key], previous[key]),
          enumerable: true,
          writable: true,
          configurable: true,
        });
      }
      return result;
    }
    return value;
  };
  const originals =
    original && typeof original === 'object' && !Array.isArray(original)
      ? (original as Record<string, unknown>)
      : {};
  const keys = new Set([...(keyOrder ?? []), ...Object.keys(fm)]);
  for (const key of keys) {
    const value = fm[key];
    if (value == null || value === '' || (Array.isArray(value) && value.length === 0)) continue;
    const node = doc.createNode(preserve(value, originals[key]));
    // Prose stays readable without adding quote escapes to the source.
    if (
      key === 'description' &&
      isScalar(node) &&
      typeof value === 'string' &&
      /:\s|\n/.test(value)
    ) {
      node.type = Scalar.BLOCK_FOLDED;
    }
    if (
      isSeq(node) &&
      value instanceof Array &&
      value.every((v) => v == null || typeof v !== 'object')
    ) {
      node.flow = value.every((v) => !/[,\s]/.test(String(v)));
    }
    doc.set(key, node);
  }
  return `---\n${doc.toString({ lineWidth: 0, singleQuote: false, flowCollectionPadding: false })}---`;
}

export function composeSkillMarkdown(
  fm: Frontmatter,
  body: string,
  keyOrder?: string[],
  rawFrontmatter?: string,
  bom = '',
  originalYaml?: string,
): string {
  // Preserve original frontmatter text when it hasn't been edited,
  // avoiding spurious formatting changes (quote style, line wrapping).
  const fmText =
    rawFrontmatter != null
      ? `---\n${rawFrontmatter}\n---`
      : serializeFrontmatter(fm, keyOrder, originalYaml);
  const bodyText = body.startsWith('\n') ? body : '\n' + body;
  return `${bom}${fmText}${bodyText}`;
}
