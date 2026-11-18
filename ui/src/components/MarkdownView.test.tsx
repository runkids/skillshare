import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import MarkdownView from './MarkdownView';

const view = (md: string, breaks?: boolean) =>
  render(<MarkdownView breaks={breaks}>{md}</MarkdownView>).container;

describe('MarkdownView', () => {
  it('renders HTML such as <details> but drops scripts and event handlers', () => {
    const html = view(
      '<details><summary>More</summary>\n\nhidden\n\n</details>\n\n<script>alert(1)</script>\n<img src="x" onerror="alert(1)">',
    ).innerHTML;
    expect([
      html.includes('<details>'),
      html.includes('<script'),
      html.includes('onerror'),
    ]).toEqual([true, false, false]);
  });
});
