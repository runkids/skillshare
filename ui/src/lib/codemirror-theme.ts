import { EditorView } from '@codemirror/view';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { tags } from '@lezer/highlight';

/** Editor chrome (gutters, cursor, selection, etc.) */
const handEditorTheme = EditorView.theme({
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
    fontFamily: "'SFMono-Regular', Menlo, Consolas, 'Courier New', monospace",
    fontSize: '13px',
  },
  '.cm-activeLineGutter': {
    backgroundColor: 'var(--color-postit)',
    color: 'var(--color-pencil)',
  },
  '.cm-activeLine': {
    backgroundColor: 'var(--color-postit)',
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
});

/** Syntax highlighting colors — uses CSS custom properties so dark mode auto-switches */
const handHighlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: 'var(--color-blue)' },
  { tag: tags.atom, color: 'var(--color-danger)' },
  { tag: tags.bool, color: 'var(--color-accent)' },
  { tag: tags.number, color: 'var(--color-warning)' },
  { tag: tags.string, color: 'var(--color-success)' },
  { tag: tags.comment, color: 'var(--color-muted-dark)', fontStyle: 'italic' },
  { tag: tags.meta, color: 'var(--color-blue-light)' },
  { tag: tags.propertyName, color: 'var(--color-blue)' },
  { tag: tags.variableName, color: 'var(--color-pencil)' },
  { tag: tags.typeName, color: 'var(--color-warning)' },
  { tag: tags.definition(tags.variableName), color: 'var(--color-blue)' },
  { tag: tags.tagName, color: 'var(--color-accent)' },
  { tag: tags.attributeName, color: 'var(--color-blue)' },
  { tag: tags.attributeValue, color: 'var(--color-success)' },
  { tag: tags.url, color: 'var(--color-blue-light)' },
  { tag: tags.operator, color: 'var(--color-pencil)' },
  { tag: tags.punctuation, color: 'var(--color-pencil-light)' },
]);

/** Combined theme: editor chrome + syntax highlighting */
export const handTheme = [handEditorTheme, syntaxHighlighting(handHighlightStyle)];
