import { describe, expect, it } from 'vitest';
import { messagesByLocale } from './index';

// Every source file of the dashboard, as text.
const sources = Object.entries(
  import.meta.glob<string>('../**/*.{ts,tsx}', { query: '?raw', import: 'default', eager: true }),
).filter(([path]) => !/\.test\.tsx?$/.test(path));

const en = messagesByLocale.en;
const namespaces = new Set(Object.keys(en).map((key) => key.split('.')[0]));
const counted = (key: string) => `${key}.one` in en && `${key}.other` in en;
// A key, or what `plural` completes to one. For a string whose use the test cannot see.
const known = (key: string) => key in en || counted(key);

// The escape hatch. A string that looks like a key but is not one is named here, with what it is.
// Files of config paths (`targets.skills.mode`), which share their first word with key namespaces.
// The checks of bare dotted names skip them; their t() and plural() calls are still checked:
const notKeyFiles = ['../lib/fieldDocs.ts', '../components/config/FieldDocs.tsx'];
const notKeys = new Set([
  'config.yaml', // a file name
  'install.track_kind_ambiguous', // an API error code
]);

function referenced(
  pattern: RegExp,
  keep: (text: string) => boolean = () => true,
  skip: string[] = [],
) {
  const found: string[] = [];
  for (const [path, text] of sources) {
    if (skip.includes(path)) continue;
    for (const match of text.matchAll(pattern))
      if (!notKeys.has(match[1]) && keep(match[1])) found.push(`${match[1]}  (${path.slice(3)})`);
  }
  return found;
}
const inNamespace = (text: string) => namespaces.has(text.split('.')[0]);

describe('translation keys the dashboard refers to', () => {
  it('finds the sources', () => {
    expect(sources.length).toBeGreaterThan(100);
  });

  // A mistyped key is not an error anywhere else: `t` takes any string and shows the key itself.
  it('has every key written out in the source', () => {
    // Any quoted dotted name in a namespace en.json has: `t('a.b')`, a key in a table, a branch of a ternary.
    const literal = /['"`]([A-Za-z][\w-]*(?:\.[\w-]+)+)['"`]/g;
    expect(
      referenced(literal, inNamespace, notKeyFiles).filter((ref) => !known(ref.split('  ')[0])),
    ).toEqual([]);
  });

  it('has every key passed straight to t(), whatever its namespace', () => {
    expect(
      referenced(/\bt\(\s*['"]([^'"]+)['"]/g).filter((ref) => !(ref.split('  ')[0] in en)),
    ).toEqual([]);
  });

  it('has both forms of every key passed straight to plural()', () => {
    expect(
      referenced(/\bplural\(\s*['"]([^'"]+)['"]/g).filter((ref) => !counted(ref.split('  ')[0])),
    ).toEqual([]);
  });

  // A key built at run time (`sync.edited.${mode}`) cannot be checked whole. Its fixed head is:
  // some key has to start with it, so a typo in the head, or a removed group, still fails here.
  it('has keys under the fixed head of every key built at run time', () => {
    const keys = Object.keys(en);
    const head = /`([A-Za-z][\w-]*(?:\.[\w-]+)*\.)\$\{/g;
    expect(
      referenced(head, inNamespace, notKeyFiles).filter(
        (ref) => !keys.some((key) => key.startsWith(ref.split('  ')[0])),
      ),
    ).toEqual([]);
  });
});
