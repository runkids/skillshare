import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { Save, FileCode, Settings, EyeOff, RefreshCw, PanelRightOpen } from 'lucide-react';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { EditorView, keymap } from '@codemirror/view';
import { linter, lintGutter } from '@codemirror/lint';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { SkillignoreResponse } from '../api/client';
import type { ValidationError } from '../hooks/useYamlValidation';
import { useYamlValidation } from '../hooks/useYamlValidation';
import { useLineDiff, computeSimpleChangeCount } from '../hooks/useLineDiff';
import { useCursorField } from '../hooks/useCursorField';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import SegmentedControl from '../components/SegmentedControl';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import AssistantPanel from '../components/config/AssistantPanel';
import IconButton from '../components/IconButton';
import ConfirmDialog from '../components/ConfirmDialog';
import { api } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { handTheme } from '../lib/codemirror-theme';
import SyncPreviewModal from '../components/SyncPreviewModal';

type ConfigTab = 'config' | 'skillignore';

export default function ConfigPage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { isProjectMode } = useAppContext();
  const [tab, setTab] = useState<ConfigTab>('config');
  const [showSyncBanner, setShowSyncBanner] = useState(false);
  const [showSyncPreview, setShowSyncPreview] = useState(false);
  const editorRef = useRef<EditorView | null>(null);
  const [panelCollapsed, setPanelCollapsed] = useState(() => {
    try { return localStorage.getItem('config-panel-collapsed') === 'true'; }
    catch { return false; }
  });
  const [showDiscardDialog, setShowDiscardDialog] = useState(false);
  const [pendingTab, setPendingTab] = useState<ConfigTab | null>(null);
  const [showRevertDialog, setShowRevertDialog] = useState(false);

  // --- config.yaml state ---
  const { data: configData, isPending: configPending, error: configError } = useQuery({
    queryKey: queryKeys.config,
    queryFn: () => api.getConfig(),
    staleTime: staleTimes.config,
  });
  const [raw, setRaw] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (configData?.raw) {
      setRaw(configData.raw);
      setDirty(false);
    }
  }, [configData]);

  const handleConfigChange = (value: string) => {
    setRaw(value);
    const changed = value !== (configData?.raw ?? '');
    setDirty(changed);
    if (changed) setShowSyncBanner(false);
  };

  const handleConfigSave = async () => {
    setSaving(true);
    try {
      const res = await api.putConfig(raw);
      if (res.warnings?.length) {
        toast(`Config saved with warnings: ${res.warnings.join('; ')}`, 'warning');
      } else {
        toast('Config saved successfully.', 'success');
      }
      setShowSyncBanner(true);
      setDirty(false);
      // Invalidate all data that depends on config
      queryClient.invalidateQueries({ queryKey: queryKeys.config });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.extras });
      queryClient.invalidateQueries({ queryKey: queryKeys.extrasDiff() });
      queryClient.invalidateQueries({ queryKey: queryKeys.diff() });
      queryClient.invalidateQueries({ queryKey: queryKeys.syncMatrix() });
      queryClient.invalidateQueries({ queryKey: queryKeys.doctor });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  // Assistant panel hooks
  const { errors: yamlErrors } = useYamlValidation(raw);
  const { fieldPath, cursorLine, extension: cursorExtension } = useCursorField();
  const { diff, changeCount } = useLineDiff(configData?.raw ?? '', raw, !panelCollapsed);

  // Linter reads errors from ref to stay stable
  const errorsRef = useRef<ValidationError[]>([]);
  errorsRef.current = yamlErrors;

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

  // Save handler reads from ref — updated per tab so Cmd+S works in both editors
  const saveRef = useRef<() => void>(() => {});

  const saveKeymap = useMemo(
    () =>
      keymap.of([{
        key: 'Mod-s',
        run: () => { saveRef.current(); return true; },
      }]),
    [],
  );

  const yamlExtensions = useMemo(
    () => [yaml(), EditorView.lineWrapping, ...handTheme, lintGutter(), linterExtension, cursorExtension, saveKeymap],
    [linterExtension, cursorExtension, saveKeymap],
  );

  // --- .skillignore state ---
  const { data: ignoreData, isPending: ignorePending, error: ignoreError } = useQuery({
    queryKey: queryKeys.skillignore,
    queryFn: () => api.getSkillignore(),
    staleTime: staleTimes.skillignore,
    enabled: tab === 'skillignore',
  });
  const [ignoreRaw, setIgnoreRaw] = useState('');
  const [ignoreDirty, setIgnoreDirty] = useState(false);
  const [ignoreSaving, setIgnoreSaving] = useState(false);

  const ignoreExtensions = useMemo(() => [EditorView.lineWrapping, ...handTheme, saveKeymap], [saveKeymap]);

  const ignoreChangeCount = useMemo(
    () => computeSimpleChangeCount(ignoreData?.raw ?? '', ignoreRaw),
    [ignoreRaw, ignoreData],
  );

  useEffect(() => {
    if (ignoreData) {
      setIgnoreRaw(ignoreData.raw ?? '');
      setIgnoreDirty(false);
    }
  }, [ignoreData]);

  const handleIgnoreChange = (value: string) => {
    setIgnoreRaw(value);
    const changed = value !== (ignoreData?.raw ?? '');
    setIgnoreDirty(changed);
    if (changed) setShowSyncBanner(false);
  };

  const handleIgnoreSave = async () => {
    setIgnoreSaving(true);
    try {
      await api.putSkillignore(ignoreRaw);
      toast('.skillignore saved successfully.', 'success');
      setIgnoreDirty(false);
      queryClient.invalidateQueries({ queryKey: queryKeys.skillignore });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.doctor });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setIgnoreSaving(false);
    }
  };

  // --- active tab dirty/saving state ---
  const activeDirty = tab === 'config' ? dirty : ignoreDirty;
  const activeSaving = tab === 'config' ? saving : ignoreSaving;
  const handleSave = tab === 'config' ? handleConfigSave : handleIgnoreSave;
  saveRef.current = handleSave;

  // --- panel toggle + Cmd+B ---
  const togglePanel = useCallback(() => {
    setPanelCollapsed(prev => {
      const next = !prev;
      try { localStorage.setItem('config-panel-collapsed', String(next)); }
      catch { /* ignore */ }
      return next;
    });
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'b') {
        e.preventDefault();
        togglePanel();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [togglePanel]);

  // --- dirty state guard for tab switch ---
  const handleTabChange = (newTab: ConfigTab) => {
    if (activeDirty) {
      setPendingTab(newTab);
      setShowDiscardDialog(true);
    } else {
      setTab(newTab);
    }
  };

  const handleDiscard = () => {
    if (pendingTab) {
      if (tab === 'config') { setRaw(configData?.raw ?? ''); setDirty(false); }
      else { setIgnoreRaw(ignoreData?.raw ?? ''); setIgnoreDirty(false); }
      setTab(pendingTab);
    }
    setShowDiscardDialog(false);
    setPendingTab(null);
  };

  const handleRevert = () => {
    setRaw(configData?.raw ?? '');
    setDirty(false);
    setShowRevertDialog(false);
  };

  // --- loading / error for active tab ---
  const isPending = tab === 'config' ? configPending : ignorePending;
  const error = tab === 'config' ? configError : ignoreError;

  if (isPending) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">
          Failed to load {tab === 'config' ? 'config' : '.skillignore'}
        </p>
        <p className="text-pencil-light text-sm mt-1">{error.message}</p>
      </Card>
    );
  }

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <PageHeader
        icon={<Settings size={24} strokeWidth={2.5} />}
        title="Config"
        subtitle={isProjectMode ? 'Edit your project configuration' : 'Edit your skillshare configuration'}
        actions={
          <>
            {activeDirty && (
              <span
                className="text-sm text-warning px-2 py-1 bg-warning-light rounded-full border border-warning"
              >
                unsaved changes
              </span>
            )}
            <Button
              onClick={handleSave}
              disabled={activeSaving || !activeDirty}
              variant="primary"
              size="sm"
            >
              <Save size={16} strokeWidth={2.5} />
              {activeSaving ? 'Saving...' : 'Save'}
            </Button>
          </>
        }
      />

      <div className="mb-4">
        <SegmentedControl
          value={tab}
          onChange={handleTabChange}
          options={[
            { value: 'config' as ConfigTab, label: 'config.yaml' },
            { value: 'skillignore' as ConfigTab, label: '.skillignore' },
          ]}
        />
      </div>

      {showSyncBanner && (
        <Card className="mb-4 animate-fade-in">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <RefreshCw size={18} strokeWidth={2.5} className="text-blue shrink-0" />
              <span className="text-pencil">
                Config updated — preview what sync will do?
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setShowSyncBanner(false)}
              >
                Dismiss
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={() => {
                  setShowSyncPreview(true);
                  setShowSyncBanner(false);
                }}
              >
                Preview Sync
              </Button>
            </div>
          </div>
        </Card>
      )}

      {tab === 'config' && (
        <div className="flex gap-4">
          <Card className="flex-[3] min-w-0 transition-[flex] duration-300 ease-in-out">
            <div className="flex items-center gap-2 mb-3">
              <FileCode size={16} strokeWidth={2.5} className="text-blue" />
              <span className="text-base text-pencil-light">
                {isProjectMode ? '.skillshare/config.yaml' : 'config.yaml'}
              </span>
              <span className="flex-1" />
              {panelCollapsed && (
                <IconButton
                  icon={<PanelRightOpen size={14} strokeWidth={2} />}
                  label="Expand assistant panel"
                  size="sm"
                  variant="ghost"
                  onClick={togglePanel}
                  className="hidden lg:inline-flex"
                />
              )}
            </div>
            <div className="min-w-0 -mx-4 -mb-4">
              <CodeMirror
                value={raw}
                onChange={handleConfigChange}
                extensions={yamlExtensions}
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

          {/* Assistant panel */}
          <div
            className={`hidden lg:block transition-all duration-300 ease-in-out overflow-hidden ${
              panelCollapsed ? 'flex-[0] w-0 opacity-0 pointer-events-none' : 'flex-[2] opacity-100'
            }`}
          >
            <Card className="!p-0 !overflow-visible min-w-[280px]">
              <AssistantPanel
                errors={yamlErrors}
                changeCount={changeCount}
                fieldPath={fieldPath}
                cursorLine={cursorLine}
                source={raw}
                diff={diff}
                editorRef={editorRef}
                collapsed={panelCollapsed}
                onToggleCollapse={togglePanel}
                onRevert={() => setShowRevertDialog(true)}
              />
            </Card>
          </div>

        </div>
      )}

      {tab === 'skillignore' && (
        <div className="flex gap-4">
          <div className="flex-[3] min-w-0 transition-[flex] duration-300 ease-in-out">
            <SkillignoreTab
              data={ignoreData!}
              raw={ignoreRaw}
              onChange={handleIgnoreChange}
              extensions={ignoreExtensions}
              panelCollapsed={panelCollapsed}
              onTogglePanel={togglePanel}
            />
          </div>

          <div
            className={`hidden lg:block transition-all duration-300 ease-in-out overflow-hidden ${
              panelCollapsed ? 'flex-[0] w-0 opacity-0 pointer-events-none' : 'flex-[2] opacity-100'
            }`}
          >
            <Card className="!p-0 !overflow-visible min-w-[280px]">
              <AssistantPanel
                mode="skillignore"
                errors={[]}
                changeCount={ignoreChangeCount}
                fieldPath={null}
                cursorLine={1}
                source={ignoreRaw}
                diff={{ lines: [], changeCount: 0 }}
                editorRef={editorRef}
                collapsed={panelCollapsed}
                onToggleCollapse={togglePanel}
                onRevert={() => {}}
                ignoredSkills={ignoreData?.stats?.ignored_skills ?? []}
              />
            </Card>
          </div>

        </div>
      )}

      <SyncPreviewModal
        open={showSyncPreview}
        onClose={() => setShowSyncPreview(false)}
      />

      <ConfirmDialog
        open={showDiscardDialog}
        onConfirm={handleDiscard}
        onCancel={() => setShowDiscardDialog(false)}
        title="Unsaved Changes"
        message="You have unsaved changes that will be lost. Discard them?"
        confirmText="Discard"
        variant="danger"
      />

      <ConfirmDialog
        open={showRevertDialog}
        onConfirm={handleRevert}
        onCancel={() => setShowRevertDialog(false)}
        title="Revert Changes"
        message="Reset editor to the last saved version? This cannot be undone."
        confirmText="Revert"
        variant="danger"
      />
    </div>
  );
}

function SkillignoreTab({
  data,
  raw,
  onChange,
  extensions,
  panelCollapsed,
  onTogglePanel,
}: {
  data: SkillignoreResponse;
  raw: string;
  onChange: (value: string) => void;
  extensions: any[];
  panelCollapsed?: boolean;
  onTogglePanel?: () => void;
}) {
  const stats = data.stats;

  return (
    <div className="space-y-4">
      <Card>
        <div className="flex items-center gap-2 mb-3">
          <EyeOff size={16} strokeWidth={2.5} className="text-pencil-light" />
          <span className="text-base text-pencil-light">
            {data.path}
          </span>
          {stats && stats.ignored_count > 0 && (
            <span className="text-xs text-pencil-light px-2 py-0.5 bg-muted rounded-full border border-muted-dark">
              {stats.ignored_count} skill{stats.ignored_count !== 1 ? 's' : ''} ignored
            </span>
          )}
          <span className="flex-1" />
          {panelCollapsed && onTogglePanel && (
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

        {!data.exists && (
          <p className="text-sm text-pencil-light mb-3">
            Create a .skillignore file to hide skills from discovery. Uses gitignore syntax.
          </p>
        )}

        <div className="min-w-0 -mx-4 -mb-4">
          <CodeMirror
            value={raw}
            onChange={onChange}
            extensions={extensions}
            theme="none"
            height="500px"
            basicSetup={{
              lineNumbers: true,
              foldGutter: false,
              highlightActiveLine: true,
              highlightSelectionMatches: true,
              bracketMatching: false,
              indentOnInput: false,
              autocompletion: false,
            }}
          />
        </div>
      </Card>

    </div>
  );
}
