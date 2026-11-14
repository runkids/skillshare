import { isCollection, isMap, isNode, isScalar, parseDocument, visit } from 'yaml';

const CONFIG_SECTIONS = [
  'sources', 'source', 'agents_source', 'extras_source', 'git_root', 'mode', 'target_naming',
  'targets', 'profiles', 'skills', 'agents', 'extras', 'mcp', 'plugins', 'hooks',
  'updates', 'projects', 'hub', 'log', 'context_budget', 'tui', 'gitlab_hosts', 'azure_hosts', 'ignore', 'audit',
];

/**
 * Normalize indentation without losing comments, scalar types or anchors. Flow style is kept, except
 * that `expandNested` turns a flow collection holding another non-empty collection into block style,
 * together with everything inside it, so a one-line `{a: {b: [...]}}` becomes readable while short
 * lists like `[claude, codex]` under block keys stay as written.
 */
export function formatYaml(source: string, { expandNested = false, organizeConfig = false } = {}): string {
  const doc = parseDocument(source);
  if (doc.errors.length) throw new Error(doc.errors[0].message);
  if (organizeConfig && isMap(doc.contents)) {
    const schemaComments: string[] = [];
    const extractSchema = (comment: string | null | undefined) => {
      if (!comment) return null;
      const remaining = comment.split('\n').filter((line) => {
        if (!/^\s*yaml-language-server:\s*\$schema=/.test(line)) return true;
        schemaComments.push(line);
        return false;
      });
      return remaining.join('\n') || null;
    };
    // Schema directives describe the document, even when the parser attaches them to a key.
    doc.commentBefore = extractSchema(doc.commentBefore);
    doc.contents.commentBefore = extractSchema(doc.contents.commentBefore);
    for (const pair of doc.contents.items) {
      if (isNode(pair.key)) pair.key.commentBefore = extractSchema(pair.key.commentBefore);
    }
    if (schemaComments.length) {
      doc.commentBefore = [...schemaComments, doc.commentBefore].filter(Boolean).join('\n');
    }
    const original = [...doc.contents.items];
    const aliases: (() => boolean)[] = [];
    visit(doc, {
      Alias(_, node) {
        const target = node.resolve(doc);
        aliases.push(() => node.resolve(doc) === target);
      },
    });
    const rank = (key: unknown) => {
      const index = isScalar(key) ? CONFIG_SECTIONS.indexOf(String(key.value)) : -1;
      return index < 0 ? CONFIG_SECTIONS.length : index;
    };
    doc.contents.items.sort((a, b) => rank(a.key) - rank(b.key));
    // Keep the original order if moving sections would change or break an alias reference.
    if (aliases.some((unchanged) => !unchanged())) doc.contents.items = original;
    doc.contents.items.forEach((pair, index) => {
      if (isNode(pair.key)) pair.key.spaceBefore = index > 0;
    });
  }
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
