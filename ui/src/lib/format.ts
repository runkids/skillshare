export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  const mb = kb / 1024;
  return `${mb.toFixed(1)} MB`;
}

/** A version as people write it: `v` before one that starts with a digit, anything else as is. */
export function formatVersion(version?: string): string | undefined {
  return version && /^\d/.test(version) ? `v${version}` : version;
}
