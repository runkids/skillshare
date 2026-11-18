import { scalarText } from '../lib/scalarText';
import { Fragment, isValidElement, useMemo, type ReactNode } from 'react';
import { Info, Lightbulb, MessageSquareWarning, OctagonAlert, TriangleAlert } from 'lucide-react';
import Markdown, { type Components, type Options } from 'react-markdown';
import rehypeRaw from 'rehype-raw';
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize';
import remarkBreaks from 'remark-breaks';
import remarkGemoji from 'remark-gemoji';
import remarkGfm from 'remark-gfm';
import { parse as parseYaml } from 'yaml';
import { parseSkillMarkdown } from '../lib/frontmatter';
import { highlightArgs } from '../lib/highlightArgs';
import { highlightLines } from '../lib/highlight';
import { useT } from '../i18n';
import SegmentedControl from './SegmentedControl';

const base: Components = {
  p: ({ children }) => <p>{highlightArgs(children)}</p>,
  li: ({ children, className, id }) => (
    <li className={className} id={id}>
      {highlightArgs(children)}
    </li>
  ),
  pre: ({ children }) => {
    const code = isValidElement<{ className?: string; children?: ReactNode }>(children)
      ? children.props
      : undefined;
    const lang = code?.className?.match(/language-([\w+-]+)/)?.[1] ?? '';
    const lines = highlightLines(
      (typeof code?.children === 'string' ? code.children : '').replace(/\n$/, ''),
      lang,
    );
    return (
      <div className="ss-pre">
        {lang && <span className="lang">{lang}</span>}
        <pre>
          <code>
            {lines.map((l, i) => (
              <Fragment key={i}>
                {i > 0 && '\n'}
                {l}
              </Fragment>
            ))}
          </code>
        </pre>
      </div>
    );
  },
  // Links out open in a new tab so the dashboard stays; in-page ones (footnotes) keep their attributes.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars -- node is react-markdown's syntax tree, not an <a> attribute
  a: ({ node: _node, ...props }) =>
    props.href?.startsWith('#') ? (
      <a {...props} />
    ) : (
      <a {...props} target="_blank" rel="noopener noreferrer" />
    ),
  // Short inline code such as -g or --force never splits at its hyphen; long paths still wrap.
  code: ({ className, children }) => (
    <code
      className={
        typeof children === 'string' && children.length <= 24 ? `${className ?? ''} nw` : className
      }
    >
      {children}
    </code>
  ),
  table: ({ children }) => (
    <div className="ss-tbl">
      <table>{children}</table>
    </div>
  ),
  blockquote: ({ children, ...props }) => {
    const kind = (props as Record<string, unknown>)['data-alert'] as
      | keyof typeof ALERTS
      | undefined;
    if (!kind) return <blockquote>{children}</blockquote>;
    const Icon = ALERTS[kind];
    return (
      <blockquote className={`alert ${kind}`}>
        <p className="alert-title">
          <Icon size={15} aria-hidden="true" />
          {kind[0].toUpperCase() + kind.slice(1)}
        </p>
        {children}
      </blockquote>
    );
  },
};

const ALERTS = {
  note: Info,
  tip: Lightbulb,
  important: MessageSquareWarning,
  warning: TriangleAlert,
  caution: OctagonAlert,
};

type HNode = {
  type: string;
  tagName?: string;
  value?: string;
  properties?: Record<string, unknown>;
  children?: HNode[];
};
const ALERT_MARK = /^\[!(note|tip|important|warning|caution)\]/i;

/** GitHub alerts: a blockquote opening with [!NOTE] and the like gets data-alert, and the marker is dropped. */
function rehypeAlerts() {
  const visit = (node: HNode) => {
    node.children?.forEach(visit);
    if (node.tagName !== 'blockquote' || !node.children) return;
    const p = node.children.find((c) => c.type === 'element');
    const first = p?.tagName === 'p' ? p.children?.[0] : undefined;
    const m = first?.type === 'text' ? ALERT_MARK.exec(first.value ?? '') : null;
    if (!p?.children || !first || !m) return;
    first.value = first.value!.slice(m[0].length);
    const rest = p.children;
    while (
      rest.length &&
      (rest[0].tagName === 'br' || (rest[0].type === 'text' && !rest[0].value!.trim()))
    )
      rest.shift();
    if (rest[0]?.type === 'text') rest[0].value = rest[0].value!.trimStart();
    if (!rest.length) node.children = node.children.filter((c) => c !== p);
    node.properties = { ...node.properties, dataAlert: m[1].toLowerCase() };
  };
  return visit;
}

type MNode = { type: string; value?: string; children?: MNode[]; data?: { hName?: string } };
const CONTAINER_OPEN = /^:::[ \t]*(\w+)[ \t]*(?:\n|$)/;
const CONTAINER_CLOSE = /(?:^|\n):::[ \t]*$/;
const CONTAINER_KIND: Record<string, string> = {
  note: 'NOTE',
  info: 'NOTE',
  tip: 'TIP',
  important: 'IMPORTANT',
  warning: 'WARNING',
  caution: 'CAUTION',
  danger: 'CAUTION',
};

/** ::: warning … ::: containers (VitePress, Docusaurus) become the matching GitHub alert. */
function remarkContainers() {
  return (tree: MNode) => {
    const kids = tree.children ?? [];
    for (let i = 0; i < kids.length; i++) {
      const head = kids[i].type === 'paragraph' ? kids[i].children?.[0] : undefined;
      const m = head?.type === 'text' ? CONTAINER_OPEN.exec(head.value!) : null;
      const kind = m && CONTAINER_KIND[m[1].toLowerCase()];
      if (!head || !m || !kind) continue;
      const end = kids.findIndex((k, j) => {
        const last = j >= i && k.type === 'paragraph' ? k.children?.at(-1) : undefined;
        return (
          last?.type === 'text' &&
          CONTAINER_CLOSE.test(last.value!) &&
          !(last === head && head.value!.trim() === m[0].trim())
        );
      });
      if (end === -1) continue;
      head.value = head.value!.slice(m[0].length);
      const last = kids[end].children!.at(-1)!;
      last.value = last.value!.replace(CONTAINER_CLOSE, '');
      const body = kids
        .slice(i, end + 1)
        .map((k) => ({ ...k, children: k.children?.filter((c) => c.type !== 'text' || c.value) }))
        .filter((k) => k.type !== 'paragraph' || k.children?.length);
      kids.splice(i, end - i + 1, {
        type: 'blockquote',
        children: [
          { type: 'paragraph', children: [{ type: 'text', value: `[!${kind}]` }] },
          ...body,
        ],
      });
    }
  };
}

const INLINE_MARKS = /==(?=\S)([^=\n]*?\S)==|\+\+(?=\S)([^+\n]*?\S)\+\+|\^([^\s^]+)\^|~([^\s~]+)~/g;
const INLINE_TAGS = ['mark', 'ins', 'sup', 'sub'];

/** markdown-it's ==mark==, ++ins++, ^sup^ and ~sub~, only when the marks hug the text, so C++ or a == b stay as written. */
function remarkInlineMarks() {
  const visit = (node: MNode) => {
    if (!node.children) return;
    node.children = node.children.flatMap((c) => {
      if (c.type !== 'text') {
        visit(c);
        return [c];
      }
      const out: MNode[] = [];
      let at = 0;
      for (const m of c.value!.matchAll(INLINE_MARKS)) {
        const g = m.slice(1).findIndex((x) => x !== undefined);
        if (m.index > at) out.push({ type: 'text', value: c.value!.slice(at, m.index) });
        out.push({
          type: 'emphasis',
          data: { hName: INLINE_TAGS[g] },
          children: [{ type: 'text', value: m[g + 1] }],
        });
        at = m.index + m[0].length;
      }
      if (!out.length) return [c];
      if (at < c.value!.length) out.push({ type: 'text', value: c.value!.slice(at) });
      return out;
    });
  };
  return visit;
}

/** Sanitizing prefixes every id with user-content-; in-page links follow, as on GitHub. */
function rehypeLocalLinks() {
  const visit = (node: HNode) => {
    node.children?.forEach(visit);
    const href = node.tagName === 'a' ? node.properties?.href : undefined;
    if (typeof href === 'string' && href.startsWith('#') && !href.startsWith('#user-content-'))
      node.properties!.href = `#user-content-${href.slice(1)}`;
  };
  return visit;
}

/** A leading --- block is frontmatter only when it is a YAML map; otherwise it is a rule and text. */
function isYamlMap(text: string) {
  try {
    const v: unknown = parseYaml(text);
    return v !== null && typeof v === 'object' && !Array.isArray(v);
  } catch {
    return false;
  }
}

const cell = (v: unknown): string =>
  v == null
    ? ''
    : Array.isArray(v)
      ? v.map(cell).join(', ')
      : typeof v === 'object'
        ? JSON.stringify(v)
        : scalarText(v);

interface Props {
  children: string;
  components?: Components;
  /** Larger headings, for a full page document. */
  size?: 'lg';
  /** Keep single line breaks, for files a tool reads as written. */
  breaks?: boolean;
  className?: string;
}

// Raw HTML such as <details> renders, but sanitized: shown files can come from anyone's skills.
type PluggableList = NonNullable<Options['remarkPlugins']>;
const schema = { ...defaultSchema, tagNames: [...defaultSchema.tagNames!, 'mark'] };
const rehypePlugins = [
  rehypeRaw,
  [rehypeSanitize, schema],
  rehypeAlerts,
  rehypeLocalLinks,
] as PluggableList;
// A single ~ is left for ~sub~, as markdown-it reads it; ~~strike~~ still works.
const remarkPlugins = [
  [remarkGfm, { singleTilde: false }],
  remarkGemoji,
  remarkContainers,
  remarkInlineMarks,
] as PluggableList;
const remarkWithBreaks = [...remarkPlugins, remarkBreaks];
const remarkRehypeOptions = { clobberPrefix: '' };

/** A leading frontmatter block as table rows, and the markdown after it. */
function splitFrontmatter(md: string): { rows: string[][]; body: string } {
  if (!/^---\r?\n/.test(md)) return { rows: [], body: md };
  const parsed = parseSkillMarkdown(md);
  if (!parsed.hasFrontmatter || !isYamlMap(parsed.rawFrontmatter)) return { rows: [], body: md };
  const rows = Object.entries(parsed.frontmatter).flatMap(([k, v]) =>
    v && typeof v === 'object' && !Array.isArray(v)
      ? Object.entries(v).map(([sub, x]) => [`${k}.${sub}`, cell(x)])
      : [[k, cell(v)]],
  );
  return { rows, body: parsed.body };
}

/** Rendered markdown. A frontmatter block at the top is shown as a key/value table, like GitHub does. */
export default function MarkdownView({
  children,
  components,
  size,
  breaks,
  className = '',
}: Props) {
  const { rows, body } = useMemo(() => splitFrontmatter(children), [children]);

  return (
    <div className={`ss-prose ${size ?? ''} ${className}`}>
      {rows.length > 0 && (
        <div className="ss-tbl fm">
          <table>
            <tbody>
              {rows.map(([k, v]) => (
                <tr key={k}>
                  <th>{k}</th>
                  <td>{v}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <Markdown
        remarkPlugins={breaks ? remarkWithBreaks : remarkPlugins}
        rehypePlugins={rehypePlugins}
        remarkRehypeOptions={remarkRehypeOptions}
        components={{ ...base, ...components }}
      >
        {body}
      </Markdown>
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
