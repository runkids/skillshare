import { useMemo } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { json } from '@codemirror/lang-json';
import { syntaxHighlighting } from '@codemirror/language';
import { EditorView } from '@codemirror/view';
import { classHighlighter } from '@lezer/highlight';

// Colours come from the .ss-code tok-* rules, so the editor matches CodeView in every theme
const chrome = EditorView.theme({
  '&': { backgroundColor: 'transparent', color: 'var(--ink)', fontSize: '12.5px' },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': { fontFamily: 'var(--fm)', lineHeight: '1.65' },
  '.cm-content': { padding: '10px 0', caretColor: 'var(--ink)' },
  '.cm-gutters': { backgroundColor: 'transparent', border: 'none', color: 'var(--ink-3)', paddingLeft: '4px' },
  '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'transparent' },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection': { backgroundColor: 'var(--accent-bg) !important' },
  '.cm-matchingBracket': { backgroundColor: 'var(--accent-bg)', outline: 'none' },
  '.cm-placeholder': { color: 'var(--ink-3)' },
});

interface Props {
  value: string;
  onChange: (value: string) => void;
  /** 'json' highlights and auto-indents; anything else edits plain text. */
  lang?: string;
  placeholder?: string;
  ariaLabel: string;
  disabled?: boolean;
  className?: string;
}

export default function CodeEditor({ value, onChange, lang = '', placeholder, ariaLabel, disabled = false, className = '' }: Props) {
  const extensions = useMemo(
    () => [chrome, syntaxHighlighting(classHighlighter), EditorView.contentAttributes.of({ 'aria-label': ariaLabel }), ...(lang === 'json' ? [json()] : [])],
    [lang, ariaLabel],
  );
  return (
    <div className={`ss-code !overflow-hidden !p-0 !whitespace-normal focus-within:!border-[var(--accent)] ${className}`}>
      <CodeMirror
        value={value}
        onChange={onChange}
        extensions={extensions}
        theme="none"
        placeholder={placeholder}
        editable={!disabled}
        minHeight="140px"
        maxHeight="320px"
        basicSetup={{
          lineNumbers: true,
          foldGutter: false,
          highlightActiveLine: false,
          highlightActiveLineGutter: false,
          highlightSelectionMatches: false,
          autocompletion: false,
          searchKeymap: false,
          bracketMatching: true,
          closeBrackets: true,
          indentOnInput: true,
          tabSize: 2,
        }}
      />
    </div>
  );
}
