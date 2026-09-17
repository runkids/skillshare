export function formatAgentDisplayName(flatName: string): string {
  return flatName.replace(/__/g, '/').replace(/\.md$/i, '');
}

export function formatSkillDisplayName(flatName: string): string {
  return flatName.replace(/__/g, '/');
}

export function formatTrackedRepoName(name: string): string {
  return name.replace(/^_/, '').replace(/__/g, '/');
}

/** Detail page URL. Skills and agents live under their own top-level routes. */
export function resourceHref(resource: { flatName: string; kind: 'skill' | 'agent' }): string {
  return `/${resource.kind === 'agent' ? 'agents' : 'skills'}/${encodeURIComponent(resource.flatName)}`;
}
