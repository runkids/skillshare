export type Platform = 'github' | 'gitlab' | 'bitbucket' | 'other';

export interface ParsedRemoteURL {
  owner: string;
  repo: string;
  ownerRepo: string;
  hostname: string;
  platform: Platform;
  webURL: string | null;
}

const PLATFORM_MAP: Record<string, Platform> = {
  'github.com': 'github',
  'gitlab.com': 'gitlab',
  'bitbucket.org': 'bitbucket',
};

/**
 * Parse a git remote URL into structured components.
 * Supports SSH (git@host:owner/repo.git, ssh://git@host/owner/repo.git)
 * and HTTPS (https://host/owner/repo.git) formats.
 */
export function parseRemoteURL(url: string | undefined | null): ParsedRemoteURL | null {
  if (!url) return null;

  let hostname = '';
  let path = '';

  // ssh://git@host/owner/repo.git
  const sshSchemeMatch = url.match(/^ssh:\/\/[^@]+@([^/]+)\/(.+)$/);
  if (sshSchemeMatch) {
    hostname = sshSchemeMatch[1];
    path = sshSchemeMatch[2];
  }

  // git@host:owner/repo.git
  if (!hostname) {
    const sshMatch = url.match(/^[^@]+@([^:]+):(.+)$/);
    if (sshMatch) {
      hostname = sshMatch[1];
      path = sshMatch[2];
    }
  }

  // https://host/owner/repo.git
  if (!hostname) {
    try {
      const parsed = new URL(url);
      hostname = parsed.hostname;
      path = parsed.pathname.replace(/^\//, '');
    } catch {
      return null;
    }
  }

  if (!hostname || !path) return null;

  // Strip .git suffix
  path = path.replace(/\.git$/, '');

  // Extract owner/repo (first two path segments)
  const segments = path.split('/').filter(Boolean);
  if (segments.length < 2) return null;

  const owner = segments[0];
  const repo = segments[1];
  const platform = PLATFORM_MAP[hostname] ?? 'other';
  const webURL = platform !== 'other' ? `https://${hostname}/${owner}/${repo}` : null;

  return {
    owner,
    repo,
    ownerRepo: `${owner}/${repo}`,
    hostname,
    platform,
    webURL,
  };
}
