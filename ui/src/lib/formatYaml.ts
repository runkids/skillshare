import { parseDocument } from 'yaml';

/** Normalize indentation without losing comments, scalar types, anchors, or flow/block style. */
export function formatYaml(source: string): string {
  const doc = parseDocument(source);
  if (doc.errors.length) throw new Error(doc.errors[0].message);
  return doc.toString({ indent: 2, lineWidth: 0, flowCollectionPadding: false });
}
