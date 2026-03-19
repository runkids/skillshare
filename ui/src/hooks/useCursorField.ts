import { useState, useEffect, useRef, useMemo } from 'react';
import { EditorView, type ViewUpdate } from '@codemirror/view';

/** Extract YAML key from a line, handling both plain keys and list item keys.
 *  "  name: foo"      → "name"
 *  "  - name: foo"    → "name"
 *  "  - path: /x"     → "path"
 *  "  targets:"       → "targets"
 */
function extractKey(line: string): string | null {
  // Try plain key first: "  key: value"
  const plain = line.match(/^\s*([a-zA-Z_][\w.-]*)\s*:/);
  if (plain) return plain[1];
  // Try list item key: "  - key: value"
  const listItem = line.match(/^\s*-\s+([a-zA-Z_][\w.-]*)\s*:/);
  if (listItem) return listItem[1];
  return null;
}

export function resolveFieldPath(lines: string[], lineIndex: number): string | null {
  if (lineIndex < 0 || lineIndex >= lines.length) return null;

  const targetLine = lines[lineIndex];
  const targetIndent = targetLine.search(/\S/);
  if (targetIndent < 0) return null;

  const key = extractKey(targetLine);
  if (!key) return null;

  const parts: string[] = [key];
  let currentIndent = targetIndent;

  for (let i = lineIndex - 1; i >= 0; i--) {
    const line = lines[i];
    const indent = line.search(/\S/);
    if (indent < 0) continue;
    if (indent < currentIndent) {
      const parentKey = extractKey(line);
      if (parentKey) {
        parts.unshift(parentKey);
        currentIndent = indent;
      }
    }
    if (currentIndent === 0) break;
  }

  return parts.join('.');
}

export function useCursorField() {
  const [fieldPath, setFieldPath] = useState<string | null>(null);
  const [cursorLine, setCursorLine] = useState(1);
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const extension = useMemo(
    () =>
      EditorView.updateListener.of((update: ViewUpdate) => {
        if (update.selectionSet || update.docChanged) {
          const pos = update.state.selection.main.head;
          const line = update.state.doc.lineAt(pos);
          const state = update.state;
          const lineNumber = line.number;

          clearTimeout(timerRef.current);
          timerRef.current = setTimeout(() => {
            const allLines = state.doc.toString().split('\n');
            setCursorLine(lineNumber);
            setFieldPath(resolveFieldPath(allLines, lineNumber - 1));
          }, 150);
        }
      }),
    [],
  );

  useEffect(() => {
    return () => clearTimeout(timerRef.current);
  }, []);

  return { fieldPath, cursorLine, extension };
}
