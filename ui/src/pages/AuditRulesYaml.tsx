import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { useT } from '../i18n';
import { FilePlus } from 'lucide-react';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { EditorView, keymap } from '@codemirror/view';
import { linter, lintGutter } from '@codemirror/lint';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ValidationError } from '../hooks/useAuditYamlValidation';
import { useAuditYamlValidation } from '../hooks/useAuditYamlValidation';
import { useLineDiff } from '../hooks/useLineDiff';
import { useCursorField } from '../hooks/useCursorField';
import Button from '../components/Button';
import EmptyState from '../components/EmptyState';
import { useToast } from '../components/Toast';
import AuditAssistantPanel from '../components/audit/AuditAssistantPanel';
import { api } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { handTheme } from '../lib/codemirror-theme';
import { PageSkeleton } from '../components/Skeleton';

/* ──────────────────────────────────────────────────────────────────────
 * Props
 * ────────────────────────────────────────────────────────────────────── */

interface AuditRulesYamlProps {
  isProjectMode: boolean;
  onSaveStateChange?: (dirty: boolean, saving: boolean, onSave: () => void) => void;
}

/* ──────────────────────────────────────────────────────────────────────
 * Helper: extract regex value from the current cursor line
 * ────────────────────────────────────────────────────────────────────── */

/** Extract the quoted or unquoted value after `regex:` on a line */
function extractRegexFromLine(line: string): string | null {
  const m = line.match(/^\s*-?\s*regex:\s*(?:'([^']*)'|"([^"]*)"|(\S.*))\s*$/);
  if (!m) return null;
  return m[1] ?? m[2] ?? m[3] ?? null;
}

/** Look for `exclude:` in nearby lines within the same rule block */
function extractExcludeNearby(lines: string[], lineIndex: number): string | null {
  // Find the indent of the current rule entry (look upward for `- ` prefix)
  let ruleStart = lineIndex;
  for (let i = lineIndex; i >= 0; i--) {
    if (/^\s+-\s/.test(lines[i])) {
      ruleStart = i;
      break;
    }
  }

  // Scan from ruleStart until next rule entry
  for (let i = ruleStart; i < lines.length; i++) {
    if (i > ruleStart && /^\s+-\s/.test(lines[i])) break;
    const m = lines[i].match(/^\s*exclude:\s*(?:'([^']*)'|"([^"]*)"|(\S.*))\s*$/);
    if (m) return m[1] ?? m[2] ?? m[3] ?? null;
  }
  return null;
}

/* ──────────────────────────────────────────────────────────────────────
 * Component
 * ────────────────────────────────────────────────────────────────────── */

export default function AuditRulesYaml({
  isProjectMode,
  onSaveStateChange,
}: AuditRulesYamlProps) {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const editorRef = useRef<EditorView | null>(null);

  // ─── Data query ───
  const rawQuery = useQuery({
    queryKey: queryKeys.audit.rules,
    queryFn: () => api.getAuditRules(),
    staleTime: staleTimes.auditRules,
  });

  // ─── Editor state ───
  const [raw, setRaw] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (rawQuery.data?.raw) {
      setRaw(rawQuery.data.raw);
      setDirty(false);
    }
  }, [rawQuery.data]);

  const handleChange = (value: string) => {
    setRaw(value);
    setDirty(value !== (rawQuery.data?.raw ?? ''));
  };

  // ─── Panel hooks ───
  const { errors } = useAuditYamlValidation(raw);
  const { fieldPath, cursorLine, extension: cursorExtension } = useCursorField();
  const { diff, changeCount } = useLineDiff(rawQuery.data?.raw ?? '', raw, true);

  // ─── Derive cursor regex / exclude from editor state ───
  const lines = useMemo(() => raw.split('\n'), [raw]);

  const cursorRegex = useMemo(() => {
    if (!fieldPath) return undefined;
    // Check if the cursor line itself contains `regex:`
    const idx = cursorLine - 1;
    if (idx < 0 || idx >= lines.length) return undefined;

    // If field path ends with regex, or the line has regex:
    if (fieldPath.endsWith('.regex') || fieldPath === 'regex') {
      return extractRegexFromLine(lines[idx]) ?? undefined;
    }
    // Also check if the current line literally has regex:
    const directExtract = extractRegexFromLine(lines[idx]);
    if (directExtract) return directExtract;

    return undefined;
  }, [fieldPath, cursorLine, lines]);

  const cursorExclude = useMemo(() => {
    if (!cursorRegex) return undefined;
    const idx = cursorLine - 1;
    return extractExcludeNearby(lines, idx) ?? undefined;
  }, [cursorRegex, cursorLine, lines]);

  // (onSaveStateChange effect moved after handleSave declaration)

  // ─── Linter (stable ref pattern from ConfigPage) ───
  const errorsRef = useRef<ValidationError[]>([]);
  errorsRef.current = errors;

  const linterExtension = useMemo(
    () =>
      linter((view) => {
        return errorsRef.current.map(err => {
          const lineObj = view.state.doc.line(Math.min(err.line, view.state.doc.lines));
          return {
            from: lineObj.from,
            to: lineObj.to,
            severity: err.severity === 'error' ? 'error' as const : 'warning' as const,
            message: err.message,
          };
        });
      }, { delay: 350 }),
    [],
  );

  // ─── Save handler via ref (for Cmd+S keymap) ───
  const saveRef = useRef<() => void>(() => {});

  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      await api.putAuditRules(raw);
      toast(t('auditRulesYaml.toast.saved'), 'success');
      setDirty(false);
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.rules });
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.compiled });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  }, [raw, toast, queryClient]);

  saveRef.current = handleSave;

  // Notify parent of save state for PageHeader Save button
  useEffect(() => {
    onSaveStateChange?.(dirty, saving, () => saveRef.current());
  }, [dirty, saving, onSaveStateChange]);

  const saveKeymap = useMemo(
    () =>
      keymap.of([{
        key: 'Mod-s',
        run: () => { saveRef.current(); return true; },
      }]),
    [],
  );

  // ─── Create handler ───
  const handleCreate = async () => {
    setCreating(true);
    try {
      await api.initAuditRules();
      toast(t('auditRulesYaml.toast.created'), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.rules });
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.compiled });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setCreating(false);
    }
  };

  // ─── Revert handler ───
  const handleRevert = useCallback(() => {
    setRaw(rawQuery.data?.raw ?? '');
    setDirty(false);
  }, [rawQuery.data]);

  // ─── Extensions ───
  const extensions = useMemo(
    () => [
      yaml(),
      EditorView.lineWrapping,
      ...handTheme,
      lintGutter(),
      linterExtension,
      cursorExtension,
      saveKeymap,
    ],
    [linterExtension, cursorExtension, saveKeymap],
  );

  // ─── Loading / error states ───
  if (rawQuery.isPending) return <PageSkeleton />;
  if (rawQuery.error) {
    return (
      <div className="ss-note bad">
        <span className="flex-1">
          <b>{t('auditRules.error.failedToLoad')}</b> {rawQuery.error.message}
        </span>
      </div>
    );
  }

  const panel = (
    <AuditAssistantPanel
      errors={errors}
      changeCount={changeCount}
      fieldPath={fieldPath}
      cursorLine={cursorLine}
      source={raw}
      diff={diff}
      editorRef={editorRef}
      onRevert={handleRevert}
      cursorRegex={cursorRegex}
      cursorExclude={cursorExclude}
    />
  );

  // The file can be long, so the panel stays where the eye is.
  const layout = (content: React.ReactNode) => (
    <div className="grid grid-cols-[minmax(0,1fr)_300px] items-start gap-6">
      <div className="flex min-w-0 flex-col gap-3">{content}</div>
      <div className="sticky top-6">{panel}</div>
    </div>
  );

  // Nothing to inspect until the file exists, so the panel stays out of the way.
  if (rawQuery.data && !rawQuery.data.exists) {
    return (
      <EmptyState
        icon={FilePlus}
        title={t('auditRulesYaml.empty.title')}
        description={t(isProjectMode ? 'auditRulesYaml.empty.descriptionProject' : 'auditRulesYaml.empty.descriptionGlobal')}
        action={
          <Button variant="primary" onClick={handleCreate} disabled={creating}>
            <FilePlus size={15} />
            {creating ? t('auditRulesYaml.creating') : t('auditRulesYaml.createButton')}
          </Button>
        }
      />
    );
  }

  return layout(
    <>
      <div className="ss-code !overflow-hidden !p-0">
        <CodeMirror
          value={raw}
          onChange={handleChange}
          extensions={extensions}
          theme="none"
          height="500px"
          onCreateEditor={(view) => { editorRef.current = view; }}
          basicSetup={{
            lineNumbers: true,
            foldGutter: true,
            highlightActiveLine: true,
            highlightSelectionMatches: true,
            bracketMatching: true,
            indentOnInput: true,
            autocompletion: false,
          }}
        />
      </div>
      <div className="flex items-center justify-between gap-4">
        <span className="font-mono text-[13px] text-ink-3">{rawQuery.data!.path}</span>
        <span className="text-[13px] text-ink-3">{t('config.saveShortcutHint')}</span>
      </div>
    </>,
  );
}
