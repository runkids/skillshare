import { useState, useEffect, useMemo } from 'react';
import { Save, FileCode, Settings, EyeOff } from 'lucide-react';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { EditorView } from '@codemirror/view';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { SkillignoreResponse } from '../api/client';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import SegmentedControl from '../components/SegmentedControl';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { handTheme } from '../lib/codemirror-theme';

type ConfigTab = 'config' | 'skillignore';

export default function ConfigPage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { isProjectMode } = useAppContext();
  const [tab, setTab] = useState<ConfigTab>('config');

  // --- config.yaml state ---
  const { data: configData, isPending: configPending, error: configError } = useQuery({
    queryKey: queryKeys.config,
    queryFn: () => api.getConfig(),
    staleTime: staleTimes.config,
  });
  const [raw, setRaw] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);

  const yamlExtensions = useMemo(() => [yaml(), EditorView.lineWrapping, ...handTheme], []);

  useEffect(() => {
    if (configData?.raw) {
      setRaw(configData.raw);
      setDirty(false);
    }
  }, [configData]);

  const handleConfigChange = (value: string) => {
    setRaw(value);
    setDirty(value !== (configData?.raw ?? ''));
  };

  const handleConfigSave = async () => {
    setSaving(true);
    try {
      await api.putConfig(raw);
      toast('Config saved successfully.', 'success');
      setDirty(false);
      queryClient.invalidateQueries({ queryKey: queryKeys.config });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

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

  const ignoreExtensions = useMemo(() => [EditorView.lineWrapping, ...handTheme], []);

  useEffect(() => {
    if (ignoreData) {
      setIgnoreRaw(ignoreData.raw ?? '');
      setIgnoreDirty(false);
    }
  }, [ignoreData]);

  const handleIgnoreChange = (value: string) => {
    setIgnoreRaw(value);
    setIgnoreDirty(value !== (ignoreData?.raw ?? ''));
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
          onChange={setTab}
          options={[
            { value: 'config' as ConfigTab, label: 'config.yaml' },
            { value: 'skillignore' as ConfigTab, label: '.skillignore' },
          ]}
        />
      </div>

      {tab === 'config' && (
        <Card>
          <div className="flex items-center gap-2 mb-3">
            <FileCode size={16} strokeWidth={2.5} className="text-blue" />
            <span className="text-base text-pencil-light">
              {isProjectMode ? '.skillshare/config.yaml' : 'config.yaml'}
            </span>
          </div>
          <div className="min-w-0 -mx-4 -mb-4">
            <CodeMirror
              value={raw}
              onChange={handleConfigChange}
              extensions={yamlExtensions}
              theme="none"
              height="500px"
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
      )}

      {tab === 'skillignore' && (
        <SkillignoreTab
          data={ignoreData!}
          raw={ignoreRaw}
          onChange={handleIgnoreChange}
          extensions={ignoreExtensions}
        />
      )}
    </div>
  );
}

function SkillignoreTab({
  data,
  raw,
  onChange,
  extensions,
}: {
  data: SkillignoreResponse;
  raw: string;
  onChange: (value: string) => void;
  extensions: any[];
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

      {stats && stats.ignored_skills && stats.ignored_skills.length > 0 && (
        <Card>
          <div className="flex items-center gap-2 mb-3">
            <EyeOff size={16} strokeWidth={2.5} className="text-pencil-light" />
            <span className="text-base font-medium text-pencil">
              Ignored Skills ({stats.ignored_skills.length})
            </span>
          </div>
          <div className="flex flex-wrap gap-2">
            {stats.ignored_skills.map((name) => (
              <span
                key={name}
                className="font-mono text-xs text-pencil-light px-2 py-1 bg-muted/60 rounded border border-muted"
              >
                {name}
              </span>
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}
