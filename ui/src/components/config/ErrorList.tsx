import { AlertCircle, AlertTriangle } from 'lucide-react';
import type { ValidationError } from '../../hooks/useYamlValidation';

interface ErrorListProps {
  errors: ValidationError[];
  onClickError: (line: number) => void;
}

export default function ErrorList({ errors, onClickError }: ErrorListProps) {
  const sorted = [...errors].sort((a, b) => a.line - b.line);
  return (
    <div className="ss-list !shadow-none" role="log">
      {sorted.map((err, i) => {
        const isError = err.severity === 'error';
        const Icon = isError ? AlertCircle : AlertTriangle;
        return (
          <button
            key={i}
            type="button"
            className="ss-r !items-start !gap-2.5 !px-2.5 !py-1.5 text-left"
            onClick={() => onClickError(err.line)}
            aria-label={`${err.severity} on line ${err.line}: ${err.message}`}
          >
            <Icon size={14} className={`mt-0.5 shrink-0 ${isError ? 'text-bad' : 'text-warn'}`} />
            <span className="flex min-w-0 flex-1 flex-col gap-px">
              <span className="text-[13px] font-medium leading-snug">{err.message}</span>
              <span className="font-mono text-xs text-ink-3">L{err.line}</span>
            </span>
          </button>
        );
      })}
    </div>
  );
}
