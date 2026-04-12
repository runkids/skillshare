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
import { api, type SkillStats } from '../api/client';
import { buildSkillDraftStats, buildSkillTokenBreakdown } from '../lib/skillMarkdown';
import SkillDetailPage from './SkillDetailPage';

vi.mock('../components/SkillMarkdownEditor', () => ({
  default: function MockSkillMarkdownEditor(props: {
    value: string;
    surface: 'rich' | 'raw';
    mode: 'edit' | 'split';
    isDirty: boolean;
    onChange: (value: string) => void;
    onSave: (value: string) => void;
    onDiscard: () => void;
    onSurfaceChange: (surface: 'rich' | 'raw') => void;
  }) {
    return (
      <section>
        <div>Mock editor mode: {props.mode}</div>
        <div>Mock editor surface: {props.surface}</div>
        <label htmlFor="skill-markdown-editor">Raw markdown editor</label>
        <textarea
          id="skill-markdown-editor"
          aria-label="Raw markdown editor"
          value={props.value}
          onChange={(event) => props.onChange(event.currentTarget.value)}
        />
        <button type="button" onClick={() => props.onSurfaceChange(props.surface === 'raw' ? 'rich' : 'raw')}>
          Toggle Surface
        </button>
        <button type="button" onClick={() => props.onSave(props.value)} disabled={!props.isDirty}>
          Save
        </button>
        <button type="button" onClick={props.onDiscard} disabled={!props.isDirty}>
          Discard
        </button>
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
      getSkill: vi.fn(),
      listSkills: vi.fn(),
      auditSkill: vi.fn(),
      diff: vi.fn(),
      deleteSkill: vi.fn(),
      deleteRepo: vi.fn(),
      update: vi.fn(),
      saveSkillFile: vi.fn(),
      openSkillFile: vi.fn(),
      enableSkill: vi.fn(),
      disableSkill: vi.fn(),
    },
  };
});

describe('SkillDetailPage', () => {
  const getSkill = vi.mocked(api.getSkill);
  const listSkills = vi.mocked(api.listSkills);
  const auditSkill = vi.mocked(api.auditSkill);
  const diff = vi.mocked(api.diff);
  const saveSkillFile = vi.mocked(api.saveSkillFile);
  const openSkillFile = vi.mocked(api.openSkillFile);

  const initialSkillMarkdown = [
    '---',
    'name: cli-e2e-test',
    'description: Initial description',
    'license: MIT',
    '---',
    '# Heading',
    '',
    'Initial body.',
  ].join('\n');

  beforeEach(() => {
    getSkill.mockReset();
    listSkills.mockReset();
    auditSkill.mockReset();
    diff.mockReset();
    saveSkillFile.mockReset();
    openSkillFile.mockReset();

    listSkills.mockResolvedValue({ skills: [] });
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
    saveSkillFile.mockResolvedValue({
      content: initialSkillMarkdown,
      filename: 'SKILL.md',
    });
    openSkillFile.mockResolvedValue({
      success: true,
      filename: 'SKILL.md',
    });
    getSkill.mockResolvedValue(buildSkillResponse(initialSkillMarkdown));
  });

  function buildSkillResponse(skillMdContent: string, stats?: SkillStats) {
    return {
      skill: {
        name: 'cli-e2e-test',
        flatName: 'cli-e2e-test',
        relPath: 'cli-e2e-test',
        sourcePath: '/tmp/skills/cli-e2e-test',
        isInRepo: false,
      },
      skillMdContent,
      files: ['SKILL.md'],
      stats: stats ?? buildSkillDraftStats(skillMdContent),
    };
  }

  function renderPage(initialEntry = '/skills/cli-e2e-test') {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });

    const routes: RouteObject[] = [
      {
        path: '/skills/:name',
        element: <SkillDetailPage />,
      },
      {
        path: '/skills',
        element: <div>Skills index</div>,
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

  it('renders server-provided skill stats instead of recomputing them in the browser', async () => {
    getSkill.mockResolvedValueOnce(buildSkillResponse(initialSkillMarkdown, {
      wordCount: 123,
      lineCount: 45,
      tokenCount: 67,
    }));

    renderPage();

    expect(await screen.findByRole('heading', { name: 'cli-e2e-test' })).toBeInTheDocument();
    expect(screen.getByText('123 words')).toBeInTheDocument();
    expect(screen.getByText('45 lines')).toBeInTheDocument();
    expect(screen.getByText('67 tokens')).toBeInTheDocument();
  });

  it('exposes Read, Edit, and Split modes on the page', async () => {
    await loadPage();

    expect(screen.getByRole('button', { name: 'Read' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Split' })).toBeInTheDocument();
  });

  it('updates the manifest preview immediately from the draft frontmatter', async () => {
    const user = userEvent.setup();
    await loadPage();

    await user.click(screen.getByRole('button', { name: 'Edit' }));

    const editor = await screen.findByRole('textbox', { name: 'Raw markdown editor' });
    fireEvent.change(editor, {
      target: { value: [
      '---',
      'name: Edited skill',
      'description: Edited description',
      'license: Apache-2.0',
      '---',
      '# Heading',
      '',
      'Initial body.',
    ].join('\n') },
    });

    expect(screen.getByText('Edited skill')).toBeInTheDocument();
    expect(screen.getByText('Edited description')).toBeInTheDocument();
    expect(screen.getAllByText('Apache-2.0').length).toBeGreaterThan(0);
  });

  it('starts the summary card directly at the manifest values without extra frontmatter intro copy', async () => {
    await loadPage();

    expect(screen.queryByText(/edit the yaml block at the top of/i)).not.toBeInTheDocument();
    expect(screen.getByText('Name')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /show frontmatter/i })).toBeInTheDocument();
  });

  it('lets the description render outside the frontmatter toggle row', async () => {
    await loadPage();

    const frontmatterToggle = screen.getByRole('button', { name: /show frontmatter/i });
    const description = screen.getByText('Initial description');

    expect(frontmatterToggle.parentElement).not.toContainElement(description);
  });

  it('shows the full frontmatter reference from the summary card', async () => {
    const user = userEvent.setup();
    await loadPage();

    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    expect(screen.getByRole('heading', { name: 'Reference' })).toBeInTheDocument();
    expect(screen.getByText(/all fields are optional/i)).toBeInTheDocument();
    expect(screen.getByText('argument-hint')).toBeInTheDocument();
    expect(screen.getByText('disable-model-invocation')).toBeInTheDocument();
    expect(screen.getByText('allowed-tools')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /claude docs/i })).not.toBeInTheDocument();
  });

  it('shows the token hover breakdown for loading the skill versus reading the preview', async () => {
    const user = userEvent.setup();
    await loadPage();

    const tokenStat = screen.getByText(`${buildSkillDraftStats(initialSkillMarkdown).tokenCount} tokens`);
    const tokenBreakdown = buildSkillTokenBreakdown(initialSkillMarkdown);

    expect(tokenStat).not.toHaveClass('cursor-help');

    await user.hover(tokenStat);

    expect(await screen.findByText(`Loading the skill: ${tokenBreakdown.loadTokens.toLocaleString()} tokens`)).toBeInTheDocument();
    expect(screen.getByText(`Reading the preview: ${tokenBreakdown.previewTokens.toLocaleString()} tokens`)).toBeInTheDocument();
  });

  it('renders substitution-shaped markdown tokens as dedicated pills in the skill view', async () => {
    getSkill.mockResolvedValueOnce(buildSkillResponse([
      '---',
      'name: cli-e2e-test',
      'description: Initial description',
      '---',
      '# Token examples',
      '',
      'Use $ARGUMENTS and ${CLAUDE_SKILL_DIR} in prose.',
      '',
      'Inline shorthand: `$1`.',
    ].join('\n')));

    const { container } = renderPage();
    expect(await screen.findByRole('heading', { name: 'cli-e2e-test' })).toBeInTheDocument();

    const tokens = Array.from(container.querySelectorAll('[data-substitution-token="true"]'))
      .map((element) => element.textContent);

    expect(tokens).toEqual(expect.arrayContaining([
      '$ARGUMENTS',
      '${CLAUDE_SKILL_DIR}',
      '$1',
    ]));
  });

  it('does not treat ordinary dollar amounts as substitution tokens', async () => {
    getSkill.mockResolvedValueOnce(buildSkillResponse([
      '---',
      'name: cli-e2e-test',
      'description: Initial description',
      '---',
      '# Token examples',
      '',
      'Budget guidance: $30 million.',
      '',
      'Actual substitution: $ARGUMENTS.',
    ].join('\n')));

    const { container } = renderPage();
    expect(await screen.findByRole('heading', { name: 'cli-e2e-test' })).toBeInTheDocument();

    const tokens = Array.from(container.querySelectorAll('[data-substitution-token="true"]'))
      .map((element) => element.textContent);

    expect(tokens).toContain('$ARGUMENTS');
    expect(tokens).not.toContain('$30');
  });

  it('opens sidebar files in the local editor', async () => {
    const user = userEvent.setup();
    await loadPage();

    await user.click(screen.getByRole('button', { name: /open file skILL\.md locally/i }));

    expect(openSkillFile).toHaveBeenCalledWith('cli-e2e-test', 'SKILL.md');
  });

  it('renders the local file button without an external-link icon', async () => {
    await loadPage();

    const openFileButton = screen.getByRole('button', { name: /open file skILL\.md locally/i });

    expect(openFileButton.querySelector('svg')).toBeNull();
  });

  it('keeps rendering the dirty draft when switching back to Read mode', async () => {
    const user = userEvent.setup();
    await loadPage();

    await user.click(screen.getByRole('button', { name: 'Edit' }));

    const draft = [
      '---',
      'name: Draft in read mode',
      'description: Draft description stays visible',
      'license: Apache-2.0',
      '---',
      '# Draft heading',
      '',
      'Draft body visible in read mode.',
    ].join('\n');
    const draftStats = buildSkillDraftStats(draft);

    const editor = await screen.findByRole('textbox', { name: 'Raw markdown editor' });
    fireEvent.change(editor, { target: { value: draft } });
    await user.click(screen.getByRole('button', { name: 'Read' }));

    expect(await screen.findByText('Draft in read mode')).toBeInTheDocument();
    expect(screen.getByText('Draft description stays visible')).toBeInTheDocument();
    expect(screen.getAllByText('Apache-2.0').length).toBeGreaterThan(0);
    expect(screen.getByRole('heading', { name: 'Draft heading' })).toBeInTheDocument();
    expect(screen.getByText('Draft body visible in read mode.')).toBeInTheDocument();
    expect(screen.getByText(`${draftStats.wordCount} words`)).toBeInTheDocument();
    expect(screen.getByText(`${draftStats.lineCount} lines`)).toBeInTheDocument();
    expect(screen.getByText(`${draftStats.tokenCount} tokens`)).toBeInTheDocument();
  });

  it('updates the content stats from the current draft while editing', async () => {
    const user = userEvent.setup();
    await loadPage();

    await user.click(screen.getByRole('button', { name: 'Edit' }));

    const draft = [
      '---',
      'name: cli-e2e-test',
      'description: Initial description',
      'license: MIT',
      '---',
      '# Heading',
      '',
      'Fresh draft body for updated stats.',
      '',
      'Another line.',
    ].join('\n');
    const draftStats = buildSkillDraftStats(draft);

    const editor = await screen.findByRole('textbox', { name: 'Raw markdown editor' });
    fireEvent.change(editor, { target: { value: draft } });

    expect(screen.getByText(`${draftStats.wordCount} words`)).toBeInTheDocument();
    expect(screen.getByText(`${draftStats.lineCount} lines`)).toBeInTheDocument();
    expect(screen.getByText(`${draftStats.tokenCount} tokens`)).toBeInTheDocument();
  });

  it('saves SKILL.md when the user presses Cmd/Ctrl+S', async () => {
    const user = userEvent.setup();
    const { queryClient } = renderPage();
    expect(await screen.findByRole('heading', { name: 'cli-e2e-test' })).toBeInTheDocument();

    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    await user.click(screen.getByRole('button', { name: 'Edit' }));

    const draft = `${initialSkillMarkdown}\nSaved by shortcut.`;
    const editor = await screen.findByRole('textbox', { name: 'Raw markdown editor' });
    fireEvent.change(editor, { target: { value: draft } });
    await user.keyboard('{Meta>}s{/Meta}');

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalledWith('cli-e2e-test', 'SKILL.md', draft);
    });
    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['skills', 'cli-e2e-test'] });
    });
  });

  it('uses the save response content as authoritative and ignores duplicate save attempts while saving', async () => {
    const user = userEvent.setup();
    let resolveSave: ((value: { content: string; filename: string }) => void) | undefined;
    saveSkillFile.mockImplementationOnce(() => new Promise<{ content: string; filename: string }>((resolve) => {
      resolveSave = resolve;
    }));

    renderPage();
    expect(await screen.findByRole('heading', { name: 'cli-e2e-test' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Edit' }));
    const pendingDraft = `${initialSkillMarkdown}\nPending save`;
    fireEvent.change(await screen.findByRole('textbox', { name: 'Raw markdown editor' }), {
      target: { value: pendingDraft },
    });

    await user.keyboard('{Meta>}s{/Meta}');
    await user.keyboard('{Meta>}s{/Meta}');

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalledTimes(1);
    });

    expect(resolveSave).toBeDefined();
    if (resolveSave) {
      resolveSave({
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
      });
    }

    await waitFor(() => {
      expect(screen.queryByText(/unsaved changes/i)).not.toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Read' }));

    await waitFor(() => {
      expect(screen.getByText('Saved canonical')).toBeInTheDocument();
    });
    expect(screen.getByText('Saved from API')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Canonical heading' })).toBeInTheDocument();
    expect(screen.getByText('Canonical body.')).toBeInTheDocument();
  });

  it('prompts before in-app navigation when there are unsaved changes', async () => {
    const user = userEvent.setup();
    const { router } = renderPage();
    expect(await screen.findByRole('heading', { name: 'cli-e2e-test' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Edit' }));
    fireEvent.change(await screen.findByRole('textbox', { name: 'Raw markdown editor' }), {
      target: { value: `${initialSkillMarkdown}\nUnsaved change` },
    });

    await router.navigate('/other');

    expect(await screen.findByRole('heading', { name: /unsaved changes/i })).toBeInTheDocument();
    expect(screen.getByText(/will be lost/i)).toBeInTheDocument();
    expect(screen.queryByText('Other page')).not.toBeInTheDocument();
  });
});
