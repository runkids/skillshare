import Button from './Button';
import { Input, Textarea } from './Input';
import SkillFrontmatterGuide from './SkillFrontmatterGuide';
import type { FrontmatterSchema } from '../lib/skillFrontmatter';

export type FrontmatterWorkspaceEntry = {
  id: string;
  key: string;
  value: string;
  isBoolean: boolean;
  isStructured: boolean;
  isCustom: boolean;
  error: string | null;
};

type SkillFrontmatterWorkspaceProps = {
  mode: 'read' | 'edit';
  schema?: FrontmatterSchema;
  builtInEntries: FrontmatterWorkspaceEntry[];
  additionalEntries: FrontmatterWorkspaceEntry[];
  referenceExcludeKeys?: string[];
  onAddBuiltInField: (key: string) => void;
  onAddCustomField: () => void;
  onRemoveField: (id: string) => void;
  onChangeFieldValue: (id: string, value: string) => void;
  onChangeFieldKey: (id: string, nextKey: string) => void;
  onBlurFieldValue?: (id: string) => void;
  onBlurFieldKey?: (id: string) => void;
};

function parseBooleanValue(value: string): 'true' | 'false' | null {
  const normalized = value.trim().toLowerCase();
  if (normalized === 'true') return 'true';
  if (normalized === 'false') return 'false';
  return null;
}

function shouldUseTextarea(entry: FrontmatterWorkspaceEntry): boolean {
  return entry.isStructured || entry.value.includes('\n');
}

function buildGuideFrontmatter(entries: FrontmatterWorkspaceEntry[]) {
  return Object.fromEntries(entries.map((entry) => [entry.key, entry.value]));
}

function FieldValueEditor({
  entry,
  mode,
  onChangeFieldValue,
  onBlurFieldValue,
}: {
  entry: FrontmatterWorkspaceEntry;
  mode: 'read' | 'edit';
  onChangeFieldValue: (id: string, value: string) => void;
  onBlurFieldValue?: (id: string) => void;
}) {
  if (entry.isBoolean) {
    const value = parseBooleanValue(entry.value);

    return (
      <div className="inline-flex rounded-[var(--radius-sm)] border border-muted bg-paper/70 p-1">
        {(['true', 'false'] as const).map((option) => {
          const isActive = value === option;
          return (
            <button
              key={option}
              type="button"
              aria-pressed={isActive}
              disabled={mode !== 'edit'}
              onClick={() => onChangeFieldValue(entry.id, option)}
              className={`min-w-16 rounded-[var(--radius-sm)] px-3 py-1.5 text-sm font-medium transition ${
                mode === 'edit' ? 'cursor-pointer' : 'cursor-default'
              } ${
                isActive
                  ? 'bg-surface text-pencil shadow-sm hover:bg-surface/90'
                  : 'text-pencil-light hover:bg-muted/40 hover:text-pencil'
              }`}
            >
              {option}
            </button>
          );
        })}
      </div>
    );
  }

  if (shouldUseTextarea(entry)) {
    return (
      <Textarea
        aria-label={`${entry.isCustom ? (entry.key || 'custom') : entry.key} frontmatter value`}
        value={entry.value}
        onChange={(event) => onChangeFieldValue(entry.id, event.target.value)}
        onBlur={() => onBlurFieldValue?.(entry.id)}
        rows={entry.isStructured ? 7 : 4}
        readOnly={mode !== 'edit'}
        className="min-h-28 border-muted/90 bg-paper/70"
      />
    );
  }

  return (
    <Input
      aria-label={`${entry.isCustom ? (entry.key || 'custom') : entry.key} frontmatter value`}
      value={entry.value}
      onChange={(event) => onChangeFieldValue(entry.id, event.target.value)}
      onBlur={() => onBlurFieldValue?.(entry.id)}
      readOnly={mode !== 'edit'}
      uiSize="sm"
      className="border-muted/90 bg-paper/70"
    />
  );
}

function ActiveFieldCard({
  entry,
  mode,
  onRemoveField,
  onChangeFieldValue,
  onChangeFieldKey,
  onBlurFieldValue,
  onBlurFieldKey,
}: {
  entry: FrontmatterWorkspaceEntry;
  mode: 'read' | 'edit';
  onRemoveField: (id: string) => void;
  onChangeFieldValue: (id: string, value: string) => void;
  onChangeFieldKey: (id: string, nextKey: string) => void;
  onBlurFieldValue?: (id: string) => void;
  onBlurFieldKey?: (id: string) => void;
}) {
  return (
    <article className="rounded-[var(--radius-md)] border border-muted-dark/40 bg-surface p-4 shadow-sm">
      <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-2">
          {mode === 'edit' ? (
            <Input
              aria-label={`${entry.key || 'custom'} frontmatter key`}
              value={entry.key}
              onChange={(event) => onChangeFieldKey(entry.id, event.target.value)}
              onBlur={() => onBlurFieldKey?.(entry.id)}
              uiSize="sm"
              className="font-mono text-sm"
            />
          ) : (
            <div className="space-y-1">
              <code className="text-sm font-semibold text-pencil">{entry.key}</code>
            </div>
          )}
        </div>

        {mode === 'edit' ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onRemoveField(entry.id)}
            aria-label={`Remove ${entry.key}`}
            className="shrink-0 border border-transparent text-pencil-light hover:border-danger/20 hover:bg-danger/10 hover:text-danger"
          >
            Remove
          </Button>
        ) : null}
      </div>

      <FieldValueEditor
        entry={entry}
        mode={mode}
        onChangeFieldValue={onChangeFieldValue}
        onBlurFieldValue={onBlurFieldValue}
      />

      {entry.error ? (
        <p className="mt-2 text-sm text-danger">{entry.error}</p>
      ) : null}
    </article>
  );
}

function CustomAddCard({ onAddCustomField, schema }: { onAddCustomField: () => void; schema: FrontmatterSchema }) {
  return (
    <article className="flex min-h-48 flex-col justify-between rounded-[var(--radius-md)] border border-dashed border-muted-dark/40 bg-paper/40 p-4">
      <div className="space-y-2">
        <div className="text-sm font-semibold text-pencil">Custom</div>
        <p className="text-sm leading-relaxed text-pencil-light">
          Add an extra frontmatter field for project-specific metadata that is not part of the built-in {schema === 'agent' ? 'subagent' : 'skill'} reference.
        </p>
      </div>

      <Button
        type="button"
        variant="secondary"
        size="sm"
        onClick={onAddCustomField}
        aria-label="Add custom frontmatter"
        className="self-start"
      >
        + Add custom frontmatter
      </Button>
    </article>
  );
}

export default function SkillFrontmatterWorkspace({
  mode,
  schema = 'skill',
  builtInEntries,
  additionalEntries,
  referenceExcludeKeys = ['name', 'description'],
  onAddBuiltInField,
  onAddCustomField,
  onRemoveField,
  onChangeFieldValue,
  onChangeFieldKey,
  onBlurFieldValue,
  onBlurFieldKey,
}: SkillFrontmatterWorkspaceProps) {
  const activeEntries = [...builtInEntries, ...additionalEntries];
  const activeBuiltInKeys = new Set(builtInEntries.map((entry) => entry.key));

  return (
    <section className="space-y-6">
      <div className="space-y-3">
        <div className="space-y-1">
          <h3 className="text-lg font-bold text-pencil">YAML Frontmatter</h3>
          <p className="max-w-2xl text-sm text-pencil-light">
            Edit the active YAML frontmatter fields here, then use the reference below to add built-in fields that are not in use yet.
          </p>
        </div>

        <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
          {activeEntries.map((entry) => (
            <ActiveFieldCard
              key={entry.id}
              entry={entry}
              mode={mode}
              onRemoveField={onRemoveField}
              onChangeFieldValue={onChangeFieldValue}
              onChangeFieldKey={onChangeFieldKey}
              onBlurFieldValue={onBlurFieldValue}
              onBlurFieldKey={onBlurFieldKey}
            />
          ))}

          {mode === 'edit' ? <CustomAddCard onAddCustomField={onAddCustomField} schema={schema} /> : null}
        </div>
      </div>

      <SkillFrontmatterGuide
        schema={schema}
        frontmatter={buildGuideFrontmatter(builtInEntries)}
        excludeKeys={referenceExcludeKeys}
        isReferenceEntryActive={(entry) => activeBuiltInKeys.has(entry.key)}
        renderReferenceAccessory={(entry) => (
          activeBuiltInKeys.has(entry.key) ? (
            <span className="rounded-full border border-success/20 bg-success/10 px-2 py-0.5 text-[11px] font-medium text-success">
              Used
            </span>
          ) : mode === 'edit' ? (
            <Button
              type="button"
              variant="ghost"
              size="xs"
              onClick={() => onAddBuiltInField(entry.key)}
              aria-label={`Add ${entry.key}`}
              className="border border-muted/80 bg-paper/70 text-pencil-light hover:border-pencil hover:bg-surface hover:text-pencil"
            >
              + Add
            </Button>
          ) : (
            <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${
              entry.required === 'Recommended'
                ? 'border border-blue/30 bg-blue/10 text-blue'
                : 'border border-muted-dark/30 bg-muted/40 text-pencil-light'
            }`}
            >
              {entry.required}
            </span>
          )
        )}
      />
    </section>
  );
}
