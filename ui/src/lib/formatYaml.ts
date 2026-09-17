import { isCollection, parseDocument, visit } from 'yaml';

/** Format the document without losing comments, scalar types, or anchors. */
export function formatYaml(source: string): string {
  const doc = parseDocument(source);
  if (doc.errors.length) throw new Error(doc.errors[0].message);
  visit(doc, (_key, node) => {
    if (isCollection(node)) node.flow = false;
  });
  return doc.toString({ indent: 2, lineWidth: 0 });
}
