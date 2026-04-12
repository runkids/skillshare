import { useEffect, useMemo, useRef, type KeyboardEvent as ReactKeyboardEvent } from 'react';
import type { CodeBlockEditorDescriptor, CodeBlockEditorProps } from '@mdxeditor/editor';
import { useCodeBlockEditorContext } from '@mdxeditor/editor';
import { indentWithTab } from '@codemirror/commands';
import { EditorState } from '@codemirror/state';
import { languages } from '@codemirror/language-data';
import { EditorView, keymap, lineNumbers } from '@codemirror/view';
import { basicSetup } from 'codemirror';
import { basicLight } from 'cm6-theme-basic-light';
import { Trash2 } from 'lucide-react';
import { $addUpdateTag, $getNodeByKey, $setSelection, REDO_COMMAND, UNDO_COMMAND } from 'lexical';
import { Select } from './Select';

const EMPTY_LANGUAGE_VALUE = '__EMPTY_VALUE__';

type CodeBlockLanguageMap = Record<string, string>;

type NormalizedCodeBlockLanguages = {
  items: Array<{ value: string; label: string }>;
  keyMap: Record<string, string>;
};

function normalizeCodeBlockLanguages(input: CodeBlockLanguageMap): NormalizedCodeBlockLanguages {
  const items: Array<{ value: string; label: string }> = [];
  const keyMap: Record<string, string> = {};
  const firstKeyByLabel: Record<string, string> = {};

  for (const [key, label] of Object.entries(input)) {
    if (!(label in firstKeyByLabel)) {
      firstKeyByLabel[label] = key;
      items.push({ value: key || EMPTY_LANGUAGE_VALUE, label });
    }
    keyMap[key] = firstKeyByLabel[label] || EMPTY_LANGUAGE_VALUE;
  }

  return { items, keyMap };
}

function resolveLanguageExtension(language: string) {
  if (!language) return null;

  const languageData = languages.find((entry) => (
    entry.name === language
    || entry.alias.includes(language)
    || entry.extensions.includes(language)
  ));

  if (!languageData) return null;
  return languageData.load();
}

function SkillCodeBlockEditor({
  code,
  language,
  focusEmitter,
  codeBlockLanguages,
}: CodeBlockEditorProps & { codeBlockLanguages: CodeBlockLanguageMap }) {
  const { parentEditor, lexicalNode, setCode } = useCodeBlockEditorContext();
  const editorViewRef = useRef<EditorView | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const setCodeRef = useRef(setCode);
  const normalizedLanguages = useMemo(
    () => normalizeCodeBlockLanguages(codeBlockLanguages),
    [codeBlockLanguages],
  );

  setCodeRef.current = setCode;

  useEffect(() => {
    focusEmitter.subscribe(() => {
      editorViewRef.current?.focus();
    });
  }, [focusEmitter]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    let cancelled = false;
    let localView: EditorView | null = null;

    void (async () => {
      const extensions = [
        basicSetup,
        basicLight,
        lineNumbers(),
        keymap.of([indentWithTab]),
        EditorView.lineWrapping,
        EditorView.updateListener.of(({ state }) => {
          setCodeRef.current(state.doc.toString());
        }),
        EditorView.domEventHandlers({
          focus: () => {
            parentEditor.update(() => {
              $setSelection(null);
            });
          },
        }),
      ];

      const resolvedLanguage = normalizedLanguages.keyMap[language] ?? language;
      const support = await resolveLanguageExtension(resolvedLanguage);
      if (support) {
        extensions.push(support.extension);
      }

      if (cancelled) return;

      container.innerHTML = '';
      localView = new EditorView({
        parent: container,
        state: EditorState.create({
          doc: code,
          extensions,
        }),
      });
      editorViewRef.current = localView;
      container.addEventListener('keydown', stopPropagationHandler);
    })();

    return () => {
      cancelled = true;
      container.removeEventListener('keydown', stopPropagationHandler);
      localView?.destroy();
      if (editorViewRef.current === localView) {
        editorViewRef.current = null;
      }
    };
  }, [code, language, normalizedLanguages, parentEditor]);

  const selectedLanguage = (normalizedLanguages.keyMap[language] ?? language) || EMPTY_LANGUAGE_VALUE;

  function handleToolbarKeyDown(event: ReactKeyboardEvent<HTMLDivElement>) {
    const modifierPressed = event.metaKey || event.ctrlKey;
    if (!modifierPressed) return;

    const key = event.key.toLowerCase();
    if (key === 'z') {
      event.preventDefault();
      parentEditor.dispatchCommand(event.shiftKey ? REDO_COMMAND : UNDO_COMMAND, undefined);
      parentEditor.focus();
      return;
    }

    if (key === 'y') {
      event.preventDefault();
      parentEditor.dispatchCommand(REDO_COMMAND, undefined);
      parentEditor.focus();
    }
  }

  return (
    <div className="ss-skill-codeblock-editor">
      <div className="ss-skill-codeblock-toolbar" onKeyDown={handleToolbarKeyDown}>
        <Select
          className="ss-skill-codeblock-language-select min-w-48"
          size="sm"
          value={selectedLanguage}
          onChange={(nextLanguage) => {
            parentEditor.update(() => {
              lexicalNode.setLanguage(nextLanguage === EMPTY_LANGUAGE_VALUE ? '' : nextLanguage);
              lexicalNode.getLatest().select();
            });
          }}
          options={normalizedLanguages.items.map((item) => ({
            value: item.value,
            label: item.label,
          }))}
        />
        <button
          className="ss-skill-codeblock-delete"
          type="button"
          title="Delete code block"
          aria-label="Delete code block"
          onMouseDown={(event) => {
            event.preventDefault();
          }}
          onClick={(event) => {
            event.preventDefault();
            parentEditor.update(() => {
              $addUpdateTag('history-push');
              const node = $getNodeByKey(lexicalNode.getKey());
              if (!node) return;
              node.selectNext();
              node.remove();
            }, { discrete: true });
            const focusParentEditor = () => {
              parentEditor.focus();
            };

            if (typeof requestAnimationFrame === 'function') {
              requestAnimationFrame(focusParentEditor);
            } else {
              setTimeout(focusParentEditor, 0);
            }
          }}
        >
          <Trash2 size={16} strokeWidth={2.1} />
        </button>
      </div>
      <div ref={containerRef} className="ss-skill-codeblock-surface" />
    </div>
  );
}

function stopPropagationHandler(event: KeyboardEvent) {
  event.stopPropagation();
}

export function createSkillCodeBlockEditorDescriptor(codeBlockLanguages: CodeBlockLanguageMap): CodeBlockEditorDescriptor {
  const normalizedLanguages = normalizeCodeBlockLanguages(codeBlockLanguages);

  return {
    priority: 10,
    match(language, meta) {
      return !meta && Object.hasOwn(normalizedLanguages.keyMap, language ?? '');
    },
    Editor: (props) => (
      <SkillCodeBlockEditor
        {...props}
        codeBlockLanguages={codeBlockLanguages}
      />
    ),
  };
}
