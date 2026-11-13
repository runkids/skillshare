import { describe, expect, it } from 'vitest';
import { parse } from 'yaml';
import { formatYaml } from '../formatYaml';

describe('formatYaml', () => {
  it('normalizes block indentation while keeping comments and flow collections', () => {
    const source = "# config\ntargets: [claude, codex]\nmcp:\n    servers:\n        docs: {url: 'https://example.com/mcp'}\n";
    const formatted = formatYaml(source);
    expect(formatted).toBe("# config\ntargets: [claude, codex]\nmcp:\n  servers:\n    docs: {url: 'https://example.com/mcp'}\n");
    expect(parse(formatted)).toEqual(parse(source));
    expect(formatYaml(formatted)).toBe(formatted);
  });
  it('expands nested flow collections on request and keeps short flow lists', () => {
    const source = 'targets: [claude, codex]\nempty: {}\nhooks:\n  entries: {guard: {bindings: {claude: {events: {Stop: [{command: echo hi}]}}}}}\n';
    const formatted = formatYaml(source, { expandNested: true });
    expect(formatted).toBe('targets: [claude, codex]\nempty: {}\nhooks:\n  entries:\n    guard:\n      bindings:\n        claude:\n          events:\n            Stop:\n              - command: echo hi\n');
    expect(parse(formatted)).toEqual(parse(source));
  });
  it('rejects invalid YAML instead of replacing it', () => {
    expect(() => formatYaml('mcp: [')).toThrow();
  });
  it('orders config sections and separates them without changing their contents', () => {
    const source = '# config\naudit: {block_threshold: CRITICAL}\nhooks: {}\nplugins: {}\nmcp: {}\nextras: []\nagents: {}\nskills: {}\ntargets: [codex, claude]\nmode: merge\n# shared sources\nsources:\n  skills: /skills\n  extras: /extras\nignore: ["**/.DS_Store", "**/.git/**"]\ncustom_b: true\ncustom_a: false\n';
    const formatted = formatYaml(source, { organizeConfig: true });
    expect(Object.keys(parse(formatted))).toEqual([
      'sources', 'mode', 'targets', 'skills', 'agents', 'extras', 'mcp', 'plugins', 'hooks',
      'ignore', 'audit', 'custom_b', 'custom_a',
    ]);
    expect(formatted).toContain('# shared sources\nsources:');
    expect(formatted).toContain('\n\nmode: merge\n\ntargets: [codex, claude]\n\n');
    expect(formatted).toContain('# config');
    expect(parse(formatted)).toEqual(parse(source));
    expect(formatYaml(formatted, { organizeConfig: true })).toBe(formatted);
  });
  it('preserves anchors and aliases when organizing config sections', () => {
    const source = 'hooks: &shared {enabled: true}\nextras: *shared\nmode: merge\n';
    const formatted = formatYaml(source, { organizeConfig: true });
    expect(parse(formatted)).toEqual(parse(source));
    expect(formatted).toContain('&shared');
    expect(formatted).toContain('*shared');
    expect(formatYaml(formatted, { organizeConfig: true })).toBe(formatted);
  });
  it('orders profiles after targets without dropping future fields or comments', () => {
    const source = '# named selections\nprofiles:\n  work:\n    skills: {include: [web*]}\n    future: {enabled: true}\nextras: []\ntargets: [claude]\n';
    const formatted = formatYaml(source, { organizeConfig: true });
    expect(Object.keys(parse(formatted))).toEqual(['targets', 'profiles', 'extras']);
    expect(parse(formatted)).toEqual(parse(source));
    expect(formatted).toContain('# named selections');
    expect(formatted).toContain('future: {enabled: true}');
    expect(formatYaml(formatted, { organizeConfig: true })).toBe(formatted);
  });
  it.each([
    '# yaml-language-server: $schema=https://example.com/config.schema.json\n# MCP servers\nmcp: {}\nsources: {skills: /skills}\n',
    'sources: {skills: /skills}\n\n# yaml-language-server: $schema=https://example.com/config.schema.json\n# MCP servers\nmcp: {}\n',
    '# yaml-language-server: $schema=https://example.com/config.schema.json\n\n# MCP servers\nmcp: {}\nsources: {skills: /skills}\n',
  ])('keeps the schema directive at the top while retaining section comments', (source) => {
    const formatted = formatYaml(source, { organizeConfig: true });
    expect(formatted.startsWith('# yaml-language-server: $schema=https://example.com/config.schema.json\n')).toBe(true);
    expect(formatted.match(/yaml-language-server/g)).toHaveLength(1);
    expect(formatted).toContain('# MCP servers\nmcp: {}');
    expect(parse(formatted)).toEqual(parse(source));
    expect(formatYaml(formatted, { organizeConfig: true })).toBe(formatted);
  });
});
