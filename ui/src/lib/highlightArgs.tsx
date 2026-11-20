import type { ReactNode } from 'react';

/**
 * Highlight `$ARGUMENTS` tokens inside Markdown-rendered text nodes.
 * Wrap each occurrence in a styled <span class="arg-token">.
 *
 * Usage: pass as `children` transform in react-markdown component overrides:
 *   p: ({ children }) => <p>{highlightArgs(children)}</p>
 */
export function highlightArgs(children: ReactNode): ReactNode {
  if (typeof children === 'string') return highlightArgsInString(children);
  if (Array.isArray(children)) {
    return children.map((c, i) =>
      typeof c === 'string' ? <span key={i}>{highlightArgsInString(c)}</span> : c,
    );
  }
  return children;
}

function highlightArgsInString(s: string): ReactNode {
  const parts = s.split(/(\$ARGUMENTS\b)/g);
  if (parts.length === 1) return s;
  return parts.map((p, i) =>
    p === '$ARGUMENTS' ? (
      <span key={i} className="arg-token">
        $ARGUMENTS
      </span>
    ) : (
      p
    ),
  );
}
