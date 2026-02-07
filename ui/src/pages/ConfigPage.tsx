import { useState, useEffect, useMemo } from 'react';
import { Save, FileCode } from 'lucide-react';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { EditorView } from '@codemirror/view';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import { useApi } from '../hooks/useApi';
import { handTheme } from '../lib/codemirror-theme';

export default function ConfigPage() {
  const { data, loading, error, refetch } = useApi(() => api.getConfig());
  const [raw, setRaw] = useState('');
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const { toast } = useToast();

  const extensions = useMemo(() => [yaml(), EditorView.lineWrapping], []);

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
      refetch();
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load config
        </p>
        <p className="text-pencil-light text-sm mt-1">{error}</p>
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
          <p className="text-pencil-light">Edit your skillshare configuration</p>
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
            config.yaml
          </span>
        </div>
        <div className="min-w-0 -mx-4 -mb-4">
          <CodeMirror
            value={raw}
            onChange={handleChange}
            extensions={extensions}
            theme={handTheme}
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
