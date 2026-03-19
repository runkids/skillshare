import { AlertCircle, AlertTriangle } from 'lucide-react';
import type { ValidationError } from '../../hooks/useYamlValidation';
import Badge from '../Badge';

interface ErrorListProps {
  errors: ValidationError[];
  onClickError: (line: number) => void;
}

export default function ErrorList({ errors, onClickError }: ErrorListProps) {
  if (errors.length === 0) {
    return (
      <div className="px-3 py-4 text-sm text-pencil-light text-center">
        No errors or warnings.
      </div>
    );
  }

  const sorted = [...errors].sort((a, b) => a.line - b.line);
  const errorItems = sorted.filter(e => e.severity === 'error');
  const warningItems = sorted.filter(e => e.severity === 'warning');

  return (
    <div role="log" aria-label="Validation errors" className="flex flex-col gap-2 p-2">
      {errorItems.length > 0 && (
        <section className="flex flex-col gap-1">
          <div className="px-1 py-1 text-[10px] font-semibold text-danger uppercase tracking-wider flex items-center gap-1.5">
            <AlertCircle size={11} strokeWidth={2} />
            Errors
          </div>
          {errorItems.map((err, i) => (
            <ErrorItem key={`err-${i}`} error={err} onClick={onClickError} />
          ))}
        </section>
      )}
      {warningItems.length > 0 && (
        <section className="flex flex-col gap-1">
          <div className="px-1 py-1 text-[10px] font-semibold text-warning uppercase tracking-wider flex items-center gap-1.5">
            <AlertTriangle size={11} strokeWidth={2} />
            Warnings
          </div>
          {warningItems.map((warn, i) => (
            <ErrorItem key={`warn-${i}`} error={warn} onClick={onClickError} />
          ))}
        </section>
      )}
    </div>
  );
}

function ErrorItem({
  error,
  onClick,
}: {
  error: ValidationError;
  onClick: (line: number) => void;
}) {
  const isError = error.severity === 'error';
  return (
    <button
      type="button"
      className={`ss-error-item w-full text-left rounded-lg p-2 flex items-start gap-2 text-sm transition-all duration-150 cursor-pointer ${
        isError
          ? 'bg-danger/5 hover:bg-danger/10 text-danger'
          : 'bg-warning/5 hover:bg-warning/10 text-warning'
      }`}
      onClick={() => onClick(error.line)}
      aria-label={`${error.severity} on line ${error.line}: ${error.message}`}
    >
      <span className="flex-shrink-0 mt-0.5">
        {isError ? (
          <AlertCircle size={13} strokeWidth={2} />
        ) : (
          <AlertTriangle size={13} strokeWidth={2} />
        )}
      </span>
      <Badge variant="default" size="sm">
        L:{error.line}
      </Badge>
      <span className="flex-1 leading-snug break-words">{error.message}</span>
    </button>
  );
}
