import { Component, useEffect, useRef, useState, type ReactNode } from 'react';
import {
  MDXEditor,
  codeBlockPlugin,
  codeMirrorPlugin,
  frontmatterPlugin,
  headingsPlugin,
  linkPlugin,
  listsPlugin,
  markdownShortcutPlugin,
  quotePlugin,
  tablePlugin,
  thematicBreakPlugin,
  type MDXEditorMethods,
} from '@mdxeditor/editor';
import '@mdxeditor/editor/style.css';
import Button from './Button';
import { createSkillCodeBlockEditorDescriptor } from './SkillCodeBlockEditor';
import { Textarea } from './Input';
import { skillMarkdownSubstitutionPlugin } from './SkillMarkdownSubstitutionPlugin';
import { normalizeMarkdownForRichEditor } from '../lib/skillMarkdown';

export type SkillMarkdownEditorSurface = 'rich' | 'raw';
export type SkillMarkdownEditorMode = 'edit' | 'split';
export type SkillMarkdownEditorSurfaceStyle = 'panel' | 'inline';

export const skillMarkdownCodeBlockLanguages = {
  txt: 'Plain text',
  md: 'Markdown',
  markdown: 'Markdown',
  yaml: 'YAML',
  yml: 'YAML',
  json: 'JSON',
  bash: 'Bash',
  sh: 'Shell',
  ts: 'TypeScript',
  tsx: 'TypeScript (React)',
  js: 'JavaScript',
  jsx: 'JavaScript (React)',
  css: 'CSS',
  html: 'HTML',
  py: 'Python',
  go: 'Go',
} as const;

type SkillMarkdownEditorProps = {
  value: string;
  onChange: (value: string) => void;
  onSave: (value: string) => void;
  onDiscard: () => void;
  onSurfaceChange: (surface: SkillMarkdownEditorSurface) => void;
  surface: SkillMarkdownEditorSurface;
  mode: SkillMarkdownEditorMode;
  isDirty: boolean;
  showToolbar?: boolean;
  surfaceStyle?: SkillMarkdownEditorSurfaceStyle;
};

const editorPlugins = [
  frontmatterPlugin(),
  headingsPlugin({ allowedHeadingLevels: [1, 2, 3, 4, 5, 6] }),
  listsPlugin(),
  quotePlugin(),
  thematicBreakPlugin(),
  linkPlugin(),
  tablePlugin(),
  skillMarkdownSubstitutionPlugin(),
  codeBlockPlugin({
    defaultCodeBlockLanguage: 'txt',
    codeBlockEditorDescriptors: [createSkillCodeBlockEditorDescriptor(skillMarkdownCodeBlockLanguages)],
  }),
  codeMirrorPlugin({
    codeBlockLanguages: skillMarkdownCodeBlockLanguages,
  }),
  markdownShortcutPlugin(),
];

class RichEditorBoundary extends Component<{
  children: ReactNode;
  onError: (message?: string) => void;
  resetKey: string;
}, { hasError: boolean }> {
  state = { hasError: false };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error: unknown) {
    const message = error instanceof Error ? error.message : undefined;
    this.props.onError(message);
  }

  componentDidUpdate(prevProps: Readonly<{ children: ReactNode; onError: (message?: string) => void; resetKey: string }>) {
    if (prevProps.resetKey !== this.props.resetKey && this.state.hasError) {
      this.setState({ hasError: false });
    }
  }

  render() {
    if (this.state.hasError) {
      return null;
    }

    return this.props.children;
  }
}

export default function SkillMarkdownEditor({
  value,
  onChange,
  onSave,
  onDiscard,
  onSurfaceChange,
  surface,
  mode,
  isDirty,
  showToolbar = true,
  surfaceStyle = 'panel',
}: SkillMarkdownEditorProps) {
  const editorSurfaceRef = useRef<HTMLDivElement | null>(null);
  const editorRef = useRef<MDXEditorMethods | null>(null);
  const lastLocalValueRef = useRef<string | null>(null);
  const pendingRichChangeRef = useRef<string | null>(null);
  const pendingRichFailureRef = useRef<string | null>(null);
  const richChangeFlushQueuedRef = useRef(false);
  const richFailureFlushQueuedRef = useRef(false);
  const previousValueRef = useRef(value);
  const [richError, setRichError] = useState<string | null>(null);
  const [richResetVersion, setRichResetVersion] = useState(0);
  const isSplitMode = mode === 'split';
  const isInlineSurface = surfaceStyle === 'inline';
  const richValue = normalizeMarkdownForRichEditor(value);
  const editorHeightClass = isSplitMode ? 'h-full min-h-[20rem]' : 'min-h-[28rem]';
  const richEditorResetKey = `${surface}:${richResetVersion}`;
  const surfaceClassName = isInlineSurface
    ? `${isSplitMode ? 'flex min-h-0 flex-1 flex-col' : ''}`
    : `rounded-[var(--radius-lg)] border-2 border-muted bg-surface p-3${isSplitMode ? ' flex min-h-0 flex-1 flex-col' : ''}`;
  const contentClassName = isInlineSurface
    ? `ss-skill-markdown-editor-content prose-hand max-w-none ${editorHeightClass} focus:outline-none`
    : `ss-skill-markdown-editor-content prose-hand max-w-none ${editorHeightClass} px-4 py-3 focus:outline-none`;

  useEffect(() => {
    const previousValue = previousValueRef.current;
    previousValueRef.current = value;

    if (lastLocalValueRef.current === value) {
      lastLocalValueRef.current = null;
      return;
    }

    if (surface !== 'rich' || previousValue === value) {
      return;
    }

    if (previousValue.trim() === value.trim()) {
      setRichResetVersion((current) => current + 1);
      return;
    }

    if (editorRef.current) {
      editorRef.current.setMarkdown(richValue);
    }
  }, [richValue, surface, value]);

  useEffect(() => {
    if (surface === 'rich') {
      setRichError(null);
    }
  }, [surface]);

  function handleRichFailure(message?: string) {
    pendingRichFailureRef.current = message ?? 'Rich editor unavailable for this document. Raw mode is active.';

    if (richFailureFlushQueuedRef.current) {
      return;
    }

    richFailureFlushQueuedRef.current = true;
    queueMicrotask(() => {
      richFailureFlushQueuedRef.current = false;
      const pendingMessage = pendingRichFailureRef.current ?? 'Rich editor unavailable for this document. Raw mode is active.';
      pendingRichFailureRef.current = null;
      setRichError(pendingMessage);
      onSurfaceChange('raw');
    });
  }

  function flushRichChange(next: string) {
    pendingRichChangeRef.current = next;

    if (richChangeFlushQueuedRef.current) {
      return;
    }

    richChangeFlushQueuedRef.current = true;
    queueMicrotask(() => {
      richChangeFlushQueuedRef.current = false;
      const pendingChange = pendingRichChangeRef.current;
      pendingRichChangeRef.current = null;

      if (pendingChange === null) {
        return;
      }

      lastLocalValueRef.current = pendingChange;
      onChange(pendingChange);
    });
  }

  return (
    <section
      className={`ss-skill-markdown-editor flex flex-col gap-3${isSplitMode ? ' h-full min-h-0' : ''}`}
      data-mode={mode}
      data-surface={surface}
    >
      {showToolbar ? (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2 text-sm text-pencil-light">
            {isDirty ? (
              <span className="rounded-full border border-pencil bg-paper px-3 py-1 font-medium text-pencil">
                Unsaved changes
              </span>
            ) : null}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button variant="secondary" size="sm" onClick={onDiscard} disabled={!isDirty}>
              Discard
            </Button>
            <Button size="sm" onClick={() => onSave(value)} disabled={!isDirty}>
              Save
            </Button>
          </div>
        </div>
      ) : null}

      {richError ? (
        <div
          role="alert"
          className="rounded-[var(--radius-md)] border border-danger/40 bg-danger/5 px-4 py-3 text-sm text-danger"
        >
          {richError}
        </div>
      ) : null}

      <div ref={editorSurfaceRef} className={`ss-skill-markdown-editor-surface ${surfaceClassName}`.trim()}>
        {surface === 'rich' ? (
          <RichEditorBoundary onError={handleRichFailure} resetKey={richEditorResetKey}>
            <MDXEditor
              key={richEditorResetKey}
              ref={editorRef}
              markdown={richValue}
              trim={false}
              onChange={(next, initialMarkdownNormalize) => {
                const isEditorFocused = editorSurfaceRef.current?.contains(document.activeElement) ?? false;

                if (initialMarkdownNormalize && !isEditorFocused) {
                  return;
                }
                flushRichChange(next);
              }}
              plugins={editorPlugins}
              className={`ss-skill-markdown-editor-instance ${editorHeightClass}`}
              contentEditableClassName={contentClassName}
              placeholder="Write markdown content"
              onError={({ error }) => {
                handleRichFailure(error);
              }}
            />
          </RichEditorBoundary>
        ) : (
          <Textarea
            aria-label="Raw markdown editor"
            value={value}
            onChange={(event) => {
              const next = event.currentTarget.value;
              lastLocalValueRef.current = next;
              onChange(next);
            }}
            spellCheck={false}
            wrapperClassName={isSplitMode ? 'flex h-full min-h-0 flex-1 flex-col' : undefined}
            className={`ss-skill-markdown-editor-input prose-hand w-full ${editorHeightClass} font-mono`}
          />
        )}
      </div>
    </section>
  );
}
