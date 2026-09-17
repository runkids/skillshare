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
  it('rejects invalid YAML instead of replacing it', () => {
    expect(() => formatYaml('mcp: [')).toThrow();
  });
});
