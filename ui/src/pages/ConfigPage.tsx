import { useState, useEffect, useMemo } from 'react';
import { Save, FileCode } from 'lucide-react';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { EditorView } from '@codemirror/view';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { handTheme } from '../lib/codemirror-theme';

export default function ConfigPage() {
  const queryClient = useQueryClient();
  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.config,
    queryFn: () => api.getConfig(),
    staleTime: staleTimes.config,
  });
  const [raw, setRaw] = useState('');
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const { toast } = useToast();
  const { isProjectMode } = useAppContext();

  const extensions = useMemo(() => [yaml(), EditorView.lineWrapping, ...handTheme], []);

  useEffect(() => {
    if (data?.raw) {
      setRaw(data.raw);
      setDirty(false);
    }
  }, [data]);

  const handleChange = (value: string) => {
    setRaw(value);
    setDirty(value !== (data?.raw ?? ''));
  };

  const handleSave = async () => {
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

  if (isPending) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load config
        </p>
        <p className="text-pencil-light text-sm mt-1">{error.message}</p>
      </Card>
    );
  }

  return (
    <div className="animate-sketch-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2
            className="text-3xl md:text-4xl font-bold text-pencil mb-1"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Config
          </h2>
          <p className="text-pencil-light">
            {isProjectMode ? 'Edit your project configuration' : 'Edit your skillshare configuration'}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {dirty && (
            <span
              className="text-sm text-warning px-2 py-1 bg-warning-light rounded-full border border-warning"
              style={{ fontFamily: 'var(--font-hand)' }}
            >
              unsaved changes
            </span>
          )}
          <HandButton
            onClick={handleSave}
            disabled={saving || !dirty}
            variant="primary"
            size="md"
          >
            <Save size={16} strokeWidth={2.5} />
            {saving ? 'Saving...' : 'Save'}
          </HandButton>
        </div>
      </div>

      <Card decoration="tape">
        <div className="flex items-center gap-2 mb-3">
          <FileCode size={16} strokeWidth={2.5} className="text-blue" />
          <span
            className="text-base text-pencil-light"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            {isProjectMode ? '.skillshare/config.yaml' : 'config.yaml'}
          </span>
        </div>
        <div className="min-w-0 -mx-4 -mb-4">
          <CodeMirror
            value={raw}
            onChange={handleChange}
            extensions={extensions}
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
    </div>
  );
}
