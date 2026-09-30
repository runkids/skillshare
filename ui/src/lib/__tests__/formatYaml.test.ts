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
});
