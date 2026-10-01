import { useMemo } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { autocompletion } from '@codemirror/autocomplete';
import type { CompletionSource } from '@codemirror/autocomplete';
import { javascript } from '@codemirror/lang-javascript';
import { json } from '@codemirror/lang-json';
import { yaml } from '@codemirror/lang-yaml';
import { syntaxHighlighting } from '@codemirror/language';
import { linter, lintGutter } from '@codemirror/lint';
import type { Diagnostic } from '@codemirror/lint';
import { Decoration, EditorView, WidgetType } from '@codemirror/view';
import { classHighlighter } from '@lezer/highlight';

// Colours come from the .ss-code tok-* rules, so the editor matches CodeView in every theme
const chrome = EditorView.theme({
  // Gutters stay put while content scrolls sideways, so they inherit the
  // wrapper's background down the chain instead of letting text show through.
  '&': { backgroundColor: 'inherit', color: 'var(--ink)', fontSize: '12.5px' },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': { backgroundColor: 'inherit', fontFamily: 'var(--fm)', lineHeight: '1.65' },
  '.cm-content': { padding: '10px 0', caretColor: 'var(--ink)' },
  '.cm-gutters': { backgroundColor: 'inherit', border: 'none', color: 'var(--ink-3)', paddingLeft: '4px' },
  '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'transparent' },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection': { backgroundColor: 'var(--accent-bg) !important' },
  '.cm-matchingBracket': { backgroundColor: 'var(--accent-bg)', outline: 'none' },
  '.cm-placeholder': { color: 'var(--ink-3)' },
  '.cm-lintRange-error': { backgroundImage: 'none', textDecoration: 'underline wavy var(--bad)', textUnderlineOffset: '3px' },
  '.cm-lintRange-warning': { backgroundImage: 'none', textDecoration: 'underline wavy var(--ink-3)', textUnderlineOffset: '3px' },
  '.cm-tooltip': { backgroundColor: 'var(--surface)', color: 'var(--ink)', border: '1px solid var(--line)', borderRadius: '8px', fontFamily: 'var(--f)', fontSize: '12.5px', overflow: 'hidden' },
  '.cm-tooltip-autocomplete > ul > li': { padding: '3px 10px !important', fontFamily: 'var(--fm)' },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': { backgroundColor: 'var(--sel)', color: 'var(--sel-ink)' },
  '.cm-completionDetail': { marginLeft: '12px', fontFamily: 'var(--f)', fontStyle: 'normal', color: 'var(--ink-3)' },
  '.cm-diagnostic': { padding: '6px 10px', borderLeft: 'none' },
  '.cm-diagnostic-error': { color: 'var(--bad)' },
  '.cm-diagnostic-warning': { color: 'var(--ink-2)' },
  '.cm-marked': { backgroundColor: 'var(--warn-bg)' },
  '.cm-marked .cm-gutterElement, .cm-gutterElement.cm-marked': { color: 'var(--warn)' },
  // A noted line holds its floating note, so a note that does not fit drops under this line, not beside the next.
  '.cm-noted': { display: 'flow-root' },
  '.cm-block': { backgroundColor: 'var(--sunken)', boxShadow: 'inset 3px 0 0 color-mix(in srgb, var(--ink-3) 40%, transparent)' },
  // Floats right of the line's last row: with wrapping it drops below rather than overlap, and a long note is cut short.
  '.cm-note': { float: 'right', maxWidth: '50%', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', paddingLeft: '16px', paddingRight: '16px', fontFamily: 'var(--f)', fontSize: '12px', color: 'var(--ink-3)', userSelect: 'none' },
});

/** How lineDecor styles one line: block tints it, note adds muted text at its right end. */
export type LineDecor = { block?: boolean; note?: string } | null;

class NoteWidget extends WidgetType {
  readonly text: string;
  constructor(text: string) {
    super();
    this.text = text;
  }
  eq(other: NoteWidget) { return other.text === this.text; }
  toDOM() {
    const el = document.createElement('span');
    el.className = 'cm-note';
    el.textContent = this.text;
    return el;
  }
}

const block = Decoration.line({ class: 'cm-block' });
const blockNoted = Decoration.line({ class: 'cm-block cm-noted' });
const noted = Decoration.line({ class: 'cm-noted' });

/** Styles lines from the whole text, recomputed as it changes. */
function decorateLines(lineDecor: (lines: string[]) => LineDecor[]) {
  const build = (state: EditorView['state']) => {
    const lines = Array.from({ length: state.doc.lines }, (_, i) => state.doc.line(i + 1).text);
    const ranges = [];
    for (const [i, d] of lineDecor(lines).entries()) {
      if (!d) continue;
      const line = state.doc.line(i + 1);
      if (d.block || d.note) ranges.push((d.block ? (d.note ? blockNoted : block) : noted).range(line.from));
      if (d.note) ranges.push(Decoration.widget({ widget: new NoteWidget(d.note), side: 1 }).range(line.to));
    }
    return Decoration.set(ranges, true);
  };
  return EditorView.decorations.compute(['doc'], build);
}

const marked = Decoration.line({ class: 'cm-marked' });

/** Tints every line markLine accepts, recomputed as the text changes. */
function markLines(markLine: (text: string) => boolean) {
  const build = (state: EditorView['state']) => {
    const ranges = [];
    for (let n = 1; n <= state.doc.lines; n++) {
      const line = state.doc.line(n);
      if (markLine(line.text)) ranges.push(marked.range(line.from));
    }
    return Decoration.set(ranges);
  };
  return EditorView.decorations.compute(['doc'], build);
}

interface Props {
  value: string;
  onChange: (value: string) => void;
  /** 'json' highlights and auto-indents; 'typescript' and 'javascript' use the JavaScript mode; anything else edits plain text. */
  lang?: string;
  placeholder?: string;
  ariaLabel: string;
  disabled?: boolean;
  className?: string;
  minHeight?: string;
  maxHeight?: string;
  /** Tints the lines it accepts, e.g. tool-specific syntax. Keep it stable (module level). */
  markLine?: (text: string) => boolean;
  /** Per-line styling that needs the whole text (e.g. a managed block). Keep it stable between renders. */
  lineDecor?: (lines: string[]) => LineDecor[];
  /** Wraps long lines instead of scrolling sideways, for prose such as Markdown. */
  wrap?: boolean;
  /** Takes the full height of its parent (a sized flex item) and scrolls inside, instead of min/max height. */
  fill?: boolean;
  /** Marks problems in the text, recomputed as it changes. Keep it stable between renders. */
  lint?: (text: string) => Diagnostic[];
  /** Offers completions as you type. Keep it stable between renders. */
  completions?: CompletionSource;
}

export default function CodeEditor({ value, onChange, lang = '', placeholder, ariaLabel, disabled = false, className = '', minHeight = '140px', maxHeight = '320px', markLine, lineDecor, wrap = false, fill = false, lint, completions }: Props) {
  const extensions = useMemo(
    () => [
      chrome,
      syntaxHighlighting(classHighlighter),
      EditorView.contentAttributes.of({ 'aria-label': ariaLabel }),
      ...(lang === 'json' ? [json()] : []),
      ...(lang === 'yaml' ? [yaml()] : []),
      ...(lang === 'typescript' || lang === 'javascript' ? [javascript({ typescript: lang === 'typescript' })] : []),
      ...(lint ? [linter((view) => lint(view.state.doc.toString()), { delay: 250 }), lintGutter()] : []),
      ...(completions ? [autocompletion({ override: [completions], icons: false })] : []),
      ...(markLine ? [markLines(markLine)] : []),
      ...(lineDecor ? [decorateLines(lineDecor)] : []),
      ...(wrap ? [EditorView.lineWrapping] : []),
    ],
    [lang, ariaLabel, markLine, lineDecor, wrap, lint, completions],
  );
  return (
    <div className={`ss-code !overflow-hidden !p-0 !whitespace-normal focus-within:!border-[var(--accent)] ${className}`}>
      <CodeMirror
        className={`bg-inherit ${fill ? 'h-full' : ''}`}
        value={value}
        onChange={onChange}
        extensions={extensions}
        theme="none"
        placeholder={placeholder}
        editable={!disabled}
        {...(fill ? { height: '100%' } : { minHeight, maxHeight })}
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
