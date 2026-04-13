import Card from './Card';
import Badge from './Badge';
import type { ManagedPreview } from '../api/client';

interface CompiledPreviewCardProps {
  preview: ManagedPreview;
}

export default function CompiledPreviewCard({ preview }: CompiledPreviewCardProps) {
  const warnings = preview.warnings ?? [];

  return (
    <Card variant="outlined" className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <h4 className="text-lg text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
          Preview for {preview.target}
        </h4>
        <Badge variant="info">{preview.files.length} file{preview.files.length !== 1 ? 's' : ''}</Badge>
      </div>

      {warnings.length > 0 && (
        <div className="space-y-2">
          {warnings.map((warning) => (
            <p key={warning} className="text-sm text-warning">
              {warning}
            </p>
          ))}
        </div>
      )}

      <div className="space-y-3">
        {preview.files.map((file) => (
          <div key={file.path} className="space-y-2 rounded-md border border-dashed border-muted-dark p-3">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="accent">{file.format}</Badge>
              <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                {file.path}
              </p>
            </div>
            <pre className="overflow-x-auto whitespace-pre-wrap break-words rounded-md bg-surface px-3 py-2 text-sm text-pencil">
              {file.content}
            </pre>
          </div>
        ))}
      </div>
    </Card>
  );
}
