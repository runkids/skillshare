export function formatAgentDisplayName(flatName: string): string {
  return flatName.replace(/__/g, '/').replace(/\.md$/i, '');
}

export function formatSkillDisplayName(flatName: string): string {
  return flatName.replace(/__/g, '/');
}

export function formatTrackedRepoName(name: string): string {
  return name.replace(/^_/, '').replace(/__/g, '/');
}

/** Folder a skill or agent lives in: its tracked repo, else its full parent path; '' at the source root. */
export function folderOf(resource: {
  relPath: string;
  isInRepo: boolean;
  repoPath?: string;
}): string {
  if (resource.isInRepo) return resource.repoPath ?? resource.relPath.split('/')[0];
  const i = resource.relPath.lastIndexOf('/');
  return i > 0 ? resource.relPath.slice(0, i) : '';
}

/** Detail page URL. Skills and agents live under their own top-level routes. */
export function resourceHref(resource: { flatName: string; kind: 'skill' | 'agent' }): string {
  return `/${resource.kind === 'agent' ? 'agents' : 'skills'}/${encodeURIComponent(resource.flatName)}`;
}
