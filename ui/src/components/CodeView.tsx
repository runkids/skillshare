import { useEffect, useMemo, useRef } from 'react';
import { highlightLines, isMarkdown } from '../lib/highlight';

interface Props {
  content: string;
  /** File name or language key; picks the highlighter and turns on wrapping for markdown. */
  lang?: string;
  /** 1-based line to highlight and scroll to. */
  line?: number;
  /** 1-based lines to tint as differing; without `line`, the first one is scrolled to. */
  marks?: number[];
  className?: string;
}

/** Read-only code with line numbers. Markdown wraps (prose lines are long); code scrolls sideways. */
export default function CodeView({ content, lang = '', line = 0, marks, className = '' }: Props) {
  const box = useRef<HTMLDivElement>(null);
  const cur = useRef<HTMLDivElement>(null);
  const lines = useMemo(() => highlightLines(content.replace(/\n$/, ''), lang), [content, lang]);
  const wrap = isMarkdown(lang);
  const digits = String(lines.length).length;
  const marked = useMemo(() => new Set(marks), [marks]);
  const target = line || marks?.[0] || 0;

  useEffect(() => {
    // Scroll the box only, so jumping to a line does not move the page
    if (box.current && cur.current) box.current.scrollTop = cur.current.offsetTop - box.current.clientHeight / 2;
  }, [content, target]);

  return (
    <div ref={box} className={`ss-code relative !overflow-auto ${className}`}>
      <div className={wrap ? '' : 'min-w-max'}>
        {lines.map((node, i) => (
          <div
            key={i}
            ref={i + 1 === target ? cur : undefined}
            className={`grid grid-cols-[auto_minmax(0,1fr)] ${i + 1 === line ? 'cur' : marked.has(i + 1) ? 'mark' : ''}`}
          >
            <span className="ln text-right" style={{ minWidth: `${digits}ch` }}>{i + 1}</span>
            <span className={wrap ? 'whitespace-pre-wrap [overflow-wrap:anywhere]' : ''}>{node}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
