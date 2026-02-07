import { EditorView } from '@codemirror/view';

/** Shared hand-drawn CodeMirror theme used across the Skillshare UI */
export const handTheme = EditorView.theme({
  '&': {
    fontSize: '14px',
    fontFamily: "'Courier New', monospace",
    backgroundColor: 'var(--color-paper-warm)',
    border: 'none',
    borderRadius: '0',
  },
  '&.cm-focused': {
    outline: 'none',
  },
  '.cm-content': {
    caretColor: 'var(--color-pencil)',
    padding: '8px 0',
  },
  '.cm-cursor': {
    borderLeftColor: 'var(--color-pencil)',
    borderLeftWidth: '2px',
  },
  '.cm-gutters': {
    backgroundColor: 'var(--color-paper)',
    color: 'var(--color-muted-dark)',
    border: 'none',
    borderRight: '2px dashed var(--color-muted)',
    fontFamily: "'Patrick Hand', cursive",
    fontSize: '13px',
  },
  '.cm-activeLineGutter': {
    backgroundColor: 'var(--color-postit)',
    color: 'var(--color-pencil)',
  },
  '.cm-activeLine': {
    backgroundColor: 'rgba(255, 249, 196, 0.3)',
  },
  '.cm-selectionBackground': {
    backgroundColor: 'var(--color-postit) !important',
  },
  '&.cm-focused .cm-selectionBackground': {
    backgroundColor: 'var(--color-postit-dark) !important',
  },
  '.cm-matchingBracket': {
    backgroundColor: 'var(--color-postit)',
    outline: '1px solid var(--color-blue)',
  },
  '.cm-searchMatch': {
    backgroundColor: 'var(--color-postit-dark)',
    borderRadius: '2px',
  },
  '.cm-searchMatch.cm-searchMatch-selected': {
    backgroundColor: 'var(--color-info-light)',
  },
  // Syntax colors
  '.cm-atom': { color: 'var(--color-danger)' },
  '.cm-number': { color: 'var(--color-warning)' },
  '.cm-keyword': { color: 'var(--color-blue)' },
  '.cm-string': { color: 'var(--color-success)' },
  '.cm-comment': { color: 'var(--color-muted-dark)', fontStyle: 'italic' },
  '.cm-meta': { color: 'var(--color-blue-light)' },
  '.cm-propertyName': { color: 'var(--color-blue)' },
  '.cm-variableName': { color: 'var(--color-pencil)' },
  '.cm-typeName': { color: 'var(--color-warning)' },
  '.cm-bool': { color: 'var(--color-accent)' },
  '.cm-definition': { color: 'var(--color-blue)' },
});
