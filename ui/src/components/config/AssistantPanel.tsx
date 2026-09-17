import { useCallback, useState } from 'react';
import type { EditorView } from '@codemirror/view';
import type { ValidationError } from '../../hooks/useYamlValidation';
import type { DiffResult } from '../../hooks/useLineDiff';
import { useT } from '../../i18n';
import ErrorList from './ErrorList';
import FieldDocs from './FieldDocs';
import StructureTree from './StructureTree';
import DiffPreview from './DiffPreview';

type View = 'field' | 'structure' | 'changes';

const VIEWS: View[] = ['field', 'structure', 'changes'];

interface Props {
  errors: ValidationError[];
  changeCount: number;
  fieldPath: string | null;
  cursorLine: number;
  source: string;
  diff: DiffResult;
  editorRef: React.RefObject<EditorView | null>;
  onRevert: () => void;
  mode?: 'config' | 'skillignore' | 'agentignore';
  ignoredSkills?: string[];
  ignoredAgents?: string[];
}

/** The panel beside the editor: what the cursor is on, the file's shape, or what changed. */
export default function AssistantPanel({
  errors,
  changeCount,
  fieldPath,
  cursorLine,
  source,
  diff,
  editorRef,
  onRevert,
  mode = 'config',
  ignoredSkills = [],
  ignoredAgents = [],
}: Props) {
  const t = useT();
  const [view, setView] = useState<View>('field');

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

  if (mode !== 'config') {
    const agents = mode === 'agentignore';
    const items = agents ? ignoredAgents : ignoredSkills;
    return (
      <div className="ss-box flex flex-col gap-3.5 !p-3.5">
        <div className="flex items-center justify-between gap-3">
          <span className="text-[13px] font-semibold">{t(agents ? 'config.ignore.ignoredAgents' : 'config.ignore.ignoredSkills')}</span>
          <span className="text-xs text-ink-3">{items.length}</span>
        </div>
        {items.length === 0 ? (
          <p className="text-[13px] text-ink-3">{t(agents ? 'config.ignore.noneAgents' : 'config.ignore.noneSkills')}</p>
        ) : (
          <div className="ss-list !shadow-none max-h-[420px] overflow-y-auto">
            {items.map((item) => (
              <div key={item} className="ss-r !min-h-[34px] !px-2.5">
                <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{item}</span>
              </div>
            ))}
          </div>
        )}
        <p className="text-xs text-ink-3">{t('config.ignore.syntaxNote')}</p>
      </div>
    );
  }

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
          <div className="ss-seg self-start" role="radiogroup" aria-label={t('settings.tab.files')}>
            {VIEWS.map((v) => (
              <button key={v} type="button" role="radio" aria-checked={view === v} className={view === v ? 'on' : ''} onClick={() => setView(v)}>
                {t(`config.panel.tab.${v}`)}
              </button>
            ))}
          </div>
          <div className="max-h-[420px] overflow-y-auto">
            {view === 'field' ? (
              fieldPath ? <FieldDocs fieldPath={fieldPath} /> : <p className="text-[13px] text-ink-3">{t('config.panel.fieldHint')}</p>
            ) : view === 'structure' ? (
              <StructureTree source={source} cursorLine={cursorLine} parseError={false} onClickNode={jumpToLine} />
            ) : (
              <DiffPreview diff={diff} onClickLine={jumpToLine} onRevert={onRevert} />
            )}
          </div>
        </>
      )}
    </div>
  );
}
