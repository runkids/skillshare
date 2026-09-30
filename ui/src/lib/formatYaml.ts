import { isCollection, parseDocument, visit } from 'yaml';

/**
 * Normalize indentation without losing comments, scalar types or anchors. Flow style is kept, except
 * that `expandNested` turns a flow collection holding another non-empty collection into block style,
 * together with everything inside it, so a one-line `{a: {b: [...]}}` becomes readable while short
 * lists like `[claude, codex]` under block keys stay as written.
 */
export function formatYaml(source: string, { expandNested = false } = {}): string {
  const doc = parseDocument(source);
  if (doc.errors.length) throw new Error(doc.errors[0].message);
  if (expandNested) {
    const filled = (node: unknown) => isCollection(node) && node.items.length > 0;
    const expanded = new Set<unknown>();
    // visit walks parents before children, so an expanded ancestor is known when its contents are reached.
    visit(doc, {
      Collection(_, node, path) {
        if (!node.flow || node.items.length === 0) return;
        const nested = node.items.some((item) => filled(item) || filled((item as { value?: unknown }).value));
        if (nested || path.some((ancestor) => expanded.has(ancestor))) {
          node.flow = false;
          expanded.add(node);
        }
      },
    });
  }
  return doc.toString({ indent: 2, lineWidth: 0, flowCollectionPadding: false });
}
