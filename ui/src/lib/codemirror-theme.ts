import { EditorView } from '@codemirror/view';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { tags } from '@lezer/highlight';

/** Editor chrome (gutters, cursor, selection, etc.). The `.ss-code` wrapper paints the background. */
const handEditorTheme = EditorView.theme({
  '&': {
    fontSize: '12.5px',
    lineHeight: '1.65',
    fontFamily: 'var(--fm)',
    backgroundColor: 'transparent',
    border: 'none',
    borderRadius: '0',
    color: 'var(--ink-2)',
  },
  '&.cm-focused': { outline: 'none' },
  '.cm-content': {
    caretColor: 'var(--ink)',
    padding: '10px 0',
  },
  '.cm-cursor': {
    borderLeftColor: 'var(--ink)',
    borderLeftWidth: '2px',
  },
  '.cm-gutters': {
    backgroundColor: 'transparent',
    color: 'var(--ink-3)',
    border: 'none',
    borderRight: '1px solid var(--line-soft)',
    fontFamily: 'var(--fm)',
    fontSize: '12px',
  },
  '.cm-activeLineGutter': {
    backgroundColor: 'var(--accent-bg)',
    color: 'var(--ink)',
  },
  // Softer than the selection colour, so a selected range still stands out on the active line
  '.cm-activeLine': {
    backgroundColor: 'var(--accent-bg)',
  },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
    backgroundColor: 'var(--sel) !important',
    color: 'var(--sel-ink)',
  },
  '.cm-matchingBracket': {
    backgroundColor: 'var(--accent-bg)',
    outline: '1px solid var(--accent)',
  },
  '.cm-searchMatch': {
    backgroundColor: 'var(--warn-bg)',
    borderRadius: '2px',
  },
  '.cm-searchMatch.cm-searchMatch-selected': {
    backgroundColor: 'var(--accent-bg)',
  },
});

/** Syntax colors — same mapping as the `.tok-*` classes in components.css, so static and live code match. */
const handHighlightStyle = HighlightStyle.define([
  { tag: [tags.keyword, tags.link], color: 'var(--accent)' },
  { tag: [tags.string, tags.special(tags.string), tags.attributeValue], color: 'var(--ok)' },
  { tag: [tags.number, tags.bool, tags.atom, tags.typeName, tags.className], color: 'var(--warn)' },
  { tag: [tags.comment, tags.meta, tags.punctuation, tags.quote], color: 'var(--ink-3)', fontStyle: 'italic' },
  { tag: [tags.propertyName, tags.attributeName], color: 'var(--c-mcp)' },
  { tag: [tags.definition(tags.variableName), tags.macroName, tags.tagName], color: 'var(--c-agent)' },
  { tag: [tags.variableName, tags.operator], color: 'var(--ink)' },
  { tag: tags.url, color: 'var(--accent)' },
  { tag: [tags.heading, tags.strong], fontWeight: '600' },
]);

/** Combined theme: editor chrome + syntax highlighting */
export const handTheme = [handEditorTheme, syntaxHighlighting(handHighlightStyle)];
