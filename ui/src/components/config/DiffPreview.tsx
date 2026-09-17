import { RotateCcw } from 'lucide-react';
import Button from '../Button';
import { useT } from '../../i18n';
import type { DiffResult } from '../../hooks/useLineDiff';

interface DiffPreviewProps {
  diff: DiffResult;
  onClickLine: (line: number) => void;
  onRevert: () => void;
}

export default function DiffPreview({ diff, onClickLine, onRevert }: DiffPreviewProps) {
  const t = useT();
  if (diff.changeCount === 0) return <p className="text-[13px] text-ink-3">{t('config.panel.noChanges')}</p>;

  return (
    <div className="flex flex-col gap-2.5">
      <div className="ss-list !shadow-none font-mono text-xs">
        {diff.lines.map((line, i) => {
          const added = line.type === 'add';
          const target = added ? line.newLine : line.oldLine;
          return (
            <button
              key={i}
              type="button"
              className="ss-r !min-h-[26px] !gap-2 !px-2.5 !py-0 text-left"
              onClick={() => target != null && onClickLine(target)}
            >
              <span className={`w-2 shrink-0 ${added ? 'text-ok' : 'text-bad'}`}>{added ? '+' : '−'}</span>
              <span className="w-7 shrink-0 text-right text-ink-3">{target ?? ''}</span>
              <span className={`min-w-0 flex-1 truncate ${added ? '' : 'text-ink-3 line-through'}`}>{line.content || ' '}</span>
            </button>
          );
        })}
      </div>
      <Button variant="ghost" size="sm" className="self-start" onClick={onRevert}>
        <RotateCcw size={14} />{t('config.revert')}
      </Button>
    </div>
  );
}
