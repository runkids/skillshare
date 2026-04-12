import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import SkillMarkdownEditor, {
  skillMarkdownCodeBlockLanguages,
  type SkillMarkdownEditorMode,
  type SkillMarkdownEditorSurface,
} from './SkillMarkdownEditor';

const mdxEditorState = vi.hoisted(() => ({
  shouldThrow: false,
  emitOnErrorOnMount: false,
  emitOnErrorForUnsafeFenceMarkdown: false,
  forceInitialNormalizeCallback: false,
  forceNormalizeFlagOnInput: false,
  callOnChangeDuringRender: false,
}));

function hasUnsafeNestedFence(markdown: string) {
  return /```[^\n]*\n[\s\S]*^\s{0,3}```(?:[^\n]*)$/m.test(markdown) && markdown.includes('```markdown');
}

vi.mock('@mdxeditor/editor', async () => {
  const React = await vi.importActual<typeof import('react')>('react');

  const MockMDXEditor = React.forwardRef<
    {
      setMarkdown: (value: string) => void;
      insertMarkdown: (value: string) => void;
      focus: () => void;
      getContentEditableHTML: () => string;
      getSelectionMarkdown: () => string;
    },
    {
      markdown: string;
      trim?: boolean;
      onChange?: (next: string, initialMarkdownNormalize: boolean) => void;
      onError?: (payload: { error: string; source: string }) => void;
    }
  >(function MockMDXEditor(props, ref) {
    const [markdown, setMarkdown] = React.useState(() => {
      return props.trim === false ? props.markdown : props.markdown.trim();
    });
    const [pendingRenderChange, setPendingRenderChange] = React.useState<string | null>(null);
    const renderChangeDeliveredRef = React.useRef(false);

    if (mdxEditorState.callOnChangeDuringRender && pendingRenderChange !== null && !renderChangeDeliveredRef.current) {
      renderChangeDeliveredRef.current = true;
      props.onChange?.(pendingRenderChange, mdxEditorState.forceNormalizeFlagOnInput);
    }

    React.useEffect(() => {
      if (mdxEditorState.emitOnErrorOnMount) {
        props.onError?.({
          error: 'Parsing of the following markdown structure failed: {"type":"code","name":"N/A"}',
          source: props.markdown,
        });
      }
      if (mdxEditorState.emitOnErrorForUnsafeFenceMarkdown && hasUnsafeNestedFence(props.markdown)) {
        props.onError?.({
          error: 'Parsing of the following markdown structure failed: {"type":"code","name":"N/A"}',
          source: props.markdown,
        });
      }
    }, []);

    React.useEffect(() => {
      const normalized = props.trim === false ? props.markdown : props.markdown.trim();
      if (mdxEditorState.forceInitialNormalizeCallback) {
        props.onChange?.(`${props.markdown}::normalized`, true);
        return;
      }
      if (props.trim !== false && normalized !== props.markdown) {
        props.onChange?.(normalized, true);
      }
    }, []);

    React.useEffect(() => {
      if (!mdxEditorState.callOnChangeDuringRender || pendingRenderChange === null || !renderChangeDeliveredRef.current) {
        return;
      }

      renderChangeDeliveredRef.current = false;
      setPendingRenderChange(null);
    }, [pendingRenderChange]);

    React.useImperativeHandle(ref, () => ({
      setMarkdown(value: string) {
        if (value.trim() === markdown.trim()) {
          return;
        }
        setMarkdown(value);
      },
      insertMarkdown() {},
      focus() {},
      getContentEditableHTML() {
        return markdown;
      },
      getSelectionMarkdown() {
        return markdown;
      },
    }), [markdown]);

    if (mdxEditorState.shouldThrow) {
      throw new Error('rich editor failed to initialize');
    }

    return (
      <div data-testid="mock-mdx-editor">
        <label htmlFor="mock-mdx-editor-input">Rich markdown editor</label>
        <textarea
          id="mock-mdx-editor-input"
          aria-label="Rich markdown editor"
          value={markdown}
          onChange={(event) => {
            const next = event.currentTarget.value;
            setMarkdown(next);
            if (mdxEditorState.callOnChangeDuringRender) {
              setPendingRenderChange(next);
              return;
            }
            props.onChange?.(next, mdxEditorState.forceNormalizeFlagOnInput);
          }}
        />
      </div>
    );
  });

  const plugin = (name: string) => () => ({ name });

  return {
    MDXEditor: MockMDXEditor,
    headingsPlugin: plugin('headings'),
    listsPlugin: plugin('lists'),
    quotePlugin: plugin('quote'),
    thematicBreakPlugin: plugin('thematic-break'),
    markdownShortcutPlugin: plugin('markdown-shortcut'),
    frontmatterPlugin: plugin('frontmatter'),
    tablePlugin: plugin('table'),
    linkPlugin: plugin('link'),
    codeBlockPlugin: plugin('code-block'),
    codeMirrorPlugin: plugin('code-mirror'),
    realmPlugin: (config: unknown) => (params?: unknown) => ({ name: 'realm', config, params }),
    addComposerChild$: Symbol('addComposerChild'),
    addLexicalNode$: Symbol('addLexicalNode'),
    rootEditor$: Symbol('rootEditor'),
  };
});

type HarnessProps = {
  initialValue?: string;
  initialSurface?: SkillMarkdownEditorSurface;
  mode?: SkillMarkdownEditorMode;
  showToolbar?: boolean;
  onSave?: (value: string) => void;
  onDiscard?: () => void;
  onSurfaceChange?: (surface: SkillMarkdownEditorSurface) => void;
  onChangeSpy?: (value: string) => void;
};

function ControlledHarness({
  initialValue = '---\nname: sample\n---\n# Heading\n',
  initialSurface = 'rich',
  mode = 'edit',
  showToolbar = true,
  onSave = vi.fn(),
  onDiscard = vi.fn(),
  onSurfaceChange = vi.fn(),
  onChangeSpy,
}: HarnessProps) {
  const [value, setValue] = useState(initialValue);
  const [savedValue, setSavedValue] = useState(initialValue);
  const [surface, setSurface] = useState<SkillMarkdownEditorSurface>(initialSurface);

  return (
    <SkillMarkdownEditor
      value={value}
      mode={mode}
      surface={surface}
      isDirty={value !== savedValue}
      showToolbar={showToolbar}
      onChange={(next) => {
        onChangeSpy?.(next);
        setValue(next);
      }}
      onSave={(next) => {
        onSave(next);
        setSavedValue(next);
      }}
      onDiscard={() => {
        onDiscard();
      }}
      onSurfaceChange={(nextSurface) => {
        onSurfaceChange(nextSurface);
        setSurface(nextSurface);
      }}
    />
  );
}

function ExternalResetHarness() {
  const [value, setValue] = useState('Hello');
  const [surface, setSurface] = useState<SkillMarkdownEditorSurface>('rich');

  return (
    <div>
      <button type="button" onClick={() => setValue('Hello\n')}>
        Apply trailing newline
      </button>
      <button type="button" onClick={() => setSurface('raw')}>
        Raw surface
      </button>
      <SkillMarkdownEditor
        value={value}
        mode="edit"
        surface={surface}
        isDirty={false}
        onChange={setValue}
        onSave={() => {}}
        onDiscard={() => {}}
        onSurfaceChange={setSurface}
      />
    </div>
  );
}

describe('SkillMarkdownEditor', () => {
  beforeEach(() => {
    mdxEditorState.shouldThrow = false;
    mdxEditorState.emitOnErrorOnMount = false;
    mdxEditorState.emitOnErrorForUnsafeFenceMarkdown = false;
    mdxEditorState.forceInitialNormalizeCallback = false;
    mdxEditorState.forceNormalizeFlagOnInput = false;
    mdxEditorState.callOnChangeDuringRender = false;
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the controlled rich editor surface when the document loads', () => {
    render(<ControlledHarness />);

    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Raw markdown editor' })).not.toBeInTheDocument();
  });

  it('does not dirty parent state on mount-time normalization callbacks', () => {
    const onChangeSpy = vi.fn();

    render(<ControlledHarness initialValue={'# Heading\n'} onChangeSpy={onChangeSpy} />);

    expect(onChangeSpy).not.toHaveBeenCalled();
    expect(screen.queryByText(/unsaved changes/i)).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toHaveValue('# Heading\n');
  });

  it('ignores normalization-only rich editor callbacks even if they fire', () => {
    mdxEditorState.forceInitialNormalizeCallback = true;
    const onChangeSpy = vi.fn();

    render(<ControlledHarness onChangeSpy={onChangeSpy} />);

    expect(onChangeSpy).not.toHaveBeenCalled();
    expect(screen.queryByText(/unsaved changes/i)).not.toBeInTheDocument();
  });

  it('includes tsx-rich code block support for skill markdown documents', () => {
    expect(skillMarkdownCodeBlockLanguages).toMatchObject({
      md: 'Markdown',
      markdown: 'Markdown',
      ts: 'TypeScript',
      tsx: 'TypeScript (React)',
      jsx: 'JavaScript (React)',
      css: 'CSS',
    });
  });

  it('requests raw mode from the parent when the rich editor fails to initialize', async () => {
    mdxEditorState.shouldThrow = true;
    const onSurfaceChange = vi.fn();

    render(<ControlledHarness onSurfaceChange={onSurfaceChange} />);

    expect(await screen.findByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
    expect(onSurfaceChange).toHaveBeenCalledWith('raw');
  });

  it('falls back to raw mode when the rich editor reports unsupported markdown after mount', async () => {
    mdxEditorState.emitOnErrorOnMount = true;
    const onSurfaceChange = vi.fn();

    render(<ControlledHarness initialValue={'```tsx\nconst x = <div />\n```'} onSurfaceChange={onSurfaceChange} />);

    expect(await screen.findByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
    expect(onSurfaceChange).toHaveBeenCalledWith('raw');
  });

  it('normalizes nested fenced markdown examples so the rich editor can stay active', () => {
    mdxEditorState.emitOnErrorForUnsafeFenceMarkdown = true;

    render(
      <ControlledHarness
        initialValue={[
          '```markdown',
          '## Release Template',
          '',
          '- Example command:',
          '  ```bash',
          '  skillshare command --flag',
          '  ```',
          '```',
        ].join('\n')}
      />,
    );

    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toHaveValue([
      '````markdown',
      '## Release Template',
      '',
      '- Example command:',
      '  ```bash',
      '  skillshare command --flag',
      '  ```',
      '````',
    ].join('\n'));
    expect(screen.queryByRole('textbox', { name: 'Raw markdown editor' })).not.toBeInTheDocument();
  });

  it('emits save and discard callbacks without resetting controlled draft content internally', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const onDiscard = vi.fn();

    render(<ControlledHarness onSave={onSave} onDiscard={onDiscard} />);

    const richEditor = screen.getByRole('textbox', { name: 'Rich markdown editor' });
    await user.clear(richEditor);
    await user.type(richEditor, '# Updated');

    await user.click(screen.getByRole('button', { name: /^save$/i }));
    expect(onSave).toHaveBeenCalledWith('# Updated');

    await user.type(screen.getByRole('textbox', { name: 'Rich markdown editor' }), ' draft');
    await user.click(screen.getByRole('button', { name: /^discard$/i }));

    expect(onDiscard).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toHaveValue('# Updated draft');
  });

  it('renders dirty UI from the controlled isDirty prop after a draft change', async () => {
    const user = userEvent.setup();

    render(<ControlledHarness />);

    const richEditor = screen.getByRole('textbox', { name: 'Rich markdown editor' });
    await user.type(richEditor, 'extra');

    expect(screen.getByText(/unsaved changes/i)).toBeInTheDocument();
  });

  it('keeps content-changing rich edits dirty even when the editor marks them as normalization callbacks', async () => {
    mdxEditorState.forceNormalizeFlagOnInput = true;
    const user = userEvent.setup();

    render(<ControlledHarness />);

    const richEditor = screen.getByRole('textbox', { name: 'Rich markdown editor' });
    await user.type(richEditor, 'extra');

    expect(screen.getByText(/unsaved changes/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^save$/i })).toBeEnabled();
  });

  it('defers rich-editor callbacks that arrive during render so the parent still becomes dirty', async () => {
    mdxEditorState.forceNormalizeFlagOnInput = true;
    mdxEditorState.callOnChangeDuringRender = true;
    const user = userEvent.setup();

    render(<ControlledHarness />);

    await user.type(screen.getByRole('textbox', { name: 'Rich markdown editor' }), 'extra');

    expect(screen.getByText(/unsaved changes/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^save$/i })).toBeEnabled();
  });

  it('can hide the internal toolbar when the parent owns the page-level actions', () => {
    render(<ControlledHarness showToolbar={false} />);

    expect(screen.queryByRole('button', { name: /^save$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^discard$/i })).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toBeInTheDocument();
  });

  it('remounts the rich editor for external whitespace-only updates that setMarkdown would ignore', async () => {
    const user = userEvent.setup();

    render(<ExternalResetHarness />);

    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toHaveValue('Hello');

    await user.click(screen.getByRole('button', { name: /apply trailing newline/i }));

    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toHaveValue('Hello\n');
  });

  it('stretches the editor shell to full height in split mode', () => {
    render(<ControlledHarness mode="split" />);

    const editorSection = screen.getByRole('textbox', { name: 'Rich markdown editor' }).closest('section');

    expect(editorSection).toHaveClass('h-full');
    expect(editorSection).toHaveClass('min-h-0');
  });

  it('stretches the raw textarea shell to full height in split mode', () => {
    render(<ControlledHarness mode="split" initialSurface="raw" />);

    const rawEditor = screen.getByRole('textbox', { name: 'Raw markdown editor' });

    expect(rawEditor.parentElement).toHaveClass('h-full');
    expect(rawEditor.parentElement).toHaveClass('flex-1');
  });
});
