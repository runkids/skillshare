import { useMemo, useState, useEffect } from 'react';
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
import CopyButton from './CopyButton';
import IconButton from './IconButton';
import Spinner from './Spinner';
import DialogShell from './DialogShell';
import { api, type SkillFileContent } from '../api/client';
import { handTheme } from '../lib/codemirror-theme';

interface FileViewerModalProps {
  skillName: string;
  filepath: string;
  sourcePath?: string;
  onClose: () => void;
}

export default function FileViewerModal({ skillName, filepath, sourcePath, onClose }: FileViewerModalProps) {
  const fullPath = sourcePath ? `${sourcePath}/${filepath}` : filepath;
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

  const cmExtensions = useMemo(() => {
    if (!data) return [];
    const exts = [EditorView.lineWrapping, EditorView.editable.of(false), ...handTheme];
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
    <DialogShell open={true} onClose={onClose} maxWidth="3xl" className="max-h-[85vh] flex flex-col">
      <Card className="flex flex-col h-full overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between mb-3 pt-2">
            <h3
              className="font-bold text-pencil truncate font-mono flex items-center gap-1.5"
              style={{ fontSize: '0.95rem' }}
            >
              {filepath}
              <CopyButton
                value={fullPath}
                title="Copy file path"
                copiedLabelClassName="text-xs font-normal"
              />
            </h3>
            <IconButton
              icon={<X size={16} strokeWidth={2.5} />}
              label="Close"
              size="md"
              onClick={onClose}
              className="shrink-0 ml-2"
            />
          </div>

          {/* Content */}
          <div className="overflow-auto flex-1 min-h-0 -mx-4 -mb-4 px-4 pb-4">
            {loading && (
              <div className="py-12 flex justify-center">
                <Spinner size="md" />
              </div>
            )}

            {error && (
              <div className="py-8 text-center">
                <p className="text-danger">
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
                    theme="none"
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
    </DialogShell>
  );
}
