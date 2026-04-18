import { describe, it, expect } from 'vitest';
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
    expect(frontmatter.description).toBe('Line one\nLine two');
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

describe('serializeFrontmatter', () => {
  it('emits known-safe scalars unquoted', () => {
    const out = serializeFrontmatter({ name: 'foo' });
    expect(out).toContain('name: foo');
  });

  it('quotes values with YAML-reserved chars', () => {
    const out = serializeFrontmatter({ description: 'hello: world' });
    expect(out).toContain('description: "hello: world"');
  });

  it('emits multiline as block scalar', () => {
    const out = serializeFrontmatter({ description: 'line1\nline2' });
    expect(out).toMatch(/description: \|\n  line1\n  line2/);
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
