import type { ReactNode } from 'react';
import Card from './Card';
import HandButton from './HandButton';
import { HandInput, HandTextarea } from './HandInput';

export interface RuleEditorValue {
  tool: string;
  relativePath: string;
  content: string;
}

interface RuleEditorProps {
  value: RuleEditorValue;
  onChange: (value: RuleEditorValue) => void;
  onSave: () => void;
  saving?: boolean;
  status?: string | null;
  submitLabel?: string;
  deleteAction?: ReactNode;
}

export default function RuleEditor({
  value,
  onChange,
  onSave,
  saving = false,
  status,
  submitLabel = 'Save Rule',
  deleteAction,
}: RuleEditorProps) {
  return (
    <Card className="space-y-4">
      <form
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          onSave();
        }}
      >
        <div className="grid gap-4 md:grid-cols-2">
          <HandInput
            label="Tool"
            value={value.tool}
            onChange={(event) => onChange({ ...value, tool: event.target.value })}
            placeholder="claude"
          />
          <HandInput
            label="Relative Path"
            value={value.relativePath}
            onChange={(event) => onChange({ ...value, relativePath: event.target.value })}
            placeholder="claude/backend.md"
          />
        </div>

        <HandTextarea
          label="Content"
          value={value.content}
          onChange={(event) => onChange({ ...value, content: event.target.value })}
          placeholder="# Rule content"
          rows={14}
        />

        {status && <p className="text-sm text-pencil-light">{status}</p>}

        <div className="flex flex-wrap gap-3">
          <HandButton type="submit" size="sm" disabled={saving}>
            {saving ? 'Working...' : submitLabel}
          </HandButton>
          {deleteAction}
        </div>
      </form>
    </Card>
  );
}
