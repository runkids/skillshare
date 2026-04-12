import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSkillCodeBlockEditorDescriptor } from './SkillCodeBlockEditor';

const codeBlockContextState = vi.hoisted(() => ({
  parentEditor: {
    update: vi.fn((callback: () => void) => callback()),
    focus: vi.fn(),
    dispatchCommand: vi.fn(),
  },
  lexicalNode: {
    getKey: vi.fn(() => 'code-block-1'),
    setLanguage: vi.fn(),
    getLatest: vi.fn(() => ({
      select: vi.fn(),
    })),
  },
  setCode: vi.fn(),
}));

const lexicalState = vi.hoisted(() => ({
  addUpdateTag: vi.fn(),
  currentNode: {
    selectNext: vi.fn(),
    remove: vi.fn(),
  },
  undoCommand: Symbol('UNDO_COMMAND'),
  redoCommand: Symbol('REDO_COMMAND'),
}));

vi.mock('@mdxeditor/editor', () => ({
  useCodeBlockEditorContext: () => codeBlockContextState,
}));

vi.mock('./Select', () => ({
  Select: ({
    value,
    onChange,
    options,
  }: {
    value: string;
    onChange: (value: string) => void;
    options: Array<{ value: string; label: string }>;
  }) => (
    <select
      aria-label="Code block language"
      value={value}
      onChange={(event) => onChange(event.currentTarget.value)}
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  ),
}));

vi.mock('@codemirror/state', () => ({
  EditorState: {
    create: vi.fn(() => ({})),
  },
}));

vi.mock('@codemirror/commands', () => ({
  indentWithTab: {},
}));

vi.mock('@codemirror/language-data', () => ({
  languages: [],
}));

vi.mock('codemirror', () => ({
  basicSetup: {},
}));

vi.mock('cm6-theme-basic-light', () => ({
  basicLight: {},
}));

vi.mock('@codemirror/view', () => {
  class MockEditorView {
    static lineWrapping = {};
    static updateListener = {
      of: vi.fn(() => ({})),
    };
    static domEventHandlers = vi.fn(() => ({}));

    destroy = vi.fn();
    focus = vi.fn();

    constructor({ parent }: { parent: HTMLElement }) {
      const editor = document.createElement('div');
      editor.className = 'cm-editor';
      parent.appendChild(editor);
    }
  }

  return {
    EditorView: MockEditorView,
    lineNumbers: vi.fn(() => ({})),
    keymap: {
      of: vi.fn(() => ({})),
    },
  };
});

vi.mock('lexical', () => ({
  $addUpdateTag: (...args: unknown[]) => lexicalState.addUpdateTag(...args),
  $getNodeByKey: vi.fn(() => lexicalState.currentNode),
  $setSelection: vi.fn(),
  UNDO_COMMAND: lexicalState.undoCommand,
  REDO_COMMAND: lexicalState.redoCommand,
}));

describe('SkillCodeBlockEditor', () => {
  beforeEach(() => {
    codeBlockContextState.parentEditor.update.mockClear();
    codeBlockContextState.parentEditor.focus.mockClear();
    codeBlockContextState.parentEditor.dispatchCommand.mockClear();
    codeBlockContextState.lexicalNode.setLanguage.mockClear();
    codeBlockContextState.lexicalNode.getLatest.mockClear();
    lexicalState.addUpdateTag.mockClear();
    lexicalState.currentNode.selectNext.mockClear();
    lexicalState.currentNode.remove.mockClear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('matches known code-block languages and ignores meta blocks', () => {
    const descriptor = createSkillCodeBlockEditorDescriptor({
      md: 'Markdown',
      bash: 'Bash',
    });

    expect(descriptor.match('bash', '')).toBe(true);
    expect(descriptor.match('bash', 'title=\"demo\"')).toBe(false);
    expect(descriptor.match('unknown', '')).toBe(false);
  });

  it('pushes code block deletion into history and refocuses the editor', async () => {
    const descriptor = createSkillCodeBlockEditorDescriptor({
      md: 'Markdown',
      bash: 'Bash',
    });
    const Editor = descriptor.Editor;

    render(
      <Editor
        code={'echo "hello"'}
        language="bash"
        meta=""
        nodeKey="code-block-1"
        focusEmitter={{ subscribe: () => {} }}
      />,
    );

    fireEvent.mouseDown(screen.getByRole('button', { name: /delete code block/i }));
    fireEvent.click(screen.getByRole('button', { name: /delete code block/i }));

    expect(lexicalState.addUpdateTag).toHaveBeenCalledWith('history-push');
    expect(lexicalState.currentNode.selectNext).toHaveBeenCalledTimes(1);
    expect(lexicalState.currentNode.remove).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.runAllTimers();
    });

    expect(codeBlockContextState.parentEditor.focus).toHaveBeenCalledTimes(1);
  });

  it('routes cmd-z from the toolbar back into the parent editor history', () => {
    const descriptor = createSkillCodeBlockEditorDescriptor({
      md: 'Markdown',
      bash: 'Bash',
    });
    const Editor = descriptor.Editor;

    render(
      <Editor
        code={'echo "hello"'}
        language="bash"
        meta=""
        nodeKey="code-block-1"
        focusEmitter={{ subscribe: () => {} }}
      />,
    );

    fireEvent.keyDown(screen.getByRole('combobox', { name: /code block language/i }), {
      key: 'z',
      metaKey: true,
    });

    expect(codeBlockContextState.parentEditor.dispatchCommand).toHaveBeenCalledWith(lexicalState.undoCommand, undefined);
    expect(codeBlockContextState.parentEditor.focus).toHaveBeenCalledTimes(1);
  });
});
