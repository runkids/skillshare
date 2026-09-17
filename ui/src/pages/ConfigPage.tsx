import { useState, useEffect, useMemo, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Save, FileCode, Info, RefreshCw, FileCog, FolderOpen, Download } from 'lucide-react';
import { useT } from '../i18n';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { EditorView, keymap } from '@codemirror/view';
import { linter, lintGutter } from '@codemirror/lint';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ExtensionInfo } from '../api/client';
import type { ValidationError } from '../hooks/useYamlValidation';
import { useYamlValidation } from '../hooks/useYamlValidation';
import { useLineDiff, computeSimpleChangeCount } from '../hooks/useLineDiff';
import { useCursorField } from '../hooks/useCursorField';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import AssistantPanel from '../components/config/AssistantPanel';
import ConfirmDialog from '../components/ConfirmDialog';
import { api } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { handTheme } from '../lib/codemirror-theme';
import SyncPreviewModal from '../components/SyncPreviewModal';
import { SettingsTabs } from './SettingsPage';
import { formatYaml } from '../lib/formatYaml';
import { shortenHome } from '../lib/paths';

type ConfigTab = 'config' | 'skillignore' | 'agentignore' | 'extensions';

const FILES: { value: ConfigTab; label: string }[] = [
  { value: 'config', label: 'config.yaml' },
  { value: 'skillignore', label: '.skillignore' },
  { value: 'agentignore', label: '.agentignore' },
];

export default function ConfigPage() {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { isProjectMode } = useAppContext();
  const [searchParams] = useSearchParams();
  // Deep link: /config?tab=extensions opens the Extensions tab directly, so
  // the Extras page can guide users here to install one.
  const [tab, setTab] = useState<ConfigTab>(() => {
    const requested = searchParams.get('tab');
    return requested === 'extensions' || requested === 'skillignore' || requested === 'agentignore'
      ? requested
      : 'config';
  });
  const overview = useQuery({ queryKey: queryKeys.overview, queryFn: () => api.getOverview(), staleTime: staleTimes.overview });
  const configDir = overview.data?.configDir;
  const urlTab = searchParams.get('tab');
  useEffect(() => {
    setTab(urlTab === 'extensions' || urlTab === 'skillignore' || urlTab === 'agentignore' ? urlTab : 'config');
  }, [urlTab]);
  const [showSyncBanner, setShowSyncBanner] = useState(false);
  const [showSyncPreview, setShowSyncPreview] = useState(false);
  const editorRef = useRef<EditorView | null>(null);
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
      const formatted = formatYaml(raw);
      const res = await api.putConfig(formatted);
      setRaw(formatted);
      if (res.warnings?.length) {
        toast(t('config.toast.savedWithWarnings', { warnings: res.warnings.join('; ') }), 'warning');
      } else {
        toast(t('config.toast.savedSuccess'), 'success');
      }
      setShowSyncBanner(true);
      setDirty(false);
      // Invalidate all data that depends on config
      queryClient.invalidateQueries({ queryKey: queryKeys.config });
      queryClient.invalidateQueries({ queryKey: queryKeys.mcp });
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
  const { diff, changeCount } = useLineDiff(configData?.raw ?? '', raw, true);

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
      queryClient.invalidateQueries({ queryKey: queryKeys.diff() });
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
      queryClient.invalidateQueries({ queryKey: queryKeys.diff() });
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
  const activeChangeCount = tab === 'config' ? changeCount : tab === 'skillignore' ? ignoreChangeCount : agentIgnoreChangeCount;
  const dirtyOf = (file: ConfigTab) => (file === 'config' ? dirty : file === 'skillignore' ? ignoreDirty : agentIgnoreDirty);

  // What the editor edits, and the line under it that says what the side panel is for.
  const ignoreHint = (data?: { exists: boolean }, fileName?: string, itemLabel?: string) =>
    data && !data.exists ? t('config.ignore.createHint', { fileName, itemLabel }) : t('config.panel.ignoreHint');
  const editor = tab === 'skillignore'
    ? { value: ignoreRaw, onChange: handleIgnoreChange, extensions: ignoreExtensions, hint: ignoreHint(ignoreData, '.skillignore', 'skill') }
    : tab === 'agentignore'
      ? { value: agentIgnoreRaw, onChange: handleAgentIgnoreChange, extensions: ignoreExtensions, hint: ignoreHint(agentIgnoreData, '.agentignore', 'agent') }
      : { value: raw, onChange: handleConfigChange, extensions: yamlExtensions, hint: t('config.saveShortcutHint') };

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
      <div className="ss-note bad">
        <span className="flex-1">
          {t('config.errorLoading', { file: tab === 'config' ? 'config' : tab === 'skillignore' ? '.skillignore' : '.agentignore' })} {error.message}
        </span>
      </div>
    );
  }

  return (
    <div className="ss-wrap animate-fade-in">
      <PageHeader
        className="!mb-0"
        title={t('layout.nav.settings')}
        subtitle={`${t(isProjectMode ? 'app.project' : 'app.global')}${configDir ? ` · ${shortenHome(configDir)}` : ''}`}
      />
      <SettingsTabs current={tab === 'extensions' ? 'extensions' : 'files'} />

      {tab === 'extensions' ? (
        <ExtensionsSection isProjectMode={isProjectMode} />
      ) : (
        <>
          <div className="ss-note inf"><Info size={16} /><span className="flex-1">{t('config.filesIntro')}</span></div>

          {showSyncBanner && (
            <div className="ss-note inf">
              <RefreshCw size={16} />
              <span className="flex-1">{t('config.banner.message')}</span>
              <Button variant="ghost" size="sm" onClick={() => setShowSyncBanner(false)}>{t('config.banner.dismiss')}</Button>
              <Button variant="secondary" size="sm" onClick={() => { setShowSyncPreview(true); setShowSyncBanner(false); }}>{t('config.banner.previewSync')}</Button>
            </div>
          )}

          <div className="grid grid-cols-[190px_minmax(0,1fr)_300px] items-start gap-6">
            <nav className="flex flex-col" aria-label={t('settings.tab.files')}>
              {FILES.map(({ value, label }) => (
                <button
                  key={value}
                  type="button"
                  className={`ss-nv ${tab === value ? 'on' : ''}`}
                  aria-current={tab === value ? 'page' : undefined}
                  onClick={() => handleTabChange(value)}
                >
                  <FileCode size={15} className="shrink-0" />
                  <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{label}</span>
                  {dirtyOf(value) && <span className="n" title={t('config.unsavedChanges')} aria-label={t('config.unsavedChanges')}>•</span>}
                </button>
              ))}
            </nav>

            <div className="flex min-w-0 flex-col gap-3">
              <div className="ss-code !overflow-hidden !p-0">
                <CodeMirror
                  key={tab}
                  value={editor.value}
                  onChange={editor.onChange}
                  extensions={editor.extensions}
                  theme="none"
                  height="500px"
                  onCreateEditor={(view) => { editorRef.current = view; }}
                  basicSetup={{
                    lineNumbers: true,
                    foldGutter: tab === 'config',
                    highlightActiveLine: true,
                    highlightSelectionMatches: true,
                    bracketMatching: tab === 'config',
                    indentOnInput: tab === 'config',
                    autocompletion: false,
                  }}
                />
              </div>
              <div className="flex items-center justify-between gap-4">
                <span className="text-[13px] text-ink-3">{editor.hint}</span>
                <span className="flex items-center gap-2.5">
                  <Button variant="ghost" size="sm" onClick={() => setShowRevertDialog(true)} disabled={!activeDirty || activeSaving}>{t('config.revert')}</Button>
                  <Button variant="primary" size="sm" onClick={handleSave} disabled={activeSaving || !activeDirty}>
                    <Save size={15} />{activeSaving ? t('config.saving') : t('config.save')}
                  </Button>
                </span>
              </div>
            </div>

            {/* Sticky: the file can be long, the panel should stay where the eye is */}
            <div className="sticky top-6">
              <AssistantPanel
                mode={tab}
                errors={tab === 'config' ? yamlErrors : []}
                changeCount={activeChangeCount}
                fieldPath={tab === 'config' ? fieldPath : null}
                cursorLine={cursorLine}
                source={editor.value}
                diff={tab === 'config' ? diff : { lines: [], changeCount: 0 }}
                editorRef={editorRef}
                onRevert={() => setShowRevertDialog(true)}
                ignoredSkills={ignoreData?.stats?.ignored_skills ?? []}
                ignoredAgents={agentIgnoreData?.stats?.ignored_agents ?? []}
              />
            </div>
          </div>
        </>
      )}

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

// ─── ExtensionsSection ──────────────────────────────────────────────────────
// Manage transform extensions for the current mode: list installed ones,
// download bundled built-ins, and open the extensions directory in an editor.
function ExtensionsSection({ isProjectMode }: { isProjectMode: boolean }) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  // Track every in-flight download by name so concurrent downloads each keep
  // their own spinner (a single string would let a later click clear an
  // earlier one's loading state).
  const [installing, setInstalling] = useState<Set<string>>(new Set());
  const [opening, setOpening] = useState(false);
  const [removing, setRemoving] = useState<string | null>(null);
  // The extension pending removal confirmation, with the extras referencing it.
  const [removeTarget, setRemoveTarget] = useState<{ name: string; usedBy: string[] } | null>(null);

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
    setInstalling((prev) => new Set(prev).add(name));
    try {
      await api.installExtension(name);
      toast(t('config.extensions.toast.installed', { name }, `Installed ${name}`), 'success');
      queryClient.invalidateQueries({ queryKey: ['extensions'] });
      queryClient.invalidateQueries({ queryKey: ['extras', 'extensions'] });
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setInstalling((prev) => {
        const next = new Set(prev);
        next.delete(name);
        return next;
      });
    }
  };

  const handleOpenDir = async () => {
    setOpening(true);
    try {
      await api.openExtensionsDir();
      toast(t('config.extensions.toast.opened', {}, 'Opened extensions directory'), 'success');
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setOpening(false);
    }
  };

  // Open the confirmation dialog; the dialog body warns when the extension is
  // still referenced by one or more extras.
  const handleRemove = (ext: ExtensionInfo) => {
    setRemoveTarget({ name: ext.name, usedBy: ext.used_by ?? [] });
  };

  const confirmRemove = async () => {
    if (!removeTarget) return;
    const { name } = removeTarget;
    setRemoving(name);
    try {
      await api.removeExtension(name);
      toast(t('config.extensions.toast.removed', { name }, `Removed ${name}`), 'success');
      queryClient.invalidateQueries({ queryKey: ['extensions'] });
      queryClient.invalidateQueries({ queryKey: ['extras', 'extensions'] });
      setRemoveTarget(null);
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setRemoving(null);
    }
  };

  const removeInUse = (removeTarget?.usedBy.length ?? 0) > 0;

  return (
    <>
      <div className="flex items-start justify-between gap-6">
        <p className="max-w-[720px] text-[13px] leading-relaxed text-ink-2">
          {t('config.extensions.description', {}, 'Extensions pass each file through a script during sync, so it arrives in the format a tool expects. Install one here, then select it on a target in Extras.')}
        </p>
        <Button variant="secondary" onClick={handleOpenDir} loading={opening}>
          <FolderOpen size={15} />
          {t('config.extensions.openDir', {}, 'Open directory')}
        </Button>
      </div>

      {isPending ? (
        <PageSkeleton />
      ) : (
        <div className="flex max-w-[860px] flex-col gap-8">
          <section>
            <div className="ss-sec">
              <h2>{t('config.extensions.installedHeading', {}, 'Installed')}</h2>
              <span className="ss-cnt">{installed.length}</span>
              <span className="ml-auto font-mono text-xs text-ink-3">{dirLabel}</span>
            </div>
            {installed.length > 0 ? (
              <div className="ss-list">
                {installed.map((e) => (
                  <ExtensionItem key={e.name} ext={e} onRemove={() => handleRemove(e)} removing={removing === e.name} />
                ))}
              </div>
            ) : (
              <p className="ss-empty">{t('config.extensions.none', {}, 'No extensions installed yet.')}</p>
            )}
          </section>

          {available.length > 0 && (
            <section>
              <div className="ss-sec">
                <h2>{t('config.extensions.available', {}, 'Available to download')}</h2>
                <span className="ss-cnt">{available.length}</span>
              </div>
              <div className="ss-list">
                {available.map((e) => (
                  <ExtensionItem key={e.name} ext={e} onInstall={() => handleInstall(e.name)} installing={installing.has(e.name)} />
                ))}
              </div>
            </section>
          )}
        </div>
      )}

    <ConfirmDialog
      open={removeTarget !== null}
      variant={removeInUse ? 'danger' : 'default'}
      title={
        removeInUse
          ? t('config.extensions.removeConfirm.inUseTitle', {}, 'Extension is in use')
          : t('config.extensions.removeConfirm.title', {}, 'Remove extension?')
      }
      message={
        removeInUse
          ? t(
              'config.extensions.removeConfirm.inUseMessage',
              { name: removeTarget?.name ?? '', extras: removeTarget?.usedBy.join(', ') ?? '' },
              `${removeTarget?.name} is used by: ${removeTarget?.usedBy.join(', ')}. Removing it will make those extras fail on next sync until you reinstall it or clear the extension from their targets. Remove anyway?`,
            )
          : t(
              'config.extensions.removeConfirm.message',
              { name: removeTarget?.name ?? '' },
              `Remove ${removeTarget?.name}? You can download it again later.`,
            )
      }
      confirmText={t('config.extensions.removeConfirm.confirm', {}, 'Remove')}
      loading={removing !== null}
      onConfirm={confirmRemove}
      onCancel={() => setRemoveTarget(null)}
    />
    </>
  );
}

// One extension row: name, what it does, and the action for its state.
function ExtensionItem({
  ext,
  onInstall,
  installing,
  onRemove,
  removing,
}: {
  ext: ExtensionInfo;
  onInstall?: () => void;
  installing?: boolean;
  onRemove?: () => void;
  removing?: boolean;
}) {
  const t = useT();
  return (
    <div className="ss-r">
      <FileCog size={16} className="shrink-0 text-ink-3" />
      <span className="flex min-w-0 flex-1 flex-col gap-px">
        <span className="nm m">{ext.name}</span>
        {ext.description && <span className="truncate text-[13px] text-ink-2">{ext.description}</span>}
      </span>
      {ext.builtin && <span className="ss-tag">built-in</span>}
      {ext.used_by && ext.used_by.length > 0 && (
        <span className="ss-tag">{t('config.extensions.usedByBadge', { count: ext.used_by.length }, `in use \u00b7 ${ext.used_by.length}`)}</span>
      )}
      {onInstall ? (
        <Button variant="secondary" size="sm" onClick={onInstall} loading={installing}>
          <Download size={14} />
          {t('config.extensions.download', {}, 'Download')}
        </Button>
      ) : (
        <Button variant="ghost" size="sm" onClick={onRemove} loading={removing}>
          {t('config.extensions.remove', {}, 'Remove')}
        </Button>
      )}
    </div>
  );
}
