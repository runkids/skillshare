import { useState } from 'react';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { composeSkillMarkdown, parseSkillMarkdown, type Frontmatter } from '../../lib/frontmatter';
import FrontmatterEditor from './FrontmatterEditor';

const records = {
  sources: [{ resource: 'docs/a.md', author: 'team:finance', 'x-acme-review': { status: 'ok' } }],
  verified: [{ by: 'other:unknown', at: '2026-06-25T09:00:00Z' }],
};
const extension = ['label', ['nested', { 'x-acme-review': 'keep' }]];
const initial: Frontmatter = {
  description: 'Original',
  metadata: { ...records, targets: ['claude', 'codex'], mixed: extension },
  'x-records': records.verified,
  'x-extension': extension,
  'x-tags': ['one', 'two'],
};

function Harness({
  saved,
  value = initial,
}: {
  saved: (content: string) => void;
  value?: Frontmatter;
}) {
  const [frontmatter, setFrontmatter] = useState(value);
  return (
    <I18nProvider>
      <FrontmatterEditor
        frontmatter={frontmatter}
        onChange={setFrontmatter}
        yaml={false}
        onYaml={vi.fn()}
      />
      <button onClick={() => saved(composeSkillMarkdown(frontmatter, '# Body'))}>Save</button>
    </I18nProvider>
  );
}

function row(name: string) {
  return screen.getByText(name, { selector: 'span' }).parentElement as HTMLElement;
}

describe('FrontmatterEditor structured lists', () => {
  it('protects records through an unrelated edit and reopen while allowing explicit removal and scalar list edits', async () => {
    const user = userEvent.setup();
    const saved = vi.fn();
    const view = render(<Harness saved={saved} />);

    for (const name of [
      'metadata.sources',
      'metadata.verified',
      'metadata.mixed',
      'x-records',
      'x-extension',
    ]) {
      expect(within(row(name)).queryByRole('textbox')).not.toBeInTheDocument();
    }
    expect(row('metadata.sources')).toHaveTextContent('team:finance');
    expect(row('metadata.sources')).toHaveTextContent('x-acme-review');
    expect(row('metadata.verified')).toHaveTextContent('other:unknown');
    expect(row('x-extension')).toHaveTextContent('x-acme-review');

    await user.clear(screen.getByRole('textbox', { name: 'description' }));
    await user.type(screen.getByRole('textbox', { name: 'description' }), 'Updated');
    await user.clear(screen.getByRole('textbox', { name: 'targets' }));
    await user.type(screen.getByRole('textbox', { name: 'targets' }), 'cursor, gemini');
    await user.clear(screen.getByRole('textbox', { name: 'x-tags' }));
    await user.type(screen.getByRole('textbox', { name: 'x-tags' }), 'three, four');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    const reopened = parseSkillMarkdown(saved.mock.lastCall![0]).frontmatter;
    expect(reopened).toEqual({
      ...initial,
      description: 'Updated',
      metadata: { ...records, mixed: extension, targets: ['cursor', 'gemini'] },
      'x-tags': ['three', 'four'],
    });

    view.unmount();
    render(<Harness saved={saved} value={reopened} />);
    expect(row('metadata.sources')).toHaveTextContent('team:finance');
    expect(row('metadata.verified')).toHaveTextContent('other:unknown');
    expect(row('x-extension')).toHaveTextContent('x-acme-review');
    await user.click(within(row('metadata.sources')).getByRole('button'));
    await user.click(within(row('x-extension')).getByRole('button'));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    const removed = parseSkillMarkdown(saved.mock.lastCall![0]).frontmatter;
    expect(removed.metadata).toEqual({
      verified: records.verified,
      mixed: extension,
      targets: ['cursor', 'gemini'],
    });
    expect(removed).not.toHaveProperty('x-extension');
    expect(removed['x-records']).toEqual(records.verified);
  });
});
