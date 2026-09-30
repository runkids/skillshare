/** Where a top-level key such as `hooks:` starts in a YAML document, or undefined when it is absent. */
export function yamlKeyOffset(text: string, key: string): number | undefined {
  const escaped = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp(`^${escaped}:`, 'm').exec(text)?.index;
}
