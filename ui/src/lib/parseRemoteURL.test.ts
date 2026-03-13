import { describe, it, expect } from 'vitest';
import { parseRemoteURL } from './parseRemoteURL';

describe('parseRemoteURL', () => {
  it('parses SSH git@host:owner/repo.git format', () => {
    const result = parseRemoteURL('git@github.com:runkids/my-skills.git');
    expect(result).toEqual({
      owner: 'runkids',
      repo: 'my-skills',
      ownerRepo: 'runkids/my-skills',
      hostname: 'github.com',
      platform: 'github',
      webURL: 'https://github.com/runkids/my-skills',
    });
  });

  it('parses SSH without .git suffix', () => {
    const result = parseRemoteURL('git@github.com:runkids/my-skills');
    expect(result!.ownerRepo).toBe('runkids/my-skills');
    expect(result!.platform).toBe('github');
  });

  it('parses ssh:// scheme format', () => {
    const result = parseRemoteURL('ssh://git@github.com/runkids/my-skills.git');
    expect(result!.ownerRepo).toBe('runkids/my-skills');
    expect(result!.hostname).toBe('github.com');
    expect(result!.platform).toBe('github');
  });

  it('parses HTTPS format', () => {
    const result = parseRemoteURL('https://github.com/runkids/my-skills.git');
    expect(result).toEqual({
      owner: 'runkids',
      repo: 'my-skills',
      ownerRepo: 'runkids/my-skills',
      hostname: 'github.com',
      platform: 'github',
      webURL: 'https://github.com/runkids/my-skills',
    });
  });

  it('parses HTTPS without .git suffix', () => {
    const result = parseRemoteURL('https://github.com/runkids/my-skills');
    expect(result!.ownerRepo).toBe('runkids/my-skills');
  });

  it('detects GitLab', () => {
    const result = parseRemoteURL('git@gitlab.com:org/project.git');
    expect(result!.platform).toBe('gitlab');
    expect(result!.webURL).toBe('https://gitlab.com/org/project');
  });

  it('detects Bitbucket', () => {
    const result = parseRemoteURL('git@bitbucket.org:team/repo.git');
    expect(result!.platform).toBe('bitbucket');
    expect(result!.webURL).toBe('https://bitbucket.org/team/repo');
  });

  it('returns "other" for self-hosted instances', () => {
    const result = parseRemoteURL('git@gitlab.mycompany.com:team/repo.git');
    expect(result!.platform).toBe('other');
    expect(result!.webURL).toBeNull();
    expect(result!.ownerRepo).toBe('team/repo');
  });

  it('returns "other" for self-hosted GitHub Enterprise', () => {
    const result = parseRemoteURL('git@github.mycompany.com:team/repo.git');
    expect(result!.platform).toBe('other');
    expect(result!.webURL).toBeNull();
  });

  it('returns null for empty/invalid input', () => {
    expect(parseRemoteURL('')).toBeNull();
    expect(parseRemoteURL(undefined)).toBeNull();
    expect(parseRemoteURL(null)).toBeNull();
  });

  it('returns null for malformed URLs', () => {
    expect(parseRemoteURL('/just/a/path')).toBeNull();
    expect(parseRemoteURL('not-a-url')).toBeNull();
    expect(parseRemoteURL('https://github.com')).toBeNull();
  });
});
