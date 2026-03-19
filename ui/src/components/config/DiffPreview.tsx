import { RotateCcw, GitCompare } from 'lucide-react';
import Button from '../Button';
import Badge from '../Badge';
import type { DiffResult } from '../../hooks/useLineDiff';

interface DiffPreviewProps {
  diff: DiffResult;
  onClickLine: (line: number) => void;
  onRevert: () => void;
}

export default function DiffPreview({ diff, onClickLine, onRevert }: DiffPreviewProps) {
  if (diff.changeCount === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-10 px-4 text-center text-pencil-light animate-fade-in">
        <GitCompare size={32} strokeWidth={1.2} className="opacity-25" />
        <p className="text-sm">No changes since last save</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-1.5 border-b border-muted/40">
        <span className="text-sm font-medium text-pencil flex items-center gap-2">
          Changes
          <Badge variant="info">
            {diff.changeCount} line{diff.changeCount !== 1 ? 's' : ''}
          </Badge>
        </span>
        <Button
          variant="ghost"
          size="sm"
          onClick={onRevert}
          className="text-xs gap-1 py-0.5 px-1.5 h-auto"
          aria-label="Revert all changes"
        >
          <RotateCcw size={12} strokeWidth={2} />
          Revert All
        </Button>
      </div>

      {/* Diff lines */}
      <div className="font-mono text-xs overflow-x-auto">
        {diff.lines.map((line, i) => {
          const isAdd = line.type === 'add';
          const targetLine = isAdd ? line.newLine : line.oldLine;
          return (
            <button
              key={i}
              type="button"
              className={`ss-diff-line w-full text-left flex items-start gap-2 px-2 py-0.5 transition-all duration-150 cursor-pointer ${
                isAdd
                  ? 'bg-success/8 hover:bg-success/15 border-l-2 border-success'
                  : 'bg-danger/8 hover:bg-danger/15 border-l-2 border-danger'
              }`}
              onClick={() => targetLine != null && onClickLine(targetLine)}
              aria-label={`${isAdd ? 'Added' : 'Removed'} line ${targetLine ?? ''}: ${line.content}`}
            >
              <span className="flex-shrink-0 w-4 select-none">
                <Badge variant={isAdd ? 'success' : 'danger'} size="sm">
                  {isAdd ? '+' : '\u2212'}
                </Badge>
              </span>
              <span className="flex-shrink-0 w-10 text-right text-muted-dark">
                {targetLine != null ? `${targetLine}` : ''}
              </span>
              <span
                className={`flex-1 whitespace-pre break-all leading-relaxed ${
                  isAdd ? '' : 'line-through opacity-75'
                }`}
              >
                {line.content || ' '}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
