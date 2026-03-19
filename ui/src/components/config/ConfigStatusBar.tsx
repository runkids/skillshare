import { PanelRightClose, PanelRightOpen } from 'lucide-react';
import type { ValidationError } from '../../hooks/useYamlValidation';
import Badge from '../Badge';
import IconButton from '../IconButton';

interface ConfigStatusBarProps {
  errors: ValidationError[];
  changeCount: number;
  collapsed: boolean;
  onToggleCollapse: () => void;
  onErrorsClick: () => void;
  schemaUnavailable?: boolean;
  mode?: 'config' | 'skillignore';
}

export default function ConfigStatusBar({
  errors,
  changeCount,
  collapsed,
  onToggleCollapse,
  onErrorsClick,
  schemaUnavailable = false,
  mode = 'config',
}: ConfigStatusBarProps) {
  const errorCount = errors.filter(e => e.severity === 'error').length;
  const warningCount = errors.filter(e => e.severity === 'warning').length;
  const totalIssues = errors.length;
  const isValid = totalIssues === 0;
  const isConfig = mode === 'config';

  return (
    <div className="ss-status-bar flex items-center gap-2 px-3 py-2 border-b border-muted/40 bg-paper text-sm select-none">
      {/* Validation state — only for config mode */}
      {isConfig && (
        <>
          {isValid ? (
            <Badge variant="success" dot>
              {schemaUnavailable ? 'Valid YAML (schema unavailable)' : 'Valid YAML'}
            </Badge>
          ) : (
            <button
              type="button"
              onClick={onErrorsClick}
              className="cursor-pointer transition-all duration-150 hover:opacity-80"
            >
              <Badge variant="danger" dot>
                {errorCount > 0 && warningCount > 0
                  ? `${errorCount} error${errorCount !== 1 ? 's' : ''}, ${warningCount} warning${warningCount !== 1 ? 's' : ''}`
                  : errorCount > 0
                    ? `${errorCount} error${errorCount !== 1 ? 's' : ''}`
                    : `${warningCount} warning${warningCount !== 1 ? 's' : ''}`}
              </Badge>
            </button>
          )}
          <span className="text-muted-dark" aria-hidden="true">·</span>
        </>
      )}

      {/* Change count */}
      <Badge variant={changeCount > 0 ? 'info' : 'default'}>
        {changeCount > 0 ? `${changeCount} change${changeCount !== 1 ? 's' : ''}` : 'No changes'}
      </Badge>

      {/* Spacer */}
      <span className="flex-1" />

      {/* Collapse toggle */}
      <IconButton
        icon={collapsed ? <PanelRightOpen size={14} strokeWidth={2} /> : <PanelRightClose size={14} strokeWidth={2} />}
        label={collapsed ? 'Expand assistant panel' : 'Collapse assistant panel'}
        size="sm"
        variant="ghost"
        onClick={onToggleCollapse}
      />
    </div>
  );
}
