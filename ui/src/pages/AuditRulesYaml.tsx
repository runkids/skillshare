import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { Save, FileCode, FilePlus, PanelRightOpen } from 'lucide-react';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { EditorView, keymap } from '@codemirror/view';
import { linter, lintGutter } from '@codemirror/lint';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ValidationError } from '../hooks/useAuditYamlValidation';
import { useAuditYamlValidation } from '../hooks/useAuditYamlValidation';
import { useLineDiff } from '../hooks/useLineDiff';
import { useCursorField } from '../hooks/useCursorField';
import Card from '../components/Card';
import Button from '../components/Button';
import EmptyState from '../components/EmptyState';
import IconButton from '../components/IconButton';
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
  panelCollapsed: boolean;
  onTogglePanel: () => void;
  isProjectMode: boolean;
  pendingRegexTest?: { regex: string; exclude?: string } | null;
  onPendingRegexTestConsumed?: () => void;
  pendingJumpToId?: string | null;
  onPendingJumpToIdConsumed?: () => void;
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
  panelCollapsed,
  onTogglePanel,
  isProjectMode,
  pendingRegexTest,
  onPendingRegexTestConsumed,
  pendingJumpToId,
  onPendingJumpToIdConsumed,
}: AuditRulesYamlProps) {
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
  const { diff, changeCount } = useLineDiff(rawQuery.data?.raw ?? '', raw, !panelCollapsed);

  // ─── Derive cursor regex / exclude from editor state ───
  const cursorRegex = useMemo(() => {
    if (!fieldPath) return undefined;
    // Check if the cursor line itself contains `regex:`
    const lines = raw.split('\n');
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
  }, [fieldPath, cursorLine, raw]);

  const cursorExclude = useMemo(() => {
    if (!cursorRegex) return undefined;
    const lines = raw.split('\n');
    const idx = cursorLine - 1;
    return extractExcludeNearby(lines, idx) ?? undefined;
  }, [cursorRegex, cursorLine, raw]);

  // ─── Cross-view pending state ───
  const effectiveRegex = pendingRegexTest?.regex ?? cursorRegex;
  const effectiveExclude = pendingRegexTest?.exclude ?? cursorExclude;

  // Consume pendingRegexTest after it's been applied
  useEffect(() => {
    if (pendingRegexTest) {
      onPendingRegexTestConsumed?.();
    }
  }, [pendingRegexTest]); // eslint-disable-line react-hooks/exhaustive-deps

  // Consume pendingJumpToId: scroll editor to matching `id:` line
  useEffect(() => {
    if (pendingJumpToId && editorRef.current) {
      const doc = editorRef.current.state.doc.toString();
      const lines = doc.split('\n');
      const idx = lines.findIndex(l => l.trim().startsWith(`id: ${pendingJumpToId}`));
      if (idx >= 0) {
        const lineInfo = editorRef.current.state.doc.line(idx + 1);
        editorRef.current.dispatch({ selection: { anchor: lineInfo.from }, scrollIntoView: true });
        editorRef.current.focus();
      }
      onPendingJumpToIdConsumed?.();
    }
  }, [pendingJumpToId]); // eslint-disable-line react-hooks/exhaustive-deps

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
      toast('Audit rules saved successfully.', 'success');
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
      toast('Audit rules file created.', 'success');
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
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">Failed to load audit rules</p>
        <p className="text-pencil-light text-sm mt-1">{rawQuery.error.message}</p>
      </Card>
    );
  }

  // ─── File doesn't exist: show create UI with panel ───
  if (rawQuery.data && !rawQuery.data.exists) {
    return (
      <div className="flex gap-4">
        <div className="flex-[3] min-w-0 transition-[flex] duration-300 ease-in-out">
          <EmptyState
            icon={FilePlus}
            title="No custom rules file"
            description={`Create ${isProjectMode ? 'a project-level' : 'a global'} audit-rules.yaml to add or override security rules`}
            action={
              <Button variant="primary" onClick={handleCreate} disabled={creating}>
                <FilePlus size={16} strokeWidth={2.5} />
                {creating ? 'Creating...' : 'Create Rules File'}
              </Button>
            }
          />
        </div>

        <div
          className={`hidden lg:block transition-all duration-300 ease-in-out overflow-hidden ${
            panelCollapsed ? 'flex-[0] w-0 opacity-0 pointer-events-none' : 'flex-[2] opacity-100'
          }`}
        >
          <Card className="h-full !p-0 !overflow-visible min-w-[280px]">
            <AuditAssistantPanel
              mode="yaml"
              errors={[]}
              changeCount={0}
              fieldPath={null}
              cursorLine={1}
              source=""
              diff={{ lines: [], changeCount: 0 }}
              editorRef={editorRef}
              collapsed={panelCollapsed}
              onToggleCollapse={onTogglePanel}
              onRevert={handleRevert}
            />
          </Card>
        </div>
      </div>
    );
  }

  // ─── Editor view ───
  return (
    <div className="flex gap-4">
      <Card className="flex-[3] min-w-0 transition-[flex] duration-300 ease-in-out">
        {/* Header: file path + save + collapse toggle */}
        <div className="flex items-center gap-2 mb-3">
          <FileCode size={16} strokeWidth={2.5} className="text-blue" />
          <span className="text-base text-pencil-light">
            {rawQuery.data!.path}
          </span>
          <span className="flex-1" />
          {dirty && (
            <span className="text-sm text-warning px-2 py-1 bg-warning-light rounded-full border border-warning">
              unsaved
            </span>
          )}
          <Button onClick={handleSave} disabled={saving || !dirty} variant="primary" size="sm">
            <Save size={16} strokeWidth={2.5} />
            {saving ? 'Saving...' : 'Save'}
          </Button>
          {panelCollapsed && (
            <IconButton
              icon={<PanelRightOpen size={14} strokeWidth={2} />}
              label="Expand assistant panel"
              size="sm"
              variant="ghost"
              onClick={onTogglePanel}
              className="hidden lg:inline-flex"
            />
          )}
        </div>
        <div className="min-w-0 -mx-4 -mb-4">
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
      </Card>

      {/* Assistant Panel */}
      <div
        className={`hidden lg:block transition-all duration-300 ease-in-out overflow-hidden ${
          panelCollapsed ? 'flex-[0] w-0 opacity-0 pointer-events-none' : 'flex-[2] opacity-100'
        }`}
      >
        <Card className="h-full !p-0 !overflow-visible min-w-[280px]">
          <AuditAssistantPanel
            mode="yaml"
            errors={errors}
            changeCount={changeCount}
            fieldPath={fieldPath}
            cursorLine={cursorLine}
            source={raw}
            diff={diff}
            editorRef={editorRef}
            collapsed={panelCollapsed}
            onToggleCollapse={onTogglePanel}
            onRevert={handleRevert}
            cursorRegex={effectiveRegex}
            cursorExclude={effectiveExclude}
          />
        </Card>
      </div>
    </div>
  );
}
