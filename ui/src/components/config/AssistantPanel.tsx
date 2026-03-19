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

      {/* Context area */}
      <div className="ss-panel-content flex-1 overflow-y-auto animate-fade-in">{renderContextArea()}</div>

      {/* Bottom bar — config mode only */}
      {mode === 'config' && (
        <div className="ss-panel-toolbar flex items-center gap-2 px-2 py-1.5 border-t border-muted/40 bg-paper">
          <div className="ss-panel-tabs inline-flex items-center p-0.5 bg-muted/20 border border-muted/40 rounded-[var(--radius-sm)]">
            <button
              type="button"
              aria-pressed={lockedView === 'structure'}
              onClick={() => toggleLock('structure')}
              className={`ss-panel-tab inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[3px] text-xs font-medium transition-all duration-150 cursor-pointer ${
                lockedView === 'structure'
                  ? 'bg-surface text-pencil shadow-sm'
                  : 'text-pencil-light hover:text-pencil'
              }`}
            >
              <List size={12} strokeWidth={2} />
              Structure
            </button>
            <button
              type="button"
              aria-pressed={lockedView === 'diff'}
              onClick={() => toggleLock('diff')}
              className={`ss-panel-tab inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[3px] text-xs font-medium transition-all duration-150 cursor-pointer ${
                lockedView === 'diff'
                  ? 'bg-surface text-pencil shadow-sm'
                  : 'text-pencil-light hover:text-pencil'
              }`}
            >
              <GitCompare size={12} strokeWidth={2} />
              Diff
            </button>
          </div>

          <span className="flex-1" />

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
