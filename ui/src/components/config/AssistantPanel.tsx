import { useState, useCallback } from 'react';
import { List, GitCompare, Unlock, EyeOff } from 'lucide-react';
import type { EditorView } from '@codemirror/view';
import type { ValidationError } from '../../hooks/useYamlValidation';
import type { DiffResult } from '../../hooks/useLineDiff';
import Badge from '../Badge';
import ConfigStatusBar from './ConfigStatusBar';
import ErrorList from './ErrorList';
import FieldDocs from './FieldDocs';
import StructureTree from './StructureTree';
import DiffPreview from './DiffPreview';

type LockedView = 'auto' | 'structure' | 'diff';

interface Props {
  errors: ValidationError[];
  changeCount: number;
  fieldPath: string | null;
  cursorLine: number;
  source: string;
  diff: DiffResult;
  editorRef: React.RefObject<EditorView | null>;
  collapsed: boolean;
  onToggleCollapse: () => void;
  onRevert: () => void;
  schemaUnavailable?: boolean;
  mode?: 'config' | 'skillignore';
  ignoredSkills?: string[];
}

export default function AssistantPanel({
  errors,
  changeCount,
  fieldPath,
  cursorLine,
  source,
  diff,
  editorRef,
  collapsed,
  onToggleCollapse,
  onRevert,
  schemaUnavailable = false,
  mode = 'config',
  ignoredSkills = [],
}: Props) {
  const [lockedView, setLockedView] = useState<LockedView>('auto');

  const jumpToLine = useCallback(
    (line: number) => {
      const view = editorRef.current;
      if (!view) return;
      const lineInfo = view.state.doc.line(Math.min(line, view.state.doc.lines));
      view.dispatch({ selection: { anchor: lineInfo.from }, scrollIntoView: true });
      view.focus();
    },
    [editorRef],
  );

  const handleErrorsClick = useCallback(() => {
    // Scroll to errors view — just ensure auto mode shows ErrorList
    setLockedView('auto');
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Escape' && lockedView !== 'auto') {
        setLockedView('auto');
      }
    },
    [lockedView],
  );

  const toggleLock = useCallback((view: 'structure' | 'diff') => {
    setLockedView(prev => (prev === view ? 'auto' : view));
  }, []);

  // Determine which context panel to render
  const renderContextArea = () => {
    if (mode === 'skillignore') {
      return (
        <div className="flex flex-col gap-1 p-3">
          <p className="text-xs font-medium text-pencil-light uppercase tracking-wide mb-2 flex items-center gap-1.5">
            <EyeOff size={12} strokeWidth={2} />
            Ignored Skills
          </p>
          {ignoredSkills.length === 0 ? (
            <p className="text-xs text-pencil-light italic">No skills ignored yet.</p>
          ) : (
            <ul className="flex flex-col gap-0.5">
              {ignoredSkills.map(skill => (
                <li key={skill} className="text-xs text-pencil font-mono bg-paper rounded px-2 py-0.5">
                  {skill}
                </li>
              ))}
            </ul>
          )}
        </div>
      );
    }

    // Config mode
    if (lockedView === 'structure') {
      return <StructureTree source={source} cursorLine={cursorLine} parseError={errors.some(e => e.severity === 'error')} onClickNode={jumpToLine} />;
    }

    if (lockedView === 'diff') {
      return <DiffPreview diff={diff} onClickLine={jumpToLine} onRevert={onRevert} />;
    }

    // Auto mode
    if (errors.length > 0) {
      return <ErrorList errors={errors} onClickError={jumpToLine} />;
    }

    if (fieldPath) {
      return <FieldDocs fieldPath={fieldPath} />;
    }

    return <StructureTree source={source} cursorLine={cursorLine} parseError={errors.some(e => e.severity === 'error')} onClickNode={jumpToLine} />;
  };

  return (
    <div
      className="ss-assistant-panel flex flex-col h-full border-l border-muted bg-surface"
      onKeyDown={handleKeyDown}
    >
      {/* Status bar */}
      <ConfigStatusBar
        errors={errors}
        changeCount={changeCount}
        collapsed={collapsed}
        onToggleCollapse={onToggleCollapse}
        onErrorsClick={handleErrorsClick}
        schemaUnavailable={schemaUnavailable}
        mode={mode}
      />

      {/* Separator */}
      <div className="border-t border-dashed border-pencil-light/30" />

      {/* Context area */}
      <div className="flex-1 overflow-y-auto animate-fade-in">{renderContextArea()}</div>

      {/* Bottom bar — config mode only */}
      {mode === 'config' && (
        <div className="flex items-center gap-1 px-2 py-1.5 border-t border-muted/40 bg-paper">
          {/* Structure button */}
          <button
            type="button"
            aria-pressed={lockedView === 'structure'}
            onClick={() => toggleLock('structure')}
            className={`inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-medium transition-all duration-150 ${
              lockedView === 'structure'
                ? 'bg-blue/10 text-blue border border-blue/20'
                : 'text-pencil-light hover:text-pencil border border-transparent'
            }`}
          >
            <List size={12} strokeWidth={2} />
            Structure
          </button>

          {/* Diff button */}
          <button
            type="button"
            aria-pressed={lockedView === 'diff'}
            onClick={() => toggleLock('diff')}
            className={`inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-medium transition-all duration-150 ${
              lockedView === 'diff'
                ? 'bg-blue/10 text-blue border border-blue/20'
                : 'text-pencil-light hover:text-pencil border border-transparent'
            }`}
          >
            <GitCompare size={12} strokeWidth={2} />
            Diff
          </button>

          {/* Spacer */}
          <span className="flex-1" />

          {/* Auto button — shown when locked */}
          {lockedView !== 'auto' && (
            <button
              type="button"
              onClick={() => setLockedView('auto')}
              className="transition-all duration-150"
            >
              <Badge variant="default">
                <Unlock size={10} strokeWidth={2} />
                Auto
              </Badge>
            </button>
          )}
        </div>
      )}
    </div>
  );
}
