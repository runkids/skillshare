import type { SkillStats } from '../api/client';
import { getEncoding, type Tiktoken } from 'js-tiktoken';
import { parse as parseYaml, stringify as stringifyYaml } from 'yaml';
import { getFrontmatterFields, type FrontmatterSchema } from './skillFrontmatter';

export type SkillManifest = {
  name?: string;
  description?: string;
  license?: string;
};

export type SkillFrontmatter = Record<string, unknown>;

export type SkillMarkdownParts = {
  frontmatter?: string;
  markdown: string;
};

const BOOLEAN_FRONTMATTER_KEYS = new Set([
  'disable-model-invocation',
  'user-invocable',
  'background',
]);

const STRUCTURED_FRONTMATTER_KEYS = new Set([
  'hooks',
  'metadata',
  'mcpServers',
]);

const STRING_OR_LIST_FRONTMATTER_KEYS = new Set([
  'allowed-tools',
  'paths',
  'tools',
  'disallowedTools',
  'skills',
]);

export function normalizeMarkdownForRichEditor(content: string): string {
  if (!content) return '';

  const lines = content.split(/\r?\n/);

  for (let index = 0; index < lines.length; index += 1) {
    const openingMatch = lines[index].match(/^(\s{0,3})(`{3,}|~{3,})(.*)$/);
    if (!openingMatch) {
      continue;
    }

    const [, openingIndent, openingFence, openingRest] = openingMatch;
    const marker = openingFence[0];
    const openingFenceLength = openingFence.length;
    let maxNestedFenceLength = 0;
    const nestedFenceStack: number[] = [];
    let closingIndex = -1;
    let closingIndent = '';
    let closingSuffix = '';

    for (let innerIndex = index + 1; innerIndex < lines.length; innerIndex += 1) {
      const line = lines[innerIndex];
      const nestedFenceMatch = line.match(/^(\s{0,3})(`{3,}|~{3,})(.*)$/);
      if (nestedFenceMatch && nestedFenceMatch[2][0] === marker) {
        const fenceLength = nestedFenceMatch[2].length;
        const fenceRest = nestedFenceMatch[3] ?? '';
        const isPlainClosingFence = fenceRest.trim().length === 0;

        if (!isPlainClosingFence) {
          nestedFenceStack.push(fenceLength);
          maxNestedFenceLength = Math.max(maxNestedFenceLength, fenceLength);
          continue;
        }

        if (nestedFenceStack.length > 0 && fenceLength >= nestedFenceStack[nestedFenceStack.length - 1]) {
          nestedFenceStack.pop();
          maxNestedFenceLength = Math.max(maxNestedFenceLength, fenceLength);
          continue;
        }

        if (fenceLength >= openingFenceLength) {
          closingIndex = innerIndex;
          closingIndent = nestedFenceMatch[1];
          closingSuffix = fenceRest;
          break;
        }

        maxNestedFenceLength = Math.max(maxNestedFenceLength, fenceLength);
      }
    }

    if (closingIndex === -1 || maxNestedFenceLength < openingFenceLength) {
      continue;
    }

    const normalizedFenceLength = maxNestedFenceLength + 1;
    const normalizedFence = marker.repeat(normalizedFenceLength);

    lines[index] = `${openingIndent}${normalizedFence}${openingRest}`;
    lines[closingIndex] = `${closingIndent}${normalizedFence}${closingSuffix}`;
    index = closingIndex;
  }

  return lines.join('\n');
}

export function serializeFrontmatterEditorValue(value: unknown): string {
  if (value === null || value === undefined) return '';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  return JSON.stringify(value, null, 2);
}

function parseFrontmatterEditorValue(key: string, rawValue: string, previousValue: unknown): unknown {
  const trimmedValue = rawValue.trim();

  if (trimmedValue.length === 0) {
    return undefined;
  }

  if (typeof previousValue === 'boolean' || BOOLEAN_FRONTMATTER_KEYS.has(key)) {
    if (trimmedValue === 'true') return true;
    if (trimmedValue === 'false') return false;
    throw new Error(`${key} must be true or false.`);
  }

  if (typeof previousValue === 'number') {
    const parsedNumber = Number(trimmedValue);
    if (Number.isNaN(parsedNumber)) {
      throw new Error(`${key} must be a number.`);
    }
    return parsedNumber;
  }

  if (STRING_OR_LIST_FRONTMATTER_KEYS.has(key)) {
    try {
      const parsedList = parseYaml(rawValue);
      if (Array.isArray(parsedList)) {
        return parsedList;
      }
    } catch {
      // Fall through to the documented string form.
    }
    return rawValue;
  }

  if (Array.isArray(previousValue)) {
    const parsedArray = parseYaml(rawValue);
    if (!Array.isArray(parsedArray)) {
      throw new Error(`${key} must be a YAML or JSON array.`);
    }
    return parsedArray;
  }

  if (
    (typeof previousValue === 'object' && previousValue !== null) ||
    STRUCTURED_FRONTMATTER_KEYS.has(key)
  ) {
    const parsedStructuredValue = parseYaml(rawValue);
    if (
      parsedStructuredValue === null ||
      typeof parsedStructuredValue !== 'object'
    ) {
      throw new Error(`${key} must be valid YAML or JSON.`);
    }
    return parsedStructuredValue;
  }

  return rawValue;
}

export function updateSkillFrontmatterField(
  content: string,
  key: string,
  rawValue: string,
  previousValue?: unknown,
  schema: FrontmatterSchema = 'skill',
): string {
  const { frontmatter, markdown } = splitSkillMarkdown(content);
  const parsedFrontmatter = frontmatter ? parseYaml(frontmatter) : {};
  const currentFrontmatter = (typeof parsedFrontmatter === 'object' && parsedFrontmatter !== null)
    ? { ...(parsedFrontmatter as Record<string, unknown>) }
    : {};

  const nextValue = parseFrontmatterEditorValue(key, rawValue, previousValue ?? currentFrontmatter[key]);
  if (nextValue === undefined) {
    delete currentFrontmatter[key];
  } else {
    currentFrontmatter[key] = nextValue;
  }

  const serializedFrontmatter = serializeCanonicalFrontmatter(currentFrontmatter, schema);
  if (!serializedFrontmatter) {
    return markdown.startsWith('\n') ? markdown.slice(1) : markdown;
  }

  const bodySeparator = markdown && !markdown.startsWith('\n') ? '\n' : '';
  return `---\n${serializedFrontmatter}\n---${bodySeparator}${markdown}`;
}

export function renameSkillFrontmatterField(
  content: string,
  currentKey: string,
  nextKey: string,
  schema: FrontmatterSchema = 'skill',
): string {
  if (currentKey === nextKey) return content;
  const { frontmatter, markdown } = splitSkillMarkdown(content);
  const parsedFrontmatter = frontmatter ? parseYaml(frontmatter) : {};
  const current = (typeof parsedFrontmatter === 'object' && parsedFrontmatter !== null)
    ? { ...(parsedFrontmatter as Record<string, unknown>) }
    : {};

  if (nextKey in current && nextKey !== currentKey) {
    throw new Error(`${nextKey} already exists.`);
  }

  const value = current[currentKey];
  delete current[currentKey];
  if (value !== undefined) current[nextKey] = value;

  const serializedFrontmatter = serializeCanonicalFrontmatter(current, schema);
  return serializedFrontmatter ? `---\n${serializedFrontmatter}\n---${markdown && !markdown.startsWith('\n') ? '\n' : ''}${markdown}` : (markdown.startsWith('\n') ? markdown.slice(1) : markdown);
}

export function splitSkillMarkdown(content: string): SkillMarkdownParts {
  if (!content) return { markdown: '' };

  const match = content.match(/^---[ \t]*\r?\n([\s\S]*?)\r?\n---[ \t]*(?=\r?\n|$)/);
  if (!match) return { markdown: content };

  return {
    frontmatter: match[1],
    markdown: content.slice(match[0].length),
  };
}

export function buildSkillDraftStats(content: string): SkillStats {
  const wordCount = countWords(content);
  const lineCount = countLines(content);
  const tokenCount = countTokens(content);

  return { wordCount, lineCount, tokenCount };
}

export function buildSkillTokenBreakdown(content: string): {
  loadTokens: number;
  previewTokens: number;
} {
  const parsed = parseSkillMarkdown(content);
  const renderedMarkdown = parsed.markdown.trim() ? parsed.markdown : content;
  const alwaysLoadedContext = [parsed.manifest.name, parsed.manifest.description]
    .filter((value): value is string => Boolean(value && value.trim()))
    .join(' ');

  return {
    loadTokens: countTokens(alwaysLoadedContext),
    previewTokens: countTokens(renderedMarkdown),
  };
}

export function parseSkillMarkdown(content: string): {
  manifest: SkillManifest;
  frontmatter: SkillFrontmatter;
  markdown: string;
} {
  const { frontmatter, markdown } = splitSkillMarkdown(content);
  if (!frontmatter) return { manifest: {}, frontmatter: {}, markdown };

  let parsed: unknown;
  try {
    parsed = parseYaml(frontmatter);
  } catch {
    return { manifest: {}, frontmatter: {}, markdown };
  }

  const source = (typeof parsed === 'object' && parsed !== null) ? parsed as Record<string, unknown> : {};
  return {
    manifest: {
      name: typeof source.name === 'string' ? source.name : undefined,
      description: typeof source.description === 'string' ? source.description : undefined,
      license: typeof source.license === 'string' ? source.license : undefined,
    },
    frontmatter: source,
    markdown,
  };
}

function countWords(content: string): number {
  const trimmed = content.trim();
  if (!trimmed) return 0;
  return trimmed.split(/\s+/).length;
}

function countLines(content: string): number {
  const trimmed = content.trim();
  if (!trimmed) return 0;
  return trimmed.replaceAll('\r\n', '\n').split('\n').length;
}

function serializeCanonicalFrontmatter(frontmatter: Record<string, unknown>, schema: FrontmatterSchema = 'skill'): string {
  const orderedFrontmatter: Record<string, unknown> = {};

  for (const field of getFrontmatterFields(schema)) {
    if (Object.prototype.hasOwnProperty.call(frontmatter, field.key)) {
      orderedFrontmatter[field.key] = frontmatter[field.key];
    }
  }

  for (const [key, value] of Object.entries(frontmatter)) {
    if (!Object.prototype.hasOwnProperty.call(orderedFrontmatter, key)) {
      orderedFrontmatter[key] = value;
    }
  }

  return stringifyYaml(orderedFrontmatter, { lineWidth: 0 }).trimEnd();
}

let cl100kEncoder: Tiktoken | null = null;
let cl100kEncoderLoadFailed = false;

function getCL100KEncoder(): Tiktoken | null {
  if (cl100kEncoder) return cl100kEncoder;
  if (cl100kEncoderLoadFailed) return null;
  try {
    cl100kEncoder = getEncoding('cl100k_base');
    return cl100kEncoder;
  } catch {
    cl100kEncoderLoadFailed = true;
    return null;
  }
}

function countTokens(content: string): number {
  const encoder = getCL100KEncoder();
  if (!encoder) return 0;
  return encoder.encode(content).length;
}
