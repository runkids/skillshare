import {
  Children,
  cloneElement,
  createElement,
  isValidElement,
  type AnchorHTMLAttributes,
  type ReactNode,
} from 'react';
import type { Components } from 'react-markdown';
import {
  getSubstitutionTokenMatches,
  isFullSubstitutionToken,
} from '../lib/substitutionTokens';

type LinkRendererArgs = {
  href?: string;
  children: ReactNode;
  props: AnchorHTMLAttributes<HTMLAnchorElement>;
};

type SkillMarkdownComponentOptions = {
  renderLink?: (args: LinkRendererArgs) => ReactNode;
};

type TextAwareTag = 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6' | 'p' | 'li' | 'blockquote' | 'th' | 'td';

function SubstitutionToken({ value, tokenKey }: { value: string; tokenKey?: string }) {
  return (
    <span
      key={tokenKey}
      className="ss-substitution-token"
      data-substitution-token="true"
      data-testid="skill-substitution-token"
    >
      {value}
    </span>
  );
}

function splitTextWithSubstitutions(text: string, keyPrefix: string): ReactNode {
  const matches = getSubstitutionTokenMatches(text);
  if (matches.length === 0) return text;

  const segments: ReactNode[] = [];
  let cursor = 0;

  matches.forEach((match, index) => {
    const token = match.value;
    const start = match.start;

    if (start > cursor) {
      segments.push(text.slice(cursor, start));
    }

    segments.push(
      <SubstitutionToken
        key={`${keyPrefix}-${start}-${index}`}
        tokenKey={`${keyPrefix}-${start}-${index}`}
        value={token}
      />,
    );
    cursor = start + token.length;
  });

  if (cursor < text.length) {
    segments.push(text.slice(cursor));
  }

  return segments;
}

function transformMarkdownChildren(children: ReactNode, keyPrefix = 'sub'): ReactNode {
  return Children.map(children, (child, index) => transformMarkdownChild(child, `${keyPrefix}-${index}`));
}

function transformMarkdownChild(child: ReactNode, keyPrefix: string): ReactNode {
  if (typeof child === 'string') {
    return splitTextWithSubstitutions(child, keyPrefix);
  }

  if (typeof child === 'number') {
    return splitTextWithSubstitutions(String(child), keyPrefix);
  }

  if (!isValidElement<{ children?: ReactNode }>(child)) {
    return child;
  }

  if (child.type === 'pre') {
    return child;
  }

  if (child.type === 'code') {
    const content = flattenText(child.props.children).trim();
    if (content && isFullSubstitutionToken(content)) {
      return <SubstitutionToken tokenKey={keyPrefix} value={content} />;
    }
    return child;
  }

  return cloneElement(child, undefined, transformMarkdownChildren(child.props.children, keyPrefix));
}

function flattenText(children: ReactNode): string {
  if (typeof children === 'string' || typeof children === 'number') {
    return String(children);
  }

  if (!children) return '';

  return Children.toArray(children)
    .map((child) => {
      if (typeof child === 'string' || typeof child === 'number') {
        return String(child);
      }
      if (isValidElement<{ children?: ReactNode }>(child)) {
        return flattenText(child.props.children);
      }
      return '';
    })
    .join('');
}

function createWrappedTag(tagName: TextAwareTag) {
  const WrappedTag = ({
    node,
    children,
    ...props
  }: {
    children?: ReactNode;
    node?: unknown;
    [key: string]: unknown;
  }) => {
    return createElement(
      tagName,
      props,
      transformMarkdownChildren(children, tagName),
    );
  };

  return WrappedTag as any;
}

export function createSkillMarkdownComponents(
  options: SkillMarkdownComponentOptions = {},
): Components {
  const { renderLink } = options;

  return {
    h1: createWrappedTag('h1'),
    h2: createWrappedTag('h2'),
    h3: createWrappedTag('h3'),
    h4: createWrappedTag('h4'),
    h5: createWrappedTag('h5'),
    h6: createWrappedTag('h6'),
    p: createWrappedTag('p'),
    li: createWrappedTag('li'),
    blockquote: createWrappedTag('blockquote'),
    th: createWrappedTag('th'),
    td: createWrappedTag('td'),
    a: ({ node, children, href, ...props }) => {
      const content = transformMarkdownChildren(children);
      if (renderLink) {
        return renderLink({
          href,
          children: content,
          props,
        });
      }

      return (
        <a href={href} {...props}>
          {content}
        </a>
      );
    },
    code: ({ node, inline, className, children, ...props }: {
      node?: unknown;
      inline?: boolean;
      className?: string;
      children?: ReactNode;
    }) => {
      if (inline) {
        const content = flattenText(children).trim();
        if (content && isFullSubstitutionToken(content)) {
          return <SubstitutionToken value={content} />;
        }

        return (
          <code className={className} {...props}>
            {transformMarkdownChildren(children)}
          </code>
        );
      }

      return (
        <code className={className} {...props}>
          {children}
        </code>
      );
    },
  };
}
