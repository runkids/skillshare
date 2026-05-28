import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { Save, FileCode, Settings, EyeOff, RefreshCw, PanelRightOpen, Puzzle, FolderOpen, Download } from 'lucide-react';
import { useT } from '../i18n';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { EditorView, keymap } from '@codemirror/view';
import { linter, lintGutter } from '@codemirror/lint';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { SkillignoreResponse, AgentignoreResponse } from '../api/client';
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

type ConfigTab = 'config' | 'skillignore' | 'agentignore' | 'extensions';

export default function ConfigPage() {
  const t = useT();
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
        toast(t('config.toast.savedWithWarnings', { warnings: res.warnings.join('; ') }), 'warning');
      } else {
        toast(t('config.toast.savedSuccess'), 'success');
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
      toast(t('config.skillignore.savedSuccess'), 'success');
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

  // --- .agentignore state ---
  const { data: agentIgnoreData, isPending: agentIgnorePending, error: agentIgnoreError } = useQuery({
    queryKey: queryKeys.agentignore,
    queryFn: () => api.getAgentignore(),
    staleTime: staleTimes.agentignore,
    enabled: tab === 'agentignore',
  });
  const [agentIgnoreRaw, setAgentIgnoreRaw] = useState('');
  const [agentIgnoreDirty, setAgentIgnoreDirty] = useState(false);
  const [agentIgnoreSaving, setAgentIgnoreSaving] = useState(false);

  const agentIgnoreChangeCount = useMemo(
    () => computeSimpleChangeCount(agentIgnoreData?.raw ?? '', agentIgnoreRaw),
    [agentIgnoreRaw, agentIgnoreData],
  );

  useEffect(() => {
    if (agentIgnoreData) {
      setAgentIgnoreRaw(agentIgnoreData.raw ?? '');
      setAgentIgnoreDirty(false);
    }
  }, [agentIgnoreData]);

  const handleAgentIgnoreChange = (value: string) => {
    setAgentIgnoreRaw(value);
    const changed = value !== (agentIgnoreData?.raw ?? '');
    setAgentIgnoreDirty(changed);
    if (changed) setShowSyncBanner(false);
  };

  const handleAgentIgnoreSave = async () => {
    setAgentIgnoreSaving(true);
    try {
      await api.putAgentignore(agentIgnoreRaw);
      toast(t('config.agentignore.savedSuccess'), 'success');
      setAgentIgnoreDirty(false);
      queryClient.invalidateQueries({ queryKey: queryKeys.agentignore });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.doctor });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setAgentIgnoreSaving(false);
    }
  };

  // --- active tab dirty/saving state ---
  const activeDirty = tab === 'config' ? dirty : tab === 'skillignore' ? ignoreDirty : tab === 'agentignore' ? agentIgnoreDirty : false;
  const activeSaving = tab === 'config' ? saving : tab === 'skillignore' ? ignoreSaving : tab === 'agentignore' ? agentIgnoreSaving : false;
  const handleSave = tab === 'config' ? handleConfigSave : tab === 'skillignore' ? handleIgnoreSave : tab === 'agentignore' ? handleAgentIgnoreSave : () => {};
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
      else if (tab === 'skillignore') { setIgnoreRaw(ignoreData?.raw ?? ''); setIgnoreDirty(false); }
      else { setAgentIgnoreRaw(agentIgnoreData?.raw ?? ''); setAgentIgnoreDirty(false); }
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
  const isPending = tab === 'config' ? configPending : tab === 'skillignore' ? ignorePending : tab === 'agentignore' ? agentIgnorePending : false;
  const error = tab === 'config' ? configError : tab === 'skillignore' ? ignoreError : tab === 'agentignore' ? agentIgnoreError : null;

  if (isPending) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">
          {t('config.errorLoading', { file: tab === 'config' ? 'config' : tab === 'skillignore' ? '.skillignore' : '.agentignore' })}
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
        title={t('config.title')}
        subtitle={isProjectMode ? t('config.subtitle.project') : t('config.subtitle.global')}
        actions={
          <>
            {activeDirty && (
              <span
                className="text-sm text-warning px-2 py-1 bg-warning-light rounded-full border border-warning"
              >
                {t('config.unsavedChanges')}
              </span>
            )}
            <Button
              onClick={handleSave}
              disabled={activeSaving || !activeDirty}
              variant="primary"
              size="sm"
            >
              <Save size={16} strokeWidth={2.5} />
              {activeSaving ? t('config.saving') : t('config.save')}
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
            { value: 'agentignore' as ConfigTab, label: '.agentignore' },
            { value: 'extensions' as ConfigTab, label: 'Extensions' },
          ]}
        />
      </div>

      {showSyncBanner && (
        <Card className="mb-4 animate-fade-in">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <RefreshCw size={18} strokeWidth={2.5} className="text-blue shrink-0" />
              <span className="text-pencil">
                {t('config.banner.message')}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setShowSyncBanner(false)}
              >
                {t('config.banner.dismiss')}
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={() => {
                  setShowSyncPreview(true);
                  setShowSyncBanner(false);
                }}
              >
                {t('config.banner.previewSync')}
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
                  label={t('config.expandAssistantPanel')}
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
            className={`hidden lg:block transition-all duration-300 ease-in-out ${
              panelCollapsed ? 'flex-[0] w-0 opacity-0 pointer-events-none overflow-hidden' : 'flex-[2] opacity-100 overflow-visible'
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
            <IgnoreTab
              kind="skill"
              data={ignoreData!}
              raw={ignoreRaw}
              onChange={handleIgnoreChange}
              extensions={ignoreExtensions}
              panelCollapsed={panelCollapsed}
              onTogglePanel={togglePanel}
            />
          </div>

          <div
            className={`hidden lg:block transition-all duration-300 ease-in-out ${
              panelCollapsed ? 'flex-[0] w-0 opacity-0 pointer-events-none overflow-hidden' : 'flex-[2] opacity-100 overflow-visible'
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

      {tab === 'agentignore' && (
        <div className="flex gap-4">
          <div className="flex-[3] min-w-0 transition-[flex] duration-300 ease-in-out">
            <IgnoreTab
              kind="agent"
              data={agentIgnoreData!}
              raw={agentIgnoreRaw}
              onChange={handleAgentIgnoreChange}
              extensions={ignoreExtensions}
              panelCollapsed={panelCollapsed}
              onTogglePanel={togglePanel}
            />
          </div>

          <div
            className={`hidden lg:block transition-all duration-300 ease-in-out ${
              panelCollapsed ? 'flex-[0] w-0 opacity-0 pointer-events-none overflow-hidden' : 'flex-[2] opacity-100 overflow-visible'
            }`}
          >
            <Card className="!p-0 !overflow-visible min-w-[280px]">
              <AssistantPanel
                mode="agentignore"
                errors={[]}
                changeCount={agentIgnoreChangeCount}
                fieldPath={null}
                cursorLine={1}
                source={agentIgnoreRaw}
                diff={{ lines: [], changeCount: 0 }}
                editorRef={editorRef}
                collapsed={panelCollapsed}
                onToggleCollapse={togglePanel}
                onRevert={() => {}}
                ignoredAgents={agentIgnoreData?.stats?.ignored_agents ?? []}
              />
            </Card>
          </div>

        </div>
      )}

      {tab === 'extensions' && <ExtensionsSection isProjectMode={isProjectMode} />}

      <SyncPreviewModal
        open={showSyncPreview}
        onClose={() => setShowSyncPreview(false)}
      />

      <ConfirmDialog
        open={showDiscardDialog}
        onConfirm={handleDiscard}
        onCancel={() => setShowDiscardDialog(false)}
        title={t('config.discard.title')}
        message={t('config.discard.message')}
        confirmText={t('config.discard.confirmText')}
        variant="danger"
      />

      <ConfirmDialog
        open={showRevertDialog}
        onConfirm={handleRevert}
        onCancel={() => setShowRevertDialog(false)}
        title={t('config.revert.title')}
        message={t('config.revert.message')}
        confirmText={t('config.revert.confirmText')}
        variant="danger"
      />
    </div>
  );
}

function IgnoreTab({
  kind,
  data,
  raw,
  onChange,
  extensions,
  panelCollapsed,
  onTogglePanel,
}: {
  kind: 'skill' | 'agent';
  data: SkillignoreResponse | AgentignoreResponse;
  raw: string;
  onChange: (value: string) => void;
  extensions: any[];
  panelCollapsed?: boolean;
  onTogglePanel?: () => void;
}) {
  const t = useT();
  const stats = data.stats;
  const fileName = kind === 'skill' ? '.skillignore' : '.agentignore';
  const itemLabel = kind === 'skill' ? 'skill' : 'agent';

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
              {t('config.ignore.ignoredCount', { count: stats.ignored_count, s: stats.ignored_count !== 1 ? 's' : '' })}
            </span>
          )}
          <span className="flex-1" />
          {panelCollapsed && onTogglePanel && (
            <IconButton
              icon={<PanelRightOpen size={14} strokeWidth={2} />}
              label={t('config.expandAssistantPanel')}
              size="sm"
              variant="ghost"
              onClick={onTogglePanel}
              className="hidden lg:inline-flex"
            />
          )}
        </div>

        {!data.exists && (
          <p className="text-sm text-pencil-light mb-3">
            {t('config.ignore.createHint', { fileName, itemLabel })}
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

// ─── ExtensionsSection ──────────────────────────────────────────────────────
// Manage transform extensions for the current mode: list installed ones,
// download bundled built-ins, and open the extensions directory in an editor.
function ExtensionsSection({ isProjectMode }: { isProjectMode: boolean }) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [installing, setInstalling] = useState<string | null>(null);
  const [opening, setOpening] = useState(false);

  const { data, isPending } = useQuery({
    queryKey: ['extensions'],
    queryFn: () => api.listExtensions(),
    staleTime: staleTimes.extras,
  });
  const extensions = data?.extensions ?? [];
  const installed = extensions.filter((e) => e.installed);
  const available = extensions.filter((e) => !e.installed);
  const dirLabel = isProjectMode ? '.skillshare/extensions' : '~/.config/skillshare/extensions';

  const handleInstall = async (name: string) => {
    setInstalling(name);
    try {
      await api.installExtension(name);
      toast(t('config.extensions.toast.installed', { name }, `Installed ${name}`), 'success');
      queryClient.invalidateQueries({ queryKey: ['extensions'] });
      queryClient.invalidateQueries({ queryKey: ['extras', 'extensions'] });
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setInstalling(null);
    }
  };

  const handleOpenDir = async () => {
    setOpening(true);
    try {
      const res = await api.openExtensionsDir();
      toast(t('config.extensions.toast.opened', { editor: res.editor }, `Opened in ${res.editor}`), 'success');
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setOpening(false);
    }
  };

  return (
    <Card>
      <div className="flex items-center justify-between gap-4 mb-3">
        <div className="flex items-center gap-2 min-w-0">
          <Puzzle size={16} strokeWidth={2.5} className="text-blue shrink-0" />
          <span className="font-bold text-pencil">Extensions</span>
          <span className="text-xs text-pencil-light font-mono truncate">({dirLabel})</span>
        </div>
        <Button variant="secondary" size="sm" onClick={handleOpenDir} loading={opening}>
          <FolderOpen size={14} strokeWidth={2.5} />
          {t('config.extensions.openDir', {}, 'Open directory')}
        </Button>
      </div>

      <p className="text-sm text-pencil-light mb-4">
        {t('config.extensions.description', {}, 'Transform extensions convert source files to a target format during sync (e.g. markdown agents to Codex TOML). Install one, then select it on an Extras target.')}
      </p>

      {isPending ? (
        <p className="text-sm text-pencil-light italic">{t('config.extensions.loading', {}, 'Loading...')}</p>
      ) : (
        <div className="space-y-4">
          <div>
            <div className="text-xs text-pencil-light uppercase tracking-wider mb-2">
              {t('config.extensions.installedHeading', {}, 'Installed')} ({installed.length})
            </div>
            {installed.length > 0 ? (
              <div className="space-y-1.5">
                {installed.map((e) => (
                  <div key={e.name} className="flex items-center gap-2">
                    <span className="font-mono text-sm text-pencil">{e.name}</span>
                    {e.builtin && (
                      <span className="text-[10px] uppercase tracking-wider text-blue border border-blue/40 rounded px-1 py-0.5">built-in</span>
                    )}
                    {e.description && <span className="text-sm text-pencil-light truncate">— {e.description}</span>}
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-pencil-light italic">{t('config.extensions.none', {}, 'No extensions installed yet.')}</p>
            )}
          </div>

          {available.length > 0 && (
            <div>
              <div className="text-xs text-pencil-light uppercase tracking-wider mb-2">
                {t('config.extensions.available', {}, 'Available built-ins')}
              </div>
              <div className="space-y-1.5">
                {available.map((e) => (
                  <div key={e.name} className="flex items-center gap-3">
                    <span className="font-mono text-sm text-pencil">{e.name}</span>
                    <span className="flex-1" />
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => handleInstall(e.name)}
                      loading={installing === e.name}
                    >
                      <Download size={14} strokeWidth={2.5} />
                      {t('config.extensions.download', {}, 'Download')}
                    </Button>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
