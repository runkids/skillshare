import { describe, it, expect } from 'vitest';
import { parse as parseYaml } from 'yaml';
import { parseSkillMarkdown, serializeFrontmatter, composeSkillMarkdown } from './frontmatter';

describe('parseSkillMarkdown', () => {
  it('parses scalar fields', () => {
    const { frontmatter, body } = parseSkillMarkdown(
      '---\nname: foo\ndescription: bar\n---\n# body'
    );
    expect(frontmatter.name).toBe('foo');
    expect(frontmatter.description).toBe('bar');
    expect(body).toBe('# body');
  });

  it('parses inline arrays', () => {
    const { frontmatter } = parseSkillMarkdown(
      '---\nallowed-tools: [Read, Grep, Bash(git status *)]\n---\nbody'
    );
    expect(frontmatter['allowed-tools']).toEqual(['Read', 'Grep', 'Bash(git status *)']);
  });

  it('handles multiline block scalars (|)', () => {
    const { frontmatter } = parseSkillMarkdown(
      '---\ndescription: |\n  Line one\n  Line two\n---\nbody'
    );
    expect(frontmatter.description).toBe('Line one\nLine two\n');
  });

  it('keeps legacy headers with unquoted description colons editable', () => {
    const { frontmatter } = parseSkillMarkdown('---\nname: foo\ndescription: Use when asked: convert PDF\n---\nbody');
    expect(frontmatter).toEqual({ name: 'foo', description: 'Use when asked: convert PDF' });
  });

  it('skips a leading BOM before the opening fence', () => {
    const { frontmatter, body, hasFrontmatter } = parseSkillMarkdown(
      '\uFEFF---\nname: foo\n---\n# body'
    );
    expect(hasFrontmatter).toBe(true);
    expect(frontmatter.name).toBe('foo');
    expect(body).toBe('# body');
  });

  it('puts the BOM back when composing', () => {
    const original = '\uFEFF---\nname: foo\n---\n# body';
    const { frontmatter, body, rawFrontmatter, bom } = parseSkillMarkdown(original);
    expect(composeSkillMarkdown(frontmatter, body, undefined, rawFrontmatter, bom)).toBe(original);
  });

  it('returns hasFrontmatter=false when missing', () => {
    const result = parseSkillMarkdown('# just a body');
    expect(result.hasFrontmatter).toBe(false);
    expect(result.body).toBe('# just a body');
  });

  it('round-trips via compose', () => {
    const original = '---\nname: foo\ndescription: bar\n---\n# body';
    const { frontmatter, body } = parseSkillMarkdown(original);
    const recomposed = composeSkillMarkdown(frontmatter, body);
    const reparsed = parseSkillMarkdown(recomposed);
    expect(reparsed.frontmatter).toEqual(frontmatter);
    expect(reparsed.body.trim()).toBe(body.trim());
  });

  it('preserves markdown list syntax through round-trip', () => {
    const original = '---\nname: foo\n---\n- item one\n- item two\n\n**bold**';
    const { body } = parseSkillMarkdown(original);
    expect(body).toContain('- item one');
    expect(body).toContain('**bold**');
  });
});

describe('edited frontmatter', () => {
  it('decodes quoted descriptions without exposing YAML escapes', () => {
    const description = 'Use when asked: "convert PDF". Path: C:\\tools';
    const content = `---\ndescription: ${JSON.stringify(description)}\n---\n# Body`;
    expect(parseSkillMarkdown(content).frontmatter.description).toBe(description);
  });

  it('preserves folded descriptions and nested flow metadata through repeated edits', () => {
    const metadata = { openclaw: { emoji: '📄', requires: { bins: ['mineru-open-api'] }, install: [{ kind: 'node', package: 'mineru-open-api' }] } };
    const original = `---\nname: MinerU Document Extractor\ndescription: >-\n  Convert PDFs.\n  Use when asked: "convert PDF to Markdown".\nmetadata: ${JSON.stringify(metadata)}\nallowed-tools: Bash(mineru-open-api:*)\n---\n# MinerU\n`;
    const description = 'MinerU document extraction: convert PDFs. Use when asked: "convert PDF". 文件提取 C:\\tools';
    let content = original;
    for (let i = 0; i < 3; i++) {
      const parsed = parseSkillMarkdown(content);
      expect(parsed.frontmatter.metadata).toEqual(metadata);
      content = composeSkillMarkdown({ ...parsed.frontmatter, name: 'mineru-document-extractor', description }, parsed.body);
      const saved = parseSkillMarkdown(content);
      expect(parseYaml(saved.rawFrontmatter)).toMatchObject({ name: 'mineru-document-extractor', description, metadata });
      expect(saved.frontmatter.description).toBe(description);
      expect(saved.body).toBe('# MinerU\n');
    }
    expect(content).toContain('description: >-\n');
    expect(content).not.toContain('\\"convert PDF');
  });

  it('preserves YAML chomp indicators, paragraphs and typed metadata', () => {
    const content = '---\ndescription: >-\n  Line one\n  wraps here\n\n  Next paragraph\nmetadata:\n  version: "1.0"\n  enabled: false\n  aliases: ["a, b", "true"]\n---\nbody';
    const parsed = parseSkillMarkdown(content);
    expect(parsed.frontmatter.description).toBe('Line one wraps here\nNext paragraph');
    expect(parsed.frontmatter.metadata).toEqual({ version: '1.0', enabled: false, aliases: ['a, b', 'true'] });
    const saved = composeSkillMarkdown({ ...parsed.frontmatter, name: 'foo' }, parsed.body);
    expect(parseYaml(parseSkillMarkdown(saved).rawFrontmatter)).toEqual({ ...parsed.frontmatter, name: 'foo' });
  });
});

describe('serializeFrontmatter', () => {
  it('emits known-safe scalars unquoted', () => {
    const out = serializeFrontmatter({ name: 'foo' });
    expect(out).toContain('name: foo');
  });

  it('quotes non-prose values with YAML-reserved chars', () => {
    const out = serializeFrontmatter({ name: 'hello: world' });
    expect(out).toContain('name: "hello: world"');
  });

  it('emits multiline as block scalar', () => {
    const out = serializeFrontmatter({ when_to_use: 'line1\nline2' });
    expect(out).toMatch(/when_to_use: \|-\n {2}line1\n {2}line2/);
    expect(parseYaml(parseSkillMarkdown(out).rawFrontmatter).when_to_use).toBe('line1\nline2');
  });

  it('emits inline arrays for scalar elements', () => {
    const out = serializeFrontmatter({ 'allowed-tools': ['Read', 'Grep'] });
    expect(out).toContain('allowed-tools: [Read, Grep]');
  });

  it('respects keyOrder', () => {
    const out = serializeFrontmatter(
      { description: 'b', name: 'a' },
      ['name', 'description']
    );
    expect(out.indexOf('name:')).toBeLessThan(out.indexOf('description:'));
  });
});
