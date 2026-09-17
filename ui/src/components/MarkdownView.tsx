import { Fragment, isValidElement, useMemo, type ReactNode } from 'react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { parseSkillMarkdown } from '../lib/frontmatter';
import { highlightArgs } from '../lib/highlightArgs';
import { highlightLines } from '../lib/highlight';
import { useT } from '../i18n';
import SegmentedControl from './SegmentedControl';

const base: Components = {
  p: ({ children }) => <p>{highlightArgs(children)}</p>,
  li: ({ children, className }) => <li className={className}>{highlightArgs(children)}</li>,
  pre: ({ children }) => {
    const code = isValidElement<{ className?: string; children?: ReactNode }>(children) ? children.props : undefined;
    const lang = code?.className?.match(/language-([\w+-]+)/)?.[1] ?? '';
    const lines = highlightLines(String(code?.children ?? '').replace(/\n$/, ''), lang);
    return (
      <div className="ss-pre">
        {lang && <span className="lang">{lang}</span>}
        <pre><code>{lines.map((l, i) => <Fragment key={i}>{i > 0 && '\n'}{l}</Fragment>)}</code></pre>
      </div>
    );
  },
  table: ({ children }) => <div className="ss-tbl"><table>{children}</table></div>,
};

const cell = (v: unknown): string =>
  v == null ? '' : Array.isArray(v) ? v.map(cell).join(', ') : typeof v === 'object' ? JSON.stringify(v) : String(v);

interface Props {
  children: string;
  components?: Components;
  /** Larger headings, for a full page document. */
  size?: 'lg';
  className?: string;
}

/** Rendered markdown. A frontmatter block at the top is shown as a key/value table, like GitHub does. */
export default function MarkdownView({ children, components, size, className = '' }: Props) {
  const { rows, body } = useMemo(() => {
    if (!/^---\r?\n/.test(children)) return { rows: [], body: children };
    const parsed = parseSkillMarkdown(children);
    const rows = Object.entries(parsed.frontmatter).flatMap(([k, v]) =>
      v && typeof v === 'object' && !Array.isArray(v)
        ? Object.entries(v).map(([sub, x]) => [`${k}.${sub}`, cell(x)])
        : [[k, cell(v)]]);
    return { rows, body: parsed.body };
  }, [children]);

  return (
    <div className={`ss-prose ${size ?? ''} ${className}`}>
      {rows.length > 0 && (
        <div className="ss-tbl fm">
          <table>
            <tbody>
              {rows.map(([k, v]) => <tr key={k}><th>{k}</th><td>{v}</td></tr>)}
            </tbody>
          </table>
        </div>
      )}
      <Markdown remarkPlugins={[remarkGfm]} components={{ ...base, ...components }}>{body}</Markdown>
    </div>
  );
}

/** Preview / Raw switch shown next to every rendered markdown file. */
export function ViewToggle({ raw, onChange }: { raw: boolean; onChange: (raw: boolean) => void }) {
  const t = useT();
  return (
    <SegmentedControl
      value={raw ? 'raw' : 'preview'}
      onChange={(v) => onChange(v === 'raw')}
      options={[
        { value: 'preview', label: t('markdownView.preview') },
        { value: 'raw', label: t('markdownView.raw') },
      ]}
    />
  );
}
