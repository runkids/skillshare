import { describe, expect, it } from 'vitest';
import { parse } from 'yaml';
import { formatYaml } from '../formatYaml';

describe('formatYaml', () => {
  it('expands MCP flow mappings and retains comments and values', () => {
    const source = "# config\nmcp: {servers: {docs: {url: 'https://example.com/mcp', targets: [claude]}}}\n";
    const formatted = formatYaml(source);
    expect(formatted).toContain('# config\nmcp:\n  servers:\n    docs:\n');
    expect(parse(formatted)).toEqual(parse(source));
    expect(formatYaml(formatted)).toBe(formatted);
  });
  it('rejects invalid YAML instead of replacing it', () => {
    expect(() => formatYaml('mcp: [')).toThrow();
  });
});
