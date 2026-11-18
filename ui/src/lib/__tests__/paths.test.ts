import { describe, expect, it } from 'vitest';
import { fileName, shortenPath } from '../paths';

describe('shortenPath', () => {
  it('keeps both ends of a long local path', () => {
    expect(shortenPath('/Users/me/Developer/Apps/github/skillshare/sources/verify-demo')).toBe(
      '~/Developer/…/sources/verify-demo',
    );
  });
  it('leaves short paths, owner/repo and URLs alone', () => {
    for (const source of [
      '~/plugins/demo',
      'runkids/docs-helper',
      'https://github.com/runkids/docs-helper/tree/main/plugins/demo',
    ])
      expect(shortenPath(source)).toBe(source);
  });
});

describe('fileName', () => {
  it('takes the last segment of a Windows path', () => {
    expect(fileName('C:\\Users\\Public\\sstest\\.claude\\CLAUDE.md')).toBe('CLAUDE.md');
  });
  it('takes the last segment of a POSIX path', () => {
    expect(fileName('/home/me/.codex/AGENTS.md')).toBe('AGENTS.md');
  });
});
