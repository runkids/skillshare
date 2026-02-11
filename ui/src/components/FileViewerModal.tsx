import { useEffect, useMemo, useState } from 'react';
import { X } from 'lucide-react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import CodeMirror from '@uiw/react-codemirror';
import { json } from '@codemirror/lang-json';
import { yaml } from '@codemirror/lang-yaml';
import { python } from '@codemirror/lang-python';
import { javascript } from '@codemirror/lang-javascript';
import { EditorView } from '@codemirror/view';
import Card from './Card';
import HandButton from './HandButton';
import { api, type SkillFileContent } from '../api/client';
import { handTheme } from '../lib/codemirror-theme';
import { wobbly } from '../design';

interface FileViewerModalProps {
  skillName: string;
  filepath: string;
  onClose: () => void;
}

export default function FileViewerModal({ skillName, filepath, onClose }: FileViewerModalProps) {
  const [data, setData] = useState<SkillFileContent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    api
      .getSkillFile(skillName, filepath)
      .then(setData)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [skillName, filepath]);

  // Close on Escape
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [onClose]);

  const cmExtensions = useMemo(() => {
    if (!data) return [];
    const exts = [EditorView.lineWrapping, EditorView.editable.of(false)];
    if (data.contentType === 'application/json') exts.push(json());
    else if (data.contentType === 'text/yaml') exts.push(yaml());
    // Infer language from filename extension
    const ext = filepath.split('.').pop()?.toLowerCase();
    if (ext === 'py') exts.push(python());
    else if (ext === 'js' || ext === 'mjs' || ext === 'cjs') exts.push(javascript());
    else if (ext === 'ts' || ext === 'mts' || ext === 'cts') exts.push(javascript({ typescript: true }));
    else if (ext === 'jsx') exts.push(javascript({ jsx: true }));
    else if (ext === 'tsx') exts.push(javascript({ jsx: true, typescript: true }));
    return exts;
  }, [data, filepath]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-pencil/30" />

      {/* Modal */}
      <div
        className="relative w-full max-w-3xl max-h-[85vh] flex flex-col"
        style={{ borderRadius: wobbly.md }}
      >
        <Card decoration="tape" className="flex flex-col h-full overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between mb-3 pt-2">
            <h3
              className="font-bold text-pencil truncate"
              style={{ fontFamily: "'Courier New', monospace", fontSize: '0.95rem' }}
            >
              {filepath}
            </h3>
            <HandButton variant="ghost" size="sm" onClick={onClose} className="shrink-0 ml-2">
              <X size={16} strokeWidth={2.5} />
            </HandButton>
          </div>

          {/* Content */}
          <div className="overflow-auto flex-1 min-h-0 -mx-4 -mb-4 px-4 pb-4">
            {loading && (
              <div className="py-12 text-center">
                <p
                  className="text-pencil-light animate-pulse"
                  style={{ fontFamily: 'var(--font-hand)' }}
                >
                  Loading...
                </p>
              </div>
            )}

            {error && (
              <div className="py-8 text-center">
                <p className="text-danger" style={{ fontFamily: 'var(--font-hand)' }}>
                  {error}
                </p>
              </div>
            )}

            {data && !loading && (
              <>
                {data.contentType === 'text/markdown' ? (
                  <div className="prose-hand">
                    <Markdown remarkPlugins={[remarkGfm]}>{data.content}</Markdown>
                  </div>
                ) : (
                  <CodeMirror
                    value={data.content}
                    extensions={cmExtensions}
                    theme={handTheme}
                    readOnly
                    editable={false}
                    basicSetup={{
                      lineNumbers: true,
                      foldGutter: true,
                      highlightActiveLine: false,
                      bracketMatching: true,
                      autocompletion: false,
                    }}
                  />
                )}
              </>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}
