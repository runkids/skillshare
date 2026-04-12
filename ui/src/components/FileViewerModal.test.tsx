import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import FileViewerModal from './FileViewerModal';
import { api } from '../api/client';
import { ToastProvider } from './Toast';

let shouldThrowEditor = false;

vi.mock('@uiw/react-codemirror', () => ({
  default: function MockCodeMirror(props: { value: string; readOnly?: boolean; editable?: boolean }) {
    return (
      <textarea
        aria-label="Code viewer"
        readOnly={props.readOnly}
        data-editable={String(props.editable)}
        value={props.value}
        onChange={() => {}}
      />
    );
  },
}));

vi.mock('@codemirror/lang-json', () => ({
  json: () => ({ name: 'json' }),
}));

vi.mock('@codemirror/lang-yaml', () => ({
  yaml: () => ({ name: 'yaml' }),
}));

vi.mock('@codemirror/lang-python', () => ({
  python: () => ({ name: 'python' }),
}));

vi.mock('@codemirror/lang-javascript', () => ({
  javascript: () => ({ name: 'javascript' }),
}));

vi.mock('@codemirror/view', () => ({
  EditorView: {
    lineWrapping: { name: 'lineWrapping' },
    editable: {
      of: () => ({ name: 'editable' }),
    },
  },
}));

vi.mock('../lib/codemirror-theme', () => ({
  handTheme: [],
}));

vi.mock('./SkillMarkdownEditor', () => ({
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
    if (shouldThrowEditor) {
      throw new Error('Mock lazy editor failure');
    }

    return (
      <section>
        <div>Mock editor mode: {props.mode}</div>
        <div>Mock editor surface: {props.surface}</div>
        <label htmlFor="file-viewer-markdown-editor">Raw markdown editor</label>
        <textarea
          id="file-viewer-markdown-editor"
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
      getSkillFile: vi.fn(),
      saveSkillFile: vi.fn(),
      openSkillFile: vi.fn(),
    },
  };
});

describe('FileViewerModal', () => {
  const getSkillFile = vi.mocked(api.getSkillFile);
  const saveSkillFile = vi.mocked(api.saveSkillFile);
  const openSkillFile = vi.mocked(api.openSkillFile);
  const writeText = vi.fn();

  beforeEach(() => {
    shouldThrowEditor = false;
    getSkillFile.mockReset();
    saveSkillFile.mockReset();
    openSkillFile.mockReset();
    writeText.mockReset();
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
  });

  function renderModal(
    filepath = 'docs/guide.md',
    onClose = vi.fn(),
    resourceKind: 'skill' | 'agent' = 'skill',
  ) {
    render(
      <ToastProvider>
        <FileViewerModal
          skillName="sample-skill"
          filepath={filepath}
          sourcePath="/tmp/sample-skill"
          resourceKind={resourceKind}
          onClose={onClose}
        />
      </ToastProvider>,
    );

    return { onClose };
  }

  it('shows the markdown editor immediately with no Edit, Open, Save, or Discard actions', async () => {
    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal();

    expect(await screen.findByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Open' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Discard' })).not.toBeInTheDocument();
    expect(screen.getByText(/tokens/i)).toBeInTheDocument();
    expect(screen.getByText(/words/i)).toBeInTheDocument();
    expect(screen.getByText(/lines/i)).toBeInTheDocument();
    expect(screen.getByText(/1 file/i)).toBeInTheDocument();
  });

  it('shows the structured frontmatter workspace in markdown modals beneath the title-row toggle', async () => {
    const user = userEvent.setup();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '---\ntitle: Guide\nowner: docs\n---\n# Guide\n\nBody',
    });

    renderModal();

    expect(await screen.findByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /show frontmatter/i })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    const workspace = screen.getByRole('heading', { name: 'YAML Frontmatter' });
    expect(workspace).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Reference' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add name/i })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'title frontmatter key' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'owner frontmatter key' })).toBeInTheDocument();
    const toggleButton = screen.getByRole('button', { name: /hide frontmatter/i });
    expect(
      toggleButton.compareDocumentPosition(workspace) & Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    expect(toggleButton).toBeInTheDocument();
  });

  it('shows the frontmatter workspace for markdown files without YAML and lets users add new fields', async () => {
    const user = userEvent.setup();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal();

    expect(await screen.findByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /show frontmatter/i })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    expect(screen.getByRole('heading', { name: 'YAML Frontmatter' })).toBeInTheDocument();
    expect(screen.getByText(/no frontmatter fields are set yet/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add custom frontmatter/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add name/i })).toBeInTheDocument();
  });

  it('shows the agent frontmatter reference in markdown modals for agent resources', async () => {
    const user = userEvent.setup();

    getSkillFile.mockResolvedValueOnce({
      filename: 'agents/reviewer.md',
      contentType: 'text/markdown',
      content: '# Reviewer\n\nBody',
    });

    renderModal('agents/reviewer.md', vi.fn(), 'agent');

    expect(await screen.findByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));

    expect(screen.getByRole('button', { name: /add permissionmode/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /add allowed-tools/i })).not.toBeInTheDocument();
  });

  it('adds modal frontmatter fields through the workspace and autosaves them on blur', async () => {
    const user = userEvent.setup();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });
    saveSkillFile.mockImplementation(async (_skillName, _filepath, content) => ({
      filename: 'docs/guide.md',
      content,
    }));

    renderModal();

    expect(await screen.findByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /show frontmatter/i }));
    await user.click(screen.getByRole('button', { name: /add name/i }));

    fireEvent.change(screen.getByRole('textbox', { name: 'name frontmatter value' }), {
      target: { value: 'Guide doc' },
    });
    await user.click(screen.getByRole('button', { name: 'Copy file path' }));

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalled();
    });

    expect(saveSkillFile.mock.calls.at(-1)?.[2]).toContain('name: Guide doc');
    expect(
      String((screen.getByRole('textbox', { name: 'Raw markdown editor' }) as HTMLTextAreaElement).value),
    ).toContain('name: Guide doc');
  });

  it('autosaves markdown changes on blur and uses the API response as authoritative content', async () => {
    const user = userEvent.setup();
    let resolveSave: ((value: { filename: string; content: string }) => void) | undefined;

    writeText.mockResolvedValue(undefined);
    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });
    saveSkillFile.mockImplementationOnce(() => new Promise((resolve) => {
      resolveSave = resolve;
    }));

    renderModal();

    await user.type(await screen.findByRole('textbox', { name: 'Raw markdown editor' }), '\nDraft change');
    await user.click(screen.getByRole('button', { name: 'Copy file path' }));

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalledTimes(1);
    });
    expect(saveSkillFile).toHaveBeenCalledWith('sample-skill', 'docs/guide.md', '# Guide\n\nBody\nDraft change');

    resolveSave?.({
      filename: 'docs/guide.md',
      content: '# Canonical\n\nSaved by API',
    });

    await waitFor(() => {
      expect(screen.getByRole('textbox', { name: 'Raw markdown editor' })).toHaveValue('# Canonical\n\nSaved by API');
    });
    expect(await screen.findByText(/^Saved$/)).toBeInTheDocument();
  });

  it('clicking Close while dirty saves first and then closes after the save finishes', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    let resolveSave: ((value: { filename: string; content: string }) => void) | undefined;

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });
    saveSkillFile.mockImplementationOnce(() => new Promise((resolve) => {
      resolveSave = resolve;
    }));

    renderModal('docs/guide.md', onClose);

    await user.type(await screen.findByRole('textbox', { name: 'Raw markdown editor' }), '\nUnsaved');
    await user.click(screen.getByRole('button', { name: 'Close' }));

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalledTimes(1);
      expect(screen.getByRole('button', { name: 'Close' })).toBeDisabled();
    });
    expect(onClose).not.toHaveBeenCalled();

    resolveSave?.({
      filename: 'docs/guide.md',
      content: '# Guide\n\nSaved cleanly',
    });

    await waitFor(() => {
      expect(screen.getByRole('textbox', { name: 'Raw markdown editor' })).toHaveValue('# Guide\n\nSaved cleanly');
    });
    await waitFor(() => {
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  it('prompts for discard if closing a dirty markdown modal triggers a save failure', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });
    saveSkillFile.mockRejectedValueOnce(new Error('save broke'));

    renderModal('docs/guide.md', onClose);

    await user.type(await screen.findByRole('textbox', { name: 'Raw markdown editor' }), '\nUnsaved');
    await user.click(screen.getByRole('button', { name: 'Close' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('save broke');
    expect(await screen.findByRole('heading', { name: /discard changes/i })).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('shows path-specific copy feedback for the file path action', async () => {
    writeText.mockResolvedValue(undefined);
    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal();

    fireEvent.click(await screen.findByRole('button', { name: 'Copy file path' }));

    expect(writeText).toHaveBeenCalledWith('/tmp/sample-skill/docs/guide.md');
    expect(await screen.findByText('Path copied!')).toBeInTheDocument();
  });

  it('shows Open for non-markdown files and calls openSkillFile', async () => {
    const user = userEvent.setup();

    getSkillFile.mockResolvedValueOnce({
      filename: 'schema.json',
      contentType: 'application/json',
      content: '{\n  "ok": true\n}',
    });
    openSkillFile.mockResolvedValueOnce({
      success: true,
      filename: 'schema.json',
    });

    renderModal('schema.json');

    expect(await screen.findByRole('textbox', { name: 'Code viewer' })).toHaveValue('{\n  "ok": true\n}');
    expect(screen.getByRole('button', { name: 'Open' })).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Raw markdown editor' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Open' }));
    expect(openSkillFile).toHaveBeenCalledWith('sample-skill', 'schema.json');
  });

  it('falls back to a local raw editor if the lazy markdown editor fails', async () => {
    shouldThrowEditor = true;

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal();

    expect(await screen.findByRole('alert')).toHaveTextContent('Mock lazy editor failure');
    expect(screen.getByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
  });
});
