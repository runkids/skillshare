import type { ReactNode } from 'react';
import { classHighlighter, highlightCode } from '@lezer/highlight';
import { javascriptLanguage, jsxLanguage, tsxLanguage, typescriptLanguage } from '@codemirror/lang-javascript';
import { jsonLanguage } from '@codemirror/lang-json';
import { pythonLanguage } from '@codemirror/lang-python';
import { yamlLanguage } from '@codemirror/lang-yaml';

const PARSERS: Record<string, typeof jsonLanguage.parser> = {
  js: javascriptLanguage.parser, mjs: javascriptLanguage.parser, cjs: javascriptLanguage.parser, javascript: javascriptLanguage.parser,
  jsx: jsxLanguage.parser, ts: typescriptLanguage.parser, typescript: typescriptLanguage.parser, tsx: tsxLanguage.parser,
  json: jsonLanguage.parser, py: pythonLanguage.parser, python: pythonLanguage.parser, yaml: yamlLanguage.parser, yml: yamlLanguage.parser,
};

const MARKDOWN = new Set(['md', 'markdown', 'mdx']);

/** Language key from a file name or a code fence info string. */
export function langOf(name: string) {
  const s = name.trim().toLowerCase();
  return s.includes('.') ? s.slice(s.lastIndexOf('.') + 1) : s;
}

export const isMarkdown = (name: string) => MARKDOWN.has(langOf(name));

/**
 * Splits code into one node per line with `tok-*` spans. Languages without a parser come back as plain lines.
 * ponytail: markdown uses a line tokenizer, not a full parser; add @codemirror/lang-markdown if it misreads real files.
 */
export function highlightLines(code: string, lang = ''): ReactNode[] {
  const key = langOf(lang);
  if (MARKDOWN.has(key)) return markdownLines(code.split('\n'));
  const parser = PARSERS[key];
  if (!parser) return code.split('\n');
  const lines: ReactNode[][] = [[]];
  highlightCode(
    code,
    parser.parse(code),
    classHighlighter,
    (text, classes) => {
      const line = lines[lines.length - 1];
      line.push(classes ? <span key={line.length} className={classes}>{text}</span> : text);
    },
    () => lines.push([]),
  );
  return lines;
}

function markdownLines(lines: string[]): ReactNode[] {
  const out: ReactNode[] = [];
  // Code (frontmatter, fences) is highlighted as one block so multi-line strings keep their colors
  const block = (from: number, to: number, lang: string) => {
    if (to > from) out.push(...highlightLines(lines.slice(from, to).join('\n'), lang));
  };
  let i = 0;
  if (lines[0]?.trim() === '---') {
    const end = lines.findIndex((l, j) => j > 0 && l.trim() === '---');
    if (end > 0) {
      out.push(<span className="tok-meta">{lines[0]}</span>);
      block(1, end, 'yaml');
      out.push(<span className="tok-meta">{lines[end]}</span>);
      i = end + 1;
    }
  }
  while (i < lines.length) {
    const line = lines[i];
    const fence = line.match(/^\s*(`{3,}|~{3,})\s*([\w+-]*)/);
    if (fence) {
      const close = lines.findIndex((l, j) => j > i && l.trim().startsWith(fence[1]));
      const end = close < 0 ? lines.length : close;
      out.push(<span className="tok-meta">{line}</span>);
      block(i + 1, end, fence[2]);
      if (close >= 0) out.push(<span className="tok-meta">{lines[close]}</span>);
      i = end + 1;
      continue;
    }
    out.push(markdownLine(line));
    i++;
  }
  return out;
}

function markdownLine(line: string): ReactNode {
  if (/^\s{0,3}#{1,6}\s/.test(line)) return <span className="tok-heading">{line}</span>;
  if (/^\s*>/.test(line)) return <span className="tok-quote">{line}</span>;
  if (/^\s*(-{3,}|\*{3,}|_{3,}|\|?(\s*:?-+:?\s*\|)+\s*:?-*:?\s*)$/.test(line)) return <span className="tok-meta">{line}</span>;
  const list = line.match(/^(\s*)([-*+]|\d+[.)])(\s+(?:\[[ xX]\]\s+)?)(.*)$/);
  if (list) {
    return <>{list[1]}<span className="tok-list">{list[2]}{list[3]}</span>{inline(list[4])}</>;
  }
  return inline(line);
}

const INLINE = /(`[^`]+`|\*\*[^*]+\*\*|\[[^\]]*\]\([^)\s]*\)|\$ARGUMENTS\b|\|)/;

function inline(text: string): ReactNode {
  const parts = text.split(INLINE);
  if (parts.length === 1) return text;
  return parts.map((p, i) => {
    if (i % 2 === 0) return p;
    const cls = p === '|' ? 'tok-punctuation'
      : p.startsWith('`') ? 'tok-string'
        : p.startsWith('**') ? 'tok-strong'
          : p.startsWith('[') ? 'tok-link'
            : 'tok-arg';
    return <span key={i} className={cls}>{p}</span>;
  });
}
