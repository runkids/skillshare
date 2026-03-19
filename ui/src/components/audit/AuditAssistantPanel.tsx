import { useState, useCallback, useEffect } from 'react';
import { List, GitCompare, FlaskConical, Unlock } from 'lucide-react';
import type { EditorView } from '@codemirror/view';
import type { ValidationError } from '../../hooks/useYamlValidation';
import type { DiffResult } from '../../hooks/useLineDiff';
import Badge from '../Badge';
import ConfigStatusBar from '../config/ConfigStatusBar';
import ErrorList from '../config/ErrorList';
import FieldDocs from '../config/FieldDocs';
import StructureTree from '../config/StructureTree';
import DiffPreview from '../config/DiffPreview';
import RegexTester from './RegexTester';
import { auditFieldDocs } from '../../lib/auditFieldDocs';

type LockedView = 'auto' | 'structure' | 'diff' | 'test';

interface Props {
  mode: 'yaml' | 'structured';
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
  cursorRegex?: string;
  cursorExclude?: string;
}

export default function AuditAssistantPanel({
  mode,
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
  cursorRegex,
  cursorExclude,
}: Props) {
  const [lockedView, setLockedView] = useState<LockedView>('auto');
  const [regexPattern, setRegexPattern] = useState(cursorRegex ?? '');

  // Sync regexPattern when cursorRegex changes (only in auto mode)
  useEffect(() => {
    if (lockedView === 'auto') {
      setRegexPattern(cursorRegex ?? '');
    }
  }, [cursorRegex, lockedView]);

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

  const toggleLock = useCallback((view: 'structure' | 'diff' | 'test') => {
    setLockedView(prev => (prev === view ? 'auto' : view));
  }, []);

  if (collapsed) {
    return (
      <div className="ss-audit-assistant-panel flex flex-col h-full border-l border-muted bg-surface">
        <ConfigStatusBar
          errors={errors}
          changeCount={changeCount}
          collapsed={collapsed}
          onToggleCollapse={onToggleCollapse}
          onErrorsClick={handleErrorsClick}
          mode="audit"
        />
      </div>
    );
  }

  // Structured mode — Phase 2 placeholder
  if (mode === 'structured') {
    return (
      <div className="ss-audit-assistant-panel flex flex-col h-full border-l border-muted bg-surface">
        <ConfigStatusBar
          errors={errors}
          changeCount={changeCount}
          collapsed={collapsed}
          onToggleCollapse={onToggleCollapse}
          onErrorsClick={handleErrorsClick}
          mode="audit"
        />
        <div className="p-3 text-sm text-pencil-light">Structured view panel — coming in Phase 2</div>
      </div>
    );
  }

  // YAML mode
  const renderContextArea = () => {
    if (lockedView === 'structure') {
      return (
        <StructureTree
          source={source}
          cursorLine={cursorLine}
          parseError={errors.some(e => e.severity === 'error')}
          onClickNode={jumpToLine}
        />
      );
    }

    if (lockedView === 'diff') {
      return <DiffPreview diff={diff} onClickLine={jumpToLine} onRevert={onRevert} />;
    }

    if (lockedView === 'test') {
      return (
        <RegexTester
          pattern={regexPattern}
          excludePattern={cursorExclude}
          onPatternChange={setRegexPattern}
        />
      );
    }

    // Auto mode
    if (errors.length > 0) {
      return <ErrorList errors={errors} onClickError={jumpToLine} />;
    }

    if (cursorRegex) {
      return (
        <RegexTester
          pattern={regexPattern}
          excludePattern={cursorExclude}
          onPatternChange={setRegexPattern}
        />
      );
    }

    if (fieldPath) {
      return <FieldDocs fieldPath={fieldPath} docs={auditFieldDocs} />;
    }

    return (
      <StructureTree
        source={source}
        cursorLine={cursorLine}
        parseError={errors.some(e => e.severity === 'error')}
        onClickNode={jumpToLine}
      />
    );
  };

  return (
    <div
      className="ss-audit-assistant-panel flex flex-col h-full border-l border-muted bg-surface"
      onKeyDown={handleKeyDown}
    >
      {/* Status bar */}
      <ConfigStatusBar
        errors={errors}
        changeCount={changeCount}
        collapsed={collapsed}
        onToggleCollapse={onToggleCollapse}
        onErrorsClick={handleErrorsClick}
        mode="audit"
      />

      {/* Context area */}
      <div className="ss-panel-content flex-1 overflow-y-auto animate-fade-in">{renderContextArea()}</div>

      {/* Bottom bar */}
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
          <button
            type="button"
            aria-pressed={lockedView === 'test'}
            onClick={() => toggleLock('test')}
            className={`ss-panel-tab inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[3px] text-xs font-medium transition-all duration-150 cursor-pointer ${
              lockedView === 'test'
                ? 'bg-surface text-pencil shadow-sm'
                : 'text-pencil-light hover:text-pencil'
            }`}
          >
            <FlaskConical size={12} strokeWidth={2} />
            Test
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
    </div>
  );
}
