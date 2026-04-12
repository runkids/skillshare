import { describe, expect, it } from 'vitest';
import {
  buildSkillDraftStats,
  buildSkillTokenBreakdown,
  serializeFrontmatterEditorValue,
  renameSkillFrontmatterField,
  updateSkillFrontmatterField,
  normalizeMarkdownForRichEditor,
  parseSkillMarkdown,
  splitSkillMarkdown,
} from './skillMarkdown';

describe('parseSkillMarkdown', () => {
  it('extracts manifest frontmatter fields and returns markdown body for SKILL.md content', () => {
    const markdown = `# Overview

Use this skill for tasks.`;
    const content = `---
name: "My Skill"
description: "A concise skill description."
license: MIT
---
${markdown}`;

    const parsed = parseSkillMarkdown(content);

    expect(parsed.manifest).toEqual({
      name: 'My Skill',
      description: 'A concise skill description.',
      license: 'MIT',
    });
    expect(parsed.markdown).toBe(`\n${markdown}`);
  });

  it('preserves markdown body with custom tags, quote, and table', () => {
    const body = `<custom-note level="warning">Careful</custom-note>

> This is a quoted line.

| Col A | Col B |
| ----- | ----- |
| 1     | 2     |`;
    const content = `---
name: custom-format-skill
---
${body}`;

    const parsed = parseSkillMarkdown(content);

    expect(parsed.markdown).toBe(`\n${body}`);
  });

  it('parses YAML block scalar descriptions from frontmatter', () => {
    const content = `---
name: yaml-block-skill
description: |
  First line
  Second line
license: MIT
---
Body`;

    const parsed = parseSkillMarkdown(content);

    expect(parsed.manifest).toEqual({
      name: 'yaml-block-skill',
      description: 'First line\nSecond line\n',
      license: 'MIT',
    });
  });

  it('returns advanced frontmatter values for the field guide', () => {
    const content = `---
name: advanced-skill
description: Advanced frontmatter example
argument-hint: "[issue-number]"
disable-model-invocation: true
user-invocable: false
allowed-tools:
  - Read
  - Grep
context: fork
agent: explorer
paths:
  - src/**
  - tests/**
shell: bash
---
Body`;

    const parsed = parseSkillMarkdown(content);

    expect(parsed.frontmatter).toMatchObject({
      name: 'advanced-skill',
      description: 'Advanced frontmatter example',
      'argument-hint': '[issue-number]',
      'disable-model-invocation': true,
      'user-invocable': false,
      'allowed-tools': ['Read', 'Grep'],
      context: 'fork',
      agent: 'explorer',
      paths: ['src/**', 'tests/**'],
      shell: 'bash',
    });
  });
});

describe('splitSkillMarkdown', () => {
  it('preserves body whitespace exactly after frontmatter delimiter', () => {
    const body = `\n\n    \`\`\`ts
    const value = 1;
    \`\`\`
`;
    const content = `---
name: preserve-whitespace
---` + body;

    const split = splitSkillMarkdown(content);

    expect(split.frontmatter).toBe('name: preserve-whitespace');
    expect(split.markdown).toBe(body);
  });
});

describe('buildSkillDraftStats', () => {
  it('builds draft stats from full SKILL.md content including frontmatter', () => {
    const content = `---
name: "My Skill"
description: parser helper
license: MIT
---
## Heading
alpha beta

- gamma`;

    expect(buildSkillDraftStats(content)).toEqual({
      wordCount: 16,
      lineCount: 9,
      tokenCount: 25,
    });
  });

  it('uses cl100k-compatible tokenization for draft stats', () => {
    expect(buildSkillDraftStats('tiktoken is great!')).toEqual({
      wordCount: 3,
      lineCount: 1,
      tokenCount: 6,
    });
  });
});

describe('buildSkillTokenBreakdown', () => {
  it('reports full skill tokens separately from the rendered preview tokens', () => {
    const content = `---
name: "My Skill"
description: parser helper
license: MIT
---
## Heading
alpha beta

- gamma`;

    expect(buildSkillTokenBreakdown(content)).toEqual({
      loadTokens: buildSkillDraftStats('My Skill parser helper').tokenCount,
      previewTokens: buildSkillDraftStats(parseSkillMarkdown(content).markdown).tokenCount,
    });
  });
});

describe('normalizeMarkdownForRichEditor', () => {
  it('expands outer fenced code blocks when they contain nested fences of the same marker type', () => {
    const content = [
      '```markdown',
      '## Release Template',
      '',
      '- Example command:',
      '  ```bash',
      '  skillshare command --flag',
      '  ```',
      '```',
    ].join('\n');

    expect(normalizeMarkdownForRichEditor(content)).toBe([
      '````markdown',
      '## Release Template',
      '',
      '- Example command:',
      '  ```bash',
      '  skillshare command --flag',
      '  ```',
      '````',
    ].join('\n'));
  });

  it('leaves already-safe fences unchanged', () => {
    const content = [
      '```bash',
      'skillshare sync',
      '```',
    ].join('\n');

    expect(normalizeMarkdownForRichEditor(content)).toBe(content);
  });
});

describe('frontmatter editing helpers', () => {
  it('serializes structured frontmatter values into editable text', () => {
    expect(serializeFrontmatterEditorValue({
      targets: ['claude', 'universal'],
    })).toBe([
      '{',
      '  "targets": [',
      '    "claude",',
      '    "universal"',
      '  ]',
      '}',
    ].join('\n'));
  });

  it('updates a string frontmatter field while preserving the markdown body', () => {
    const content = [
      '---',
      'name: skillshare-changelog',
      'argument-hint: "[tag-version]"',
      'metadata:',
      '  targets: [claude, universal]',
      '---',
      '',
      '# Body',
    ].join('\n');

    expect(updateSkillFrontmatterField(content, 'argument-hint', '[release-version]', '[tag-version]')).toContain('argument-hint: "[release-version]"');
    expect(updateSkillFrontmatterField(content, 'argument-hint', '[release-version]', '[tag-version]')).toContain('\n# Body');
  });

  it('accepts plain string allowed-tools values', () => {
    const content = [
      '---',
      'name: skillshare-changelog',
      '---',
      '',
      '# Body',
    ].join('\n');

    expect(updateSkillFrontmatterField(content, 'allowed-tools', 'Read Grep')).toContain('allowed-tools: Read Grep');
  });

  it('accepts plain string paths values', () => {
    const content = [
      '---',
      'name: skillshare-changelog',
      '---',
      '',
      '# Body',
    ].join('\n');

    expect(updateSkillFrontmatterField(content, 'paths', 'src/**, tests/**')).toContain('paths: src/**, tests/**');
  });

  it('keeps built-in fields in canonical order when adding a built-in field', () => {
    const content = [
      '---',
      'name: skillshare-changelog',
      'custom-note: hello',
      '---',
      '',
      '# Body',
    ].join('\n');

    expect(updateSkillFrontmatterField(content, 'description', 'A concise description.')).toBe([
      '---',
      'name: skillshare-changelog',
      'description: A concise description.',
      'custom-note: hello',
      '---',
      '',
      '# Body',
    ].join('\n'));
  });

  it('removes a frontmatter field when the editor value is cleared', () => {
    const content = [
      '---',
      'name: skillshare-changelog',
      'argument-hint: "[tag-version]"',
      '---',
      '',
      '# Body',
    ].join('\n');

    expect(updateSkillFrontmatterField(content, 'argument-hint', '', '[tag-version]')).not.toContain('argument-hint:');
  });

  it('adds a new custom frontmatter field when a value is provided', () => {
    const content = [
      '---',
      'name: skillshare-changelog',
      '---',
      '',
      '# Body',
    ].join('\n');

    expect(updateSkillFrontmatterField(content, 'custom-note', 'hello world')).toContain('custom-note: hello world');
  });

  it('parses structured frontmatter editor values as objects when the existing field is structured', () => {
    const content = [
      '---',
      'name: skillshare-changelog',
      'metadata:',
      '  targets: [claude, universal]',
      '---',
      '',
      '# Body',
    ].join('\n');

    expect(updateSkillFrontmatterField(
      content,
      'metadata',
      [
        '{',
        '  "targets": ["claude"]',
        '}',
      ].join('\n'),
      { targets: ['claude', 'universal'] },
    )).toContain('targets:\n    - claude');
  });

  it('renames a field while preserving canonical built-in order', () => {
    const content = [
      '---',
      'name: skillshare-changelog',
      'paths: src/**, tests/**',
      'custom-note: hello',
      '---',
      '',
      '# Body',
    ].join('\n');

    expect(renameSkillFrontmatterField(content, 'custom-note', 'description')).toBe([
      '---',
      'name: skillshare-changelog',
      'description: hello',
      'paths: src/**, tests/**',
      '---',
      '',
      '# Body',
    ].join('\n'));
  });

  it('rejects renaming into an existing field', () => {
    const content = [
      '---',
      'name: skillshare-changelog',
      'description: Existing',
      '---',
      '',
      '# Body',
    ].join('\n');

    expect(() => renameSkillFrontmatterField(content, 'name', 'description')).toThrow('description already exists.');
  });
});
