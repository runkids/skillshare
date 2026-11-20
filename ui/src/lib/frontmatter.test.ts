import { describe, it, expect, vi } from 'vitest';
import { createElement } from 'react';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { api, type Skill } from '../api/client';
import { I18nProvider } from '../i18n';
import { ToastProvider } from '../components/Toast';
import SkillEditor from '../components/skill-editor/SkillEditor';
import { parse as parseYaml } from 'yaml';
import {
  parseSkillMarkdown,
  serializeFrontmatter,
  composeSkillMarkdown,
  type FrontmatterNested,
} from './frontmatter';

describe('parseSkillMarkdown', () => {
  it('parses scalar fields', () => {
    const { frontmatter, body } = parseSkillMarkdown(
      '---\nname: foo\ndescription: bar\n---\n# body',
    );
    expect(frontmatter.name).toBe('foo');
    expect(frontmatter.description).toBe('bar');
    expect(body).toBe('# body');
  });

  it('parses inline arrays', () => {
    const { frontmatter } = parseSkillMarkdown(
      '---\nallowed-tools: [Read, Grep, Bash(git status *)]\n---\nbody',
    );
    expect(frontmatter['allowed-tools']).toEqual(['Read', 'Grep', 'Bash(git status *)']);
  });

  it('handles multiline block scalars (|)', () => {
    const { frontmatter } = parseSkillMarkdown(
      '---\ndescription: |\n  Line one\n  Line two\n---\nbody',
    );
    expect(frontmatter.description).toBe('Line one\nLine two\n');
  });

  it('keeps legacy headers with unquoted description colons editable', () => {
    const { frontmatter } = parseSkillMarkdown(
      '---\nname: foo\ndescription: Use when asked: convert PDF\n---\nbody',
    );
    expect(frontmatter).toEqual({ name: 'foo', description: 'Use when asked: convert PDF' });
  });

  it('skips a leading BOM before the opening fence', () => {
    const { frontmatter, body, hasFrontmatter } = parseSkillMarkdown(
      '\uFEFF---\nname: foo\n---\n# body',
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
    const metadata = {
      openclaw: {
        emoji: '📄',
        requires: { bins: ['mineru-open-api'] },
        install: [{ kind: 'node', package: 'mineru-open-api' }],
      },
    };
    const original = `---\nname: MinerU Document Extractor\ndescription: >-\n  Convert PDFs.\n  Use when asked: "convert PDF to Markdown".\nmetadata: ${JSON.stringify(metadata)}\nallowed-tools: Bash(mineru-open-api:*)\n---\n# MinerU\n`;
    const description =
      'MinerU document extraction: convert PDFs. Use when asked: "convert PDF". 文件提取 C:\\tools';
    let content = original;
    for (let i = 0; i < 3; i++) {
      const parsed = parseSkillMarkdown(content);
      expect(parsed.frontmatter.metadata).toEqual(metadata);
      content = composeSkillMarkdown(
        { ...parsed.frontmatter, name: 'mineru-document-extractor', description },
        parsed.body,
      );
      const saved = parseSkillMarkdown(content);
      expect(parseYaml(saved.rawFrontmatter)).toMatchObject({
        name: 'mineru-document-extractor',
        description,
        metadata,
      });
      expect(saved.frontmatter.description).toBe(description);
      expect(saved.body).toBe('# MinerU\n');
    }
    expect(content).toContain('description: >-\n');
    expect(content).not.toContain('\\"convert PDF');
  });

  it('preserves YAML chomp indicators, paragraphs and typed metadata', () => {
    const content =
      '---\ndescription: >-\n  Line one\n  wraps here\n\n  Next paragraph\nmetadata:\n  version: "1.0"\n  enabled: false\n  aliases: ["a, b", "true"]\n---\nbody';
    const parsed = parseSkillMarkdown(content);
    expect(parsed.frontmatter.description).toBe('Line one wraps here\nNext paragraph');
    expect(parsed.frontmatter.metadata).toEqual({
      version: '1.0',
      enabled: false,
      aliases: ['a, b', 'true'],
    });
    const saved = composeSkillMarkdown({ ...parsed.frontmatter, name: 'foo' }, parsed.body);
    expect(parseYaml(parseSkillMarkdown(saved).rawFrontmatter)).toEqual({
      ...parsed.frontmatter,
      name: 'foo',
    });
  });
});

describe('exact integers in edited headers', () => {
  it('keeps legacy headers editable on a dirty save', () => {
    const parsed = parseSkillMarkdown(
      '---\nname: foo\ndescription: Use when asked: convert PDF\n---\nbody',
    );
    const saved = composeSkillMarkdown(
      { ...parsed.frontmatter, name: 'bar' },
      parsed.body,
      undefined,
      undefined,
      parsed.bom,
      parsed.rawFrontmatter,
    );
    expect(parseSkillMarkdown(saved).frontmatter).toEqual({
      name: 'bar',
      description: 'Use when asked: convert PDF',
    });
  });

  it('retains integers reached through YAML aliases', () => {
    const parsed = parseSkillMarkdown(
      '---\nname: demo\nmetadata:\n  original: &event\n    id: 9007199254740993\n  copy: *event\n---\nbody',
    );
    const saved = composeSkillMarkdown(
      { ...parsed.frontmatter, description: 'edited' },
      parsed.body,
      undefined,
      undefined,
      parsed.bom,
      parsed.rawFrontmatter,
    );
    const exact = parseYaml(parseSkillMarkdown(saved).rawFrontmatter, { intAsBigInt: true });
    expect(exact.metadata.original.id).toBe(9007199254740993n);
    expect(exact.metadata.copy.id).toBe(9007199254740993n);
  });

  it('retains untouched values through description, targets, deletion and body edits', () => {
    const original =
      '---\nname: demo\ndescription: old\ntargets: [claude]\nmetadata:\n  x-acme-event-id: 9007199254740993\n  nested:\n    count: 9007199254740993\n    remove: yes\n  sources:\n    - resource: a.md\n      x-acme: 9007199254740993\n---\nbody';
    const first = parseSkillMarkdown(original);
    expect(typeof (first.frontmatter.metadata as FrontmatterNested)['x-acme-event-id']).toBe(
      'number',
    );
    expect(() => JSON.stringify(first.frontmatter)).not.toThrow();
    const bodyOnly = composeSkillMarkdown(
      first.frontmatter,
      'changed body',
      undefined,
      first.rawFrontmatter,
      first.bom,
    );
    expect(parseSkillMarkdown(bodyOnly).rawFrontmatter).toBe(first.rawFrontmatter);
    let content = composeSkillMarkdown(
      { ...first.frontmatter, description: 'new' },
      first.body,
      undefined,
      undefined,
      first.bom,
      first.rawFrontmatter,
    );
    let reopened = parseSkillMarkdown(content);
    const metadata = reopened.frontmatter.metadata as FrontmatterNested;
    const nested = metadata.nested as FrontmatterNested;
    content = composeSkillMarkdown(
      {
        ...reopened.frontmatter,
        targets: null,
        metadata: { ...metadata, targets: ['codex'], nested: { count: nested.count } },
      },
      reopened.body,
      undefined,
      undefined,
      reopened.bom,
      reopened.rawFrontmatter,
    );
    reopened = parseSkillMarkdown(content);
    const exact = parseYaml(reopened.rawFrontmatter, { intAsBigInt: true });
    expect(exact.description).toBe('new');
    expect(exact.targets).toBeUndefined();
    expect(exact.metadata.targets).toEqual(['codex']);
    expect(exact.metadata.nested.remove).toBeUndefined();
    expect(exact.metadata['x-acme-event-id']).toBe(9007199254740993n);
    expect(exact.metadata.nested.count).toBe(9007199254740993n);
    expect(exact.metadata.sources[0]['x-acme']).toBe(9007199254740993n);
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
    const out = serializeFrontmatter({ description: 'b', name: 'a' }, ['name', 'description']);
    expect(out.indexOf('name:')).toBeLessThan(out.indexOf('description:'));
  });
});

it('saves exact integers through the SkillEditor form and body-only paths', async () => {
  const save = vi.spyOn(api, 'saveSkillContent').mockResolvedValue({} as never);
  const resource: Skill = {
    name: 'demo',
    kind: 'skill',
    flatName: 'demo',
    relPath: 'demo',
    sourcePath: '/skills/demo',
    isInRepo: false,
  };
  const original =
    '---\nname: demo\ndescription: old\ntargets: [claude]\nmetadata:\n  x-acme: 9007199254740993\n  sources:\n    - resource: a.md\n      x-acme: 9007199254740993\n---\nbody';
  const mount = () =>
    render(
      createElement(
        MemoryRouter,
        null,
        createElement(
          I18nProvider,
          null,
          createElement(
            ToastProvider,
            null,
            createElement(SkillEditor, {
              resource,
              docName: 'SKILL.md',
              initialContent: original,
              onBack: vi.fn(),
              onSaved: vi.fn(),
            }),
          ),
        ),
      ),
    );
  const first = mount();
  fireEvent.change(screen.getByLabelText('Body'), { target: { value: 'new body' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save' }));
  fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Save' }));
  await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
  expect(parseSkillMarkdown(save.mock.calls[0][1]).rawFrontmatter).toBe(
    parseSkillMarkdown(original).rawFrontmatter,
  );
  first.unmount();
  mount();
  fireEvent.change(screen.getByDisplayValue('old'), { target: { value: 'new description' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save' }));
  fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Save' }));
  await waitFor(() => expect(save).toHaveBeenCalledTimes(2));
  const result = parseYaml(parseSkillMarkdown(save.mock.calls[1][1]).rawFrontmatter, {
    intAsBigInt: true,
  });
  expect(result.description).toBe('new description');
  expect(result.targets).toBeUndefined();
  expect(result.metadata.targets).toEqual(['claude']);
  expect(result.metadata['x-acme']).toBe(9007199254740993n);
  expect(result.metadata.sources[0]['x-acme']).toBe(9007199254740993n);
  save.mockRestore();
});
