import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createMemoryRouter,
  RouterProvider,
  type RouteObject,
} from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import ResourceDetailPage from './ResourceDetailPage';

vi.mock('../components/SkillMarkdownEditor', () => ({
  default: function MockSkillMarkdownEditor(props: {
    value: string;
    surface: 'rich' | 'raw';
    mode: 'edit' | 'split';
    isDirty: boolean;
    showToolbar?: boolean;
    onChange: (value: string) => void;
    onSave: (value: string) => void;
    onDiscard: () => void;
    onSurfaceChange: (surface: 'rich' | 'raw') => void;
  }) {
    return (
      <section>
        <div>Mock editor mode: {props.mode}</div>
        <div>Mock editor surface: {props.surface}</div>
        <label htmlFor="resource-markdown-editor">Raw markdown editor</label>
        <textarea
          id="resource-markdown-editor"
          aria-label="Raw markdown editor"
          value={props.value}
          onChange={(event) => props.onChange(event.currentTarget.value)}
        />
      </section>
    );
  },
}));

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      getResource: vi.fn(),
      listSkills: vi.fn(),
      auditSkill: vi.fn(),
      diff: vi.fn(),
      deleteResource: vi.fn(),
      deleteRepo: vi.fn(),
      openSkillFile: vi.fn(),
      update: vi.fn(),
      saveSkillFile: vi.fn(),
      enableResource: vi.fn(),
      disableResource: vi.fn(),
    },
  };
});

describe('ResourceDetailPage', () => {
  const getResource = vi.mocked(api.getResource);
  const listSkills = vi.mocked(api.listSkills);
  const auditSkill = vi.mocked(api.auditSkill);
  const diff = vi.mocked(api.diff);
  const openSkillFile = vi.mocked(api.openSkillFile);
  const saveSkillFile = vi.mocked(api.saveSkillFile);
  const writeText = vi.fn();

  const initialSkillMarkdown = [
    '---',
    'name: cli-e2e-test',
    'description: Initial description',
    'license: MIT',
    'custom-note: hidden by default',
    '---',
    '# Heading',
    '',
    'Initial body.',
  ].join('\n');

  let currentSkillMarkdown = initialSkillMarkdown;

  beforeEach(() => {
    currentSkillMarkdown = initialSkillMarkdown;

    getResource.mockReset();
    listSkills.mockReset();
    auditSkill.mockReset();
    diff.mockReset();
    openSkillFile.mockReset();
    saveSkillFile.mockReset();
    writeText.mockReset();
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });

    getResource.mockImplementation(async () => buildResourceResponse(currentSkillMarkdown));
    listSkills.mockResolvedValue({ resources: [] });
    auditSkill.mockResolvedValue({
      result: {
        skillName: 'cli-e2e-test',
        findings: [],
        riskScore: 0,
        riskLabel: 'low',
        threshold: 'warn',
        isBlocked: false,
      },
      summary: {
        total: 0,
        passed: 0,
        warning: 0,
        failed: 0,
        critical: 0,
        high: 0,
        medium: 0,
        low: 0,
        info: 0,
        threshold: 'warn',
        riskScore: 0,
        riskLabel: 'low',
      },
    });
    diff.mockResolvedValue({
      diffs: [],
      ignored_count: 0,
      ignored_skills: [],
      ignore_root: '',
      ignore_repos: [],
    });
    openSkillFile.mockResolvedValue({
      success: true,
      filename: 'SKILL.md',
    });
    saveSkillFile.mockImplementation(async (_name, _filepath, content) => {
      currentSkillMarkdown = content;
      return {
        content,
        filename: 'SKILL.md',
      };
    });
  });

  function buildResourceResponse(skillMdContent: string) {
    return {
      resource: {
        kind: 'skill' as const,
        name: 'cli-e2e-test',
        flatName: 'cli-e2e-test',
        relPath: 'cli-e2e-test',
        sourcePath: '/tmp/skills/cli-e2e-test',
        isInRepo: false,
      },
      skillMdContent,
      files: ['SKILL.md'],
    };
  }

  function renderPage(initialEntry = '/resources/cli-e2e-test') {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });

    const routes: RouteObject[] = [
      {
        path: '/resources/:name',
        element: <ResourceDetailPage />,
      },
      {
        path: '/resources',
        element: <div>Resources index</div>,
      },
      {
        path: '/other',
        element: <div>Other page</div>,
      },
    ];

    const router = createMemoryRouter(routes, {
      initialEntries: [initialEntry],
    });

    const view = render(
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <RouterProvider router={router} />
        </ToastProvider>
      </QueryClientProvider>,
    );

    return { ...view, queryClient, router };
  }

  async function loadPage() {
    renderPage();
    expect(await screen.findByRole('heading', { name: 'cli-e2e-test' })).toBeInTheDocument();
  }

  it('shows immediately editable manifest fields, an always-present editor, and no primary edit controls', async () => {
    await loadPage();

    expect(screen.getByRole('textbox', { name: 'Skill name' })).toHaveValue('cli-e2e-test');
    expect(screen.getByRole('textbox', { name: 'Skill description' })).toHaveValue('Initial description');
    expect(screen.getByRole('textbox', { name: 'Skill description' })).toHaveAttribute('rows', '1');
    expect(screen.getByRole('textbox', { name: 'Raw markdown editor' })).toHaveValue(initialSkillMarkdown);
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Discard' })).not.toBeInTheDocument();
  });

  it('autosaves manifest changes when focus leaves the field', async () => {
    const user = userEvent.setup();
    await loadPage();

    const nameInput = screen.getByRole('textbox', { name: 'Skill name' });
    await user.clear(nameInput);
    await user.type(nameInput, 'Edited skill');
    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalled();
    });

    expect(await screen.findByText(/^Saved$/)).toBeInTheDocument();
    const lastSave = saveSkillFile.mock.calls.at(-1);
    expect(lastSave?.[2]).toContain('name: Edited skill');
  });

  it('autosaves body changes when focus leaves the editor', async () => {
    const user = userEvent.setup();
    await loadPage();

    await user.type(screen.getByRole('textbox', { name: 'Raw markdown editor' }), '\nBody draft');
    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalled();
    });

    expect(await screen.findByText(/^Saved$/)).toBeInTheDocument();
    expect(saveSkillFile.mock.calls.at(-1)?.[2]).toContain('Body draft');
  });

  it('hides the frontmatter workspace by default and reveals YAML Frontmatter on demand', async () => {
    const user = userEvent.setup();
    currentSkillMarkdown = [
      '---',
      'name: cli-e2e-test',
      'description: Initial description',
      'argument-hint: [tag-version]',
      'allowed-tools:',
      '  - Read',
      '  - Edit',
      'custom-note: hidden by default',
      '---',
      '# Heading',
      '',
      'Initial body.',
    ].join('\n');

    await loadPage();

    expect(screen.queryByRole('heading', { name: 'YAML Frontmatter' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    const yamlFrontmatterHeading = screen.getByRole('heading', { name: 'YAML Frontmatter' });
    expect(yamlFrontmatterHeading).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Reference' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /add name/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /add description/i })).not.toBeInTheDocument();

    const statsBar = document.querySelector('.ss-detail-stats');
    expect(statsBar).not.toBeNull();
    expect(
      (statsBar as Node).compareDocumentPosition(yamlFrontmatterHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    const hideButton = screen.getByRole('button', { name: /hide frontmatter/i });
    expect(
      hideButton.compareDocumentPosition(yamlFrontmatterHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it('edits configured frontmatter fields from the expanded workspace and autosaves on blur', async () => {
    const user = userEvent.setup();
    currentSkillMarkdown = [
      '---',
      'name: cli-e2e-test',
      'description: Initial description',
      'argument-hint: "[tag-version]"',
      'metadata:',
      '  targets: [claude, universal]',
      '---',
      '# Heading',
      '',
      'Initial body.',
    ].join('\n');

    await loadPage();
    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    const argumentHintInput = await screen.findByRole('textbox', { name: 'argument-hint frontmatter value' });
    fireEvent.change(argumentHintInput, { target: { value: '[release-version]' } });
    await user.click(screen.getByRole('textbox', { name: 'Skill description' }));

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalled();
    });

    const editor = screen.getByRole('textbox', { name: 'Raw markdown editor' }) as HTMLTextAreaElement;
    expect(editor.value).toContain('argument-hint: "[release-version]"');
    expect(saveSkillFile.mock.calls.at(-1)?.[2]).toContain('argument-hint: "[release-version]"');
  });

  it('renaming a built-in frontmatter field frees the built-in reference slot', async () => {
    const user = userEvent.setup();
    currentSkillMarkdown = [
      '---',
      'name: cli-e2e-test',
      'description: Initial description',
      'argument-hint: "[tag-version]"',
      '---',
      '# Heading',
      '',
      'Initial body.',
    ].join('\n');

    await loadPage();
    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    const fieldKey = await screen.findByRole('textbox', { name: 'argument-hint frontmatter key' });
    await user.clear(fieldKey);
    await user.type(fieldKey, 'custom-argument');

    expect(screen.getByRole('button', { name: /add argument-hint/i })).toBeInTheDocument();
    expect(screen.getByDisplayValue('custom-argument')).toBeInTheDocument();
  });

  it('adds, removes, and saves built-in and custom frontmatter fields from the expanded workspace', async () => {
    const user = userEvent.setup();
    await loadPage();

    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));
    await user.click(screen.getByRole('button', { name: /remove custom-note/i }));
    await user.click(screen.getByRole('button', { name: /add argument-hint/i }));
    fireEvent.change(screen.getByRole('textbox', { name: 'argument-hint frontmatter value' }), {
      target: { value: '[release-version]' },
    });
    await user.click(screen.getByRole('button', { name: /add custom frontmatter/i }));

    const customKeyInput = screen.getAllByRole('textbox').find((element) => element.getAttribute('aria-label') === 'custom frontmatter key');
    expect(customKeyInput).toBeDefined();
    await user.type(customKeyInput as HTMLInputElement, 'metadata-note');
    await user.type(screen.getByRole('textbox', { name: 'metadata-note frontmatter value' }), 'hello');
    await user.click(screen.getByRole('textbox', { name: 'Skill description' }));

    await waitFor(() => {
      expect(
        saveSkillFile.mock.calls.some((call) => call[2].includes('metadata-note: hello')),
      ).toBe(true);
    });

    const saveCall = [...saveSkillFile.mock.calls]
      .reverse()
      .find((call) => call[2].includes('metadata-note: hello'));
    expect(saveCall?.[2]).toContain('argument-hint: "[release-version]"');
    expect(saveCall?.[2]).toContain('metadata-note: hello');
    expect(saveCall?.[2]).not.toContain('custom-note: hidden by default');
  });

  it('updates token, word, and line counts live while manifest and frontmatter fields change', async () => {
    const user = userEvent.setup();
    await loadPage();

    const statsBar = document.querySelector('.ss-detail-stats');
    expect(statsBar).not.toBeNull();
    const extractCounts = (text: string) => {
      const match = text.match(/~([\d,]+) tokens[\s\S]*?(\d+) words[\s\S]*?(\d+) lines/i);
      expect(match).not.toBeNull();
      return {
        tokens: match?.[1],
        words: match?.[2],
        lines: match?.[3],
      };
    };
    const before = extractCounts(statsBar?.textContent ?? '');

    await user.type(screen.getByRole('textbox', { name: 'Skill description' }), ' plus more words');
    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));
    fireEvent.change(screen.getByRole('textbox', { name: 'custom-note frontmatter value' }), {
      target: { value: 'now even longer than before' },
    });

    const after = extractCounts(document.querySelector('.ss-detail-stats')?.textContent ?? '');
    expect(after.tokens).not.toBe(before.tokens);
    expect(after.words).not.toBe(before.words);
  });

  it('shows path-specific copy feedback for the resource path row', async () => {
    writeText.mockResolvedValue(undefined);

    await loadPage();

    fireEvent.click(screen.getByRole('button', { name: /copy to clipboard/i }));

    expect(writeText).toHaveBeenCalledWith('/tmp/skills/cli-e2e-test');
    expect(await screen.findByText('Path copied!')).toBeInTheDocument();
  });

  it('uses the save response content as authoritative after autosave even if the refetch stays stale', async () => {
    const user = userEvent.setup();
    saveSkillFile.mockImplementationOnce(async () => ({
      content: [
        '---',
        'name: Saved canonical',
        'description: Saved from API',
        'license: MIT',
        '---',
        '# Canonical heading',
        '',
        'Canonical body.',
      ].join('\n'),
      filename: 'SKILL.md',
    }));

    renderPage();
    expect(await screen.findByRole('heading', { name: 'cli-e2e-test' })).toBeInTheDocument();

    await user.clear(screen.getByRole('textbox', { name: 'Skill name' }));
    await user.type(screen.getByRole('textbox', { name: 'Skill name' }), 'Pending save');
    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalled();
    });

    expect(await screen.findByDisplayValue('Saved canonical')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Saved from API')).toBeInTheDocument();
    const editor = screen.getByRole('textbox', { name: 'Raw markdown editor' }) as HTMLTextAreaElement;
    expect(editor.value).toContain('# Canonical heading');
    expect(editor.value).toContain('Canonical body.');
  });
});
