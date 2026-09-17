import { useCallback, useEffect, useState } from 'react';
import type { EditorView } from '@codemirror/view';
import type { ValidationError } from '../../hooks/useYamlValidation';
import type { DiffResult } from '../../hooks/useLineDiff';
import { useT } from '../../i18n';
import ErrorList from '../config/ErrorList';
import FieldDocs from '../config/FieldDocs';
import StructureTree from '../config/StructureTree';
import DiffPreview from '../config/DiffPreview';
import RegexTester from './RegexTester';
import { auditFieldDocs } from '../../lib/auditFieldDocs';

type View = 'field' | 'structure' | 'changes' | 'test';

const VIEWS: View[] = ['field', 'structure', 'changes', 'test'];

interface Props {
  errors: ValidationError[];
  changeCount: number;
  fieldPath: string | null;
  cursorLine: number;
  source: string;
  diff: DiffResult;
  editorRef: React.RefObject<EditorView | null>;
  onRevert: () => void;
  cursorRegex?: string;
  cursorExclude?: string;
}

/** The panel beside the rules editor: field docs, file shape, changes, or a regex bench. */
export default function AuditAssistantPanel({
  errors,
  changeCount,
  fieldPath,
  cursorLine,
  source,
  diff,
  editorRef,
  onRevert,
  cursorRegex,
  cursorExclude,
}: Props) {
  const t = useT();
  const [view, setView] = useState<View>('field');
  const [pattern, setPattern] = useState(cursorRegex ?? '');

  // Moving the cursor onto another rule loads that rule's pattern into the bench
  useEffect(() => {
    if (cursorRegex) setPattern(cursorRegex);
  }, [cursorRegex]);

  const jumpToLine = useCallback(
    (line: number) => {
      const editor = editorRef.current;
      if (!editor) return;
      const info = editor.state.doc.line(Math.min(line, editor.state.doc.lines));
      editor.dispatch({ selection: { anchor: info.from }, scrollIntoView: true });
      editor.focus();
    },
    [editorRef],
  );

  const errorCount = errors.filter((e) => e.severity === 'error').length;
  const warningCount = errors.length - errorCount;

  return (
    <div className="ss-box flex flex-col gap-3.5 !p-3.5">
      <div className="flex items-center justify-between gap-3">
        {errors.length === 0 ? (
          <span className="ss-st ok">Valid YAML</span>
        ) : (
          <span className={`ss-st ${errorCount > 0 ? 'bad' : 'warn'}`}>
            {[
              errorCount > 0 && t(errorCount === 1 ? 'config.panel.errors.one' : 'config.panel.errors.other', { count: errorCount }),
              warningCount > 0 && t(warningCount === 1 ? 'config.panel.warnings.one' : 'config.panel.warnings.other', { count: warningCount }),
            ].filter(Boolean).join(', ')}
          </span>
        )}
        <span className="text-xs text-ink-3">
          {errorCount > 0
            ? t('config.panel.saveBlocked')
            : changeCount > 0
              ? t(changeCount === 1 ? 'config.panel.changes.one' : 'config.panel.changes.other', { count: changeCount })
              : t('config.panel.noChanges')}
        </span>
      </div>

      {errors.length > 0 ? (
        <>
          <div className="max-h-[420px] overflow-y-auto">
            <ErrorList errors={errors} onClickError={jumpToLine} />
          </div>
          <p className="text-xs text-ink-3">{t('config.panel.errorHint')}</p>
        </>
      ) : (
        <>
          <div className="ss-seg self-start" role="radiogroup" aria-label={t('audit.tab.rules')}>
            {VIEWS.map((v) => (
              <button key={v} type="button" role="radio" aria-checked={view === v} className={view === v ? 'on' : ''} onClick={() => setView(v)}>
                {t(`config.panel.tab.${v}`)}
              </button>
            ))}
          </div>
          <div className="max-h-[420px] overflow-y-auto">
            {view === 'field' ? (
              fieldPath ? <FieldDocs fieldPath={fieldPath} docs={auditFieldDocs} /> : <p className="text-[13px] text-ink-3">{t('config.panel.fieldHint')}</p>
            ) : view === 'structure' ? (
              <StructureTree source={source} cursorLine={cursorLine} parseError={false} onClickNode={jumpToLine} />
            ) : view === 'changes' ? (
              <DiffPreview diff={diff} onClickLine={jumpToLine} onRevert={onRevert} />
            ) : (
              <RegexTester pattern={pattern} excludePattern={cursorExclude} onPatternChange={setPattern} />
            )}
          </div>
        </>
      )}
    </div>
  );
}
