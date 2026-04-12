import { AlignLeft, Files, Scale, Type, Zap } from 'lucide-react';
import Tooltip from './Tooltip';

type ContentStatsBarProps = {
  content: string;
  description?: string;
  body?: string;
  fileCount: number;
  license?: string;
  className?: string;
};

export default function ContentStatsBar({
  content,
  description,
  body,
  fileCount,
  license,
  className = '',
}: ContentStatsBarProps) {
  const trimmed = content.trim();
  const wordCount = trimmed ? trimmed.split(/\s+/).length : 0;
  const lineCount = trimmed ? trimmed.split(/\r?\n/).length : 0;
  const descTokens = description ? Math.round(description.length / 4) : 0;
  const bodyTokens = body ? Math.round(body.trim().length / 4) : 0;
  const totalTokens = descTokens + bodyTokens || Math.round(trimmed.length / 4);

  return (
    <div className={`ss-detail-stats flex flex-wrap items-center gap-4 border-b border-dashed border-pencil-light/30 py-3 text-sm text-pencil-light ${className}`.trim()}>
      <Tooltip
        content={`Description: ~${descTokens.toLocaleString()}\nBody: ~${bodyTokens.toLocaleString()}\nTotal: ~${totalTokens.toLocaleString()}\n(~4 chars/token estimate)`}
      >
        <span className="inline-flex items-center gap-1.5">
          <Zap size={12} strokeWidth={2.5} />
          ~{totalTokens.toLocaleString()} tokens
          {descTokens > 0 ? (
            <span className="text-pencil-light/60">
              (desc ~{descTokens.toLocaleString()} · body ~{bodyTokens.toLocaleString()})
            </span>
          ) : null}
        </span>
      </Tooltip>
      <span className="inline-flex items-center gap-1.5">
        <Type size={12} strokeWidth={2.5} />
        {wordCount.toLocaleString()} words
      </span>
      <span className="inline-flex items-center gap-1.5">
        <AlignLeft size={12} strokeWidth={2.5} />
        {lineCount.toLocaleString()} lines
      </span>
      <span className="inline-flex items-center gap-1.5">
        <Files size={12} strokeWidth={2.5} />
        {fileCount} file{fileCount !== 1 ? 's' : ''}
      </span>
      {license ? (
        <span className="inline-flex items-center gap-1.5">
          <Scale size={12} strokeWidth={2.5} />
          {license}
        </span>
      ) : null}
    </div>
  );
}
