import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Code2, LayoutGrid, Plus, X } from 'lucide-react';
import type { Frontmatter, FrontmatterValue } from '../../lib/frontmatter';
import { serializeFrontmatter } from '../../lib/frontmatter';
import { Input } from '../Input';
import EditorSegment from './controls/EditorSegment';
import SwitchToggle from './controls/SwitchToggle';
import CharBudget from './controls/CharBudget';
import { useT } from '../../i18n';

const DESC_BUDGET = 1536;

type FieldType = 'text' | 'multiline' | 'array' | 'enum' | 'bool';

interface FieldDef {
  key: string;
  label: string;
  hint: string;
  type: FieldType;
  required?: boolean;
  options?: string[];
  showWhen?: { key: string; value: string };
  placeholder?: string;
  arrayPlaceholder?: string;
  arrayItemLabel?: string;
  rows?: number;
}

interface GroupDef {
  id: 'identity' | 'invocation' | 'execution';
  label: string;
  defaultOpen: boolean;
  fields: FieldDef[];
}

function getGroups(t: ReturnType<typeof useT>): GroupDef[] {
  return [
    {
      id: 'identity',
      label: 'Identity',
      defaultOpen: true,
      fields: [
        {
          key: 'name',
          label: 'name',
          hint: t('frontmatterEditor.field.name.hint'),
          type: 'text',
          required: true,
        },
        {
          key: 'description',
          label: 'description',
          hint: t('frontmatterEditor.field.description.hint'),
          type: 'multiline',
          required: true,
          rows: 5,
        },
        {
          key: 'when_to_use',
          label: 'when_to_use',
          hint: t('frontmatterEditor.field.whenToUse.hint'),
          type: 'multiline',
          rows: 3,
        },
      ],
    },
    {
      id: 'invocation',
      label: 'Invocation',
      defaultOpen: true,
      fields: [
        {
          key: 'argument-hint',
          label: 'argument-hint',
          hint: t('frontmatterEditor.field.argumentHint.hint'),
          type: 'text',
        },
        {
          key: 'paths',
          label: 'paths',
          hint: t('frontmatterEditor.field.paths.hint'),
          type: 'array',
          arrayPlaceholder: 'src/**/*.ts',
          arrayItemLabel: 'path',
        },
        {
          key: 'disable-model-invocation',
          label: 'disable-model-invocation',
          hint: t('frontmatterEditor.field.disableModelInvocation.hint'),
          type: 'bool',
        },
        {
          key: 'user-invocable',
          label: 'user-invocable',
          hint: t('frontmatterEditor.field.userInvocable.hint'),
          type: 'bool',
        },
      ],
    },
    {
      id: 'execution',
      label: 'Execution',
      defaultOpen: false,
      fields: [
        {
          key: 'allowed-tools',
          label: 'allowed-tools',
          hint: t('frontmatterEditor.field.allowedTools.hint'),
          type: 'array',
          arrayPlaceholder: 'Tool(pattern:*)',
          arrayItemLabel: 'tool',
        },
        {
          key: 'context',
          label: 'context',
          hint: t('frontmatterEditor.field.context.hint'),
          type: 'enum',
          options: ['', 'fork'],
        },
        {
          key: 'agent',
          label: 'agent',
          hint: t('frontmatterEditor.field.agent.hint'),
          type: 'text',
          showWhen: { key: 'context', value: 'fork' },
          placeholder: 'Explore / Plan / general-purpose',
        },
        {
          key: 'shell',
          label: 'shell',
          hint: t('frontmatterEditor.field.shell.hint'),
          type: 'enum',
          options: ['', 'bash', 'powershell'],
        },
      ],
    },
  ];
}

const _STATIC_GROUPS_FOR_ORDER: GroupDef[] = [
  {
    id: 'identity',
    label: 'Identity',
    defaultOpen: true,
    fields: [
      { key: 'name', label: 'name', hint: '', type: 'text', required: true },
      { key: 'description', label: 'description', hint: '', type: 'multiline', required: true },
      { key: 'when_to_use', label: 'when_to_use', hint: '', type: 'multiline' },
    ],
  },
  {
    id: 'invocation',
    label: 'Invocation',
    defaultOpen: true,
    fields: [
      { key: 'argument-hint', label: 'argument-hint', hint: '', type: 'text' },
      { key: 'paths', label: 'paths', hint: '', type: 'array' },
      { key: 'disable-model-invocation', label: 'disable-model-invocation', hint: '', type: 'bool' },
      { key: 'user-invocable', label: 'user-invocable', hint: '', type: 'bool' },
    ],
  },
  {
    id: 'execution',
    label: 'Execution',
    defaultOpen: false,
    fields: [
      { key: 'allowed-tools', label: 'allowed-tools', hint: '', type: 'array' },
      { key: 'context', label: 'context', hint: '', type: 'enum' },
      { key: 'agent', label: 'agent', hint: '', type: 'text' },
      { key: 'shell', label: 'shell', hint: '', type: 'enum' },
    ],
  },
];

export const FM_FIELD_ORDER = _STATIC_GROUPS_FOR_ORDER.flatMap((g) => g.fields.map((f) => f.key));

type SetField = (key: string, value: string | string[] | boolean | null) => void;

interface FrontmatterEditorProps {
  frontmatter: Frontmatter;
  onChange: (next: Frontmatter) => void;
  yamlMode: boolean;
  onToggleYaml: (next: boolean) => void;
  metadataHint?: ReactNode;
}

export default function FrontmatterEditor({
  frontmatter,
  onChange,
  yamlMode,
  onToggleYaml,
  metadataHint,
}: FrontmatterEditorProps) {
  const t = useT();
  const groups = useMemo(() => getGroups(t), [t]);
  const yaml = useMemo(
    () => (yamlMode ? serializeFrontmatter(frontmatter, FM_FIELD_ORDER) : ''),
    [frontmatter, yamlMode],
  );

  const setField: SetField = (key, value) => {
    const next = { ...frontmatter };
    if (value == null || value === '' || (Array.isArray(value) && value.length === 0)) {
      delete next[key];
    } else {
      (next as Record<string, FrontmatterValue>)[key] = value;
    }
    onChange(next);
  };

  return (
    <div className="fm-block">
      <div className="fm-head">
        <div className="fm-title">
          <span className="fm-tick">---</span>
          <span>{t('frontmatterEditor.title')}</span>
          <span className="fm-sub">{t('frontmatterEditor.subtitle')}</span>
        </div>
        <EditorSegment<'fields' | 'yaml'>
          value={yamlMode ? 'yaml' : 'fields'}
          onChange={(v) => onToggleYaml(v === 'yaml')}
          options={[
            { value: 'fields', label: <><LayoutGrid size={12} /> {t('frontmatterEditor.viewFields')}</> },
            { value: 'yaml', label: <><Code2 size={12} /> {t('frontmatterEditor.viewYaml')}</> },
          ]}
        />
      </div>

      {!yamlMode ? (
        <div className="fm-groups">
          {groups.map((group) => (
            <FrontmatterGroup
              key={group.id}
              group={group}
              frontmatter={frontmatter}
              setField={setField}
            />
          ))}
          <FrontmatterMetadataGroup
            frontmatter={frontmatter}
            onChange={onChange}
            hint={metadataHint}
          />
        </div>
      ) : (
        <pre className="fm-yaml">{yaml}</pre>
      )}
    </div>
  );
}

function isFieldSet(key: string, fm: Frontmatter): boolean {
  const v = fm[key];
  if (v == null) return false;
  if (Array.isArray(v)) return v.length > 0;
  if (typeof v === 'string') return v.trim() !== '';
  if (typeof v === 'boolean') return v === true;
  return false;
}

function CollapsibleGroup({
  label,
  count,
  secondaryCount,
  defaultOpen,
  children,
  collapsedExtras,
}: {
  label: string;
  count: number;
  secondaryCount?: string;
  defaultOpen: boolean;
  children: (open: boolean) => ReactNode;
  collapsedExtras?: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <section className={`fm-group ${open ? 'open' : 'closed'}`}>
      <button type="button" className="fm-group-head" onClick={() => setOpen(!open)}>
        <span className="fm-group-caret">{open ? '▾' : '▸'}</span>
        <span className="fm-group-label">{label}</span>
        <span className="fm-group-count">
          {count}
          {!open && secondaryCount ? ` · ${secondaryCount}` : ''}
        </span>
      </button>
      {!open && collapsedExtras}
      {children(open)}
    </section>
  );
}

const GROUP_LABEL_KEYS: Record<GroupDef['id'], string> = {
  identity: 'frontmatterEditor.group.identity',
  invocation: 'frontmatterEditor.group.invocation',
  execution: 'frontmatterEditor.group.execution',
};

function FrontmatterGroup({
  group,
  frontmatter,
  setField,
}: {
  group: GroupDef;
  frontmatter: Frontmatter;
  setField: SetField;
}) {
  const t = useT();
  const isVisible = (f: FieldDef) =>
    !f.showWhen || frontmatter[f.showWhen.key] === f.showWhen.value;
  const visibleFields = group.fields.filter(isVisible);
  const pinned = !group.defaultOpen
    ? visibleFields.filter((f) => isFieldSet(f.key, frontmatter))
    : [];

  const groupLabel = t(GROUP_LABEL_KEYS[group.id]);

  return (
    <CollapsibleGroup
      label={groupLabel}
      count={visibleFields.length}
      secondaryCount={pinned.length > 0 ? `${pinned.length} set` : undefined}
      defaultOpen={group.defaultOpen}
      collapsedExtras={
        pinned.length > 0 ? (
          <div className="fm-grid fm-grid-pinned">
            {pinned.map((def) => (
              <FrontmatterField
                key={def.key}
                def={def}
                frontmatter={frontmatter}
                setField={setField}
              />
            ))}
          </div>
        ) : null
      }
    >
      {(open) =>
        open ? (
          <div className="fm-grid">
            {visibleFields.map((def) => (
              <FrontmatterField
                key={def.key}
                def={def}
                frontmatter={frontmatter}
                setField={setField}
              />
            ))}
          </div>
        ) : null
      }
    </CollapsibleGroup>
  );
}

function FrontmatterField({
  def,
  frontmatter,
  setField,
}: {
  def: FieldDef;
  frontmatter: Frontmatter;
  setField: SetField;
}) {
  const value = frontmatter[def.key];

  return (
    <div className="fm-row" key={def.key}>
      <label className="fm-label">
        <div className="fm-label-row">
          <span className="fm-key">{def.label}</span>
          {def.required && <span className="fm-req" title="Required">*</span>}
          {(def.key === 'description' || def.key === 'when_to_use') && (
            <CharBudget
              used={
                String(frontmatter['description'] ?? '').length +
                String(frontmatter['when_to_use'] ?? '').length
              }
              cap={DESC_BUDGET}
            />
          )}
        </div>
        <span className="fm-hint">{def.hint}</span>
      </label>
      <div className="fm-val">
        <FieldControl def={def} value={value} setField={setField} />
      </div>
    </div>
  );
}

function FieldControl({
  def,
  value,
  setField,
}: {
  def: FieldDef;
  value: FrontmatterValue | undefined;
  setField: SetField;
}) {
  switch (def.type) {
    case 'enum':
      return <EnumField def={def} value={typeof value === 'string' ? value : ''} setField={setField} />;
    case 'bool':
      return <BoolField def={def} value={value === true} setField={setField} />;
    case 'array':
      return (
        <ArrayField
          def={def}
          value={Array.isArray(value) ? value.map((v) => String(v ?? '')) : []}
          setField={setField}
        />
      );
    case 'multiline':
      return (
        <TextField
          def={def}
          value={typeof value === 'string' ? value : ''}
          setField={setField}
          multiline
        />
      );
    default:
      return <TextField def={def} value={typeof value === 'string' ? value : ''} setField={setField} />;
  }
}

function EnumField({ def, value, setField }: { def: FieldDef; value: string; setField: SetField }) {
  return (
    <EditorSegment<string>
      value={value}
      onChange={(next) => setField(def.key, next || null)}
      className="seg-group-field"
      role="radiogroup"
      options={(def.options ?? []).map((o) => ({ value: o, label: o || 'inherit' }))}
    />
  );
}

function BoolField({ def, value, setField }: { def: FieldDef; value: boolean; setField: SetField }) {
  return (
    <SwitchToggle
      checked={value}
      onChange={(next) => setField(def.key, next ? true : null)}
      label={value ? 'enabled' : 'disabled'}
    />
  );
}

function ArrayField({
  def,
  value,
  setField,
}: {
  def: FieldDef;
  value: string[];
  setField: SetField;
}) {
  return (
    <div className="tool-chips">
      {value.map((item, i) => (
        <span className="chip" key={i}>
          <input
            className="chip-input"
            value={item}
            placeholder={def.arrayPlaceholder ?? ''}
            onChange={(e) => {
              const next = [...value];
              next[i] = e.target.value;
              setField(def.key, next);
            }}
          />
          <button
            type="button"
            className="chip-x"
            onClick={() => {
              const next = value.filter((_, idx) => idx !== i);
              setField(def.key, next.length ? next : null);
            }}
            aria-label="Remove"
          >
            <X size={10} strokeWidth={2.4} />
          </button>
        </span>
      ))}
      <button
        type="button"
        className="chip add"
        onClick={() => setField(def.key, [...value, ''])}
      >
        <Plus size={12} strokeWidth={2.2} /> {def.arrayItemLabel ?? 'item'}
      </button>
    </div>
  );
}

function TextField({
  def,
  value,
  setField,
  multiline = false,
}: {
  def: FieldDef;
  value: string;
  setField: SetField;
  multiline?: boolean;
}) {
  const placeholder = def.placeholder ?? `set ${def.label}…`;
  if (multiline) {
    return (
      <textarea
        className="fm-input"
        rows={def.rows ?? 2}
        value={value}
        onChange={(e) => setField(def.key, e.target.value)}
        placeholder={placeholder}
      />
    );
  }
  return (
    <input
      type="text"
      className="fm-input"
      value={value}
      onChange={(e) => setField(def.key, e.target.value)}
      placeholder={placeholder}
    />
  );
}

interface MetadataRow {
  id: string;
  key: string;
}

const LIST_VALUED_KEYS = new Set<string>(['targets']);

function readMetadata(frontmatter: Frontmatter): Record<string, FrontmatterValue> {
  const raw = frontmatter.metadata;
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
    return raw as Record<string, FrontmatterValue>;
  }
  return {};
}

function writeMetadata(
  frontmatter: Frontmatter,
  nextMeta: Record<string, FrontmatterValue>,
): Frontmatter {
  const next = { ...frontmatter };
  if (Object.keys(nextMeta).length === 0) {
    delete next.metadata;
  } else {
    next.metadata = nextMeta as Frontmatter[string];
  }
  return next;
}

function FrontmatterMetadataGroup({
  frontmatter,
  onChange,
  hint,
}: {
  frontmatter: Frontmatter;
  onChange: (next: Frontmatter) => void;
  hint?: ReactNode;
}) {
  const t = useT();
  const metadata = readMetadata(frontmatter);
  const metaKeys = Object.keys(metadata);
  const rowIdRef = useRef(0);
  const nextRowId = () => `r:${rowIdRef.current++}`;
  const [rows, setRows] = useState<MetadataRow[]>(() =>
    metaKeys.map((k) => ({ id: nextRowId(), key: k })),
  );

  useEffect(() => {
    setRows((prev) => {
      const seen = new Set<string>();
      const kept: MetadataRow[] = [];
      for (const r of prev) {
        if (r.key === '') {
          kept.push(r);
          continue;
        }
        if (metaKeys.includes(r.key) && !seen.has(r.key)) {
          kept.push(r);
          seen.add(r.key);
        }
      }
      for (const k of metaKeys) {
        if (!seen.has(k)) {
          kept.push({ id: nextRowId(), key: k });
        }
      }
      if (
        kept.length === prev.length &&
        kept.every((r, i) => r.id === prev[i].id && r.key === prev[i].key)
      ) {
        return prev;
      }
      return kept;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [metaKeys.join('|')]);

  const commitKey = (rowId: string, oldKey: string, newKey: string) => {
    if (oldKey === newKey) return;
    if (oldKey === '' && newKey === '') return;
    const meta = readMetadata(frontmatter);
    if (oldKey && !newKey) {
      if (oldKey in meta) {
        const nextMeta = { ...meta };
        delete nextMeta[oldKey];
        onChange(writeMetadata(frontmatter, nextMeta));
      }
      setRows((arr) => arr.filter((r) => r.id !== rowId));
      return;
    }
    if (!oldKey && newKey) {
      if (newKey in meta) return;
      onChange(writeMetadata(frontmatter, { ...meta, [newKey]: '' }));
      setRows((arr) => arr.map((r) => (r.id === rowId ? { ...r, key: newKey } : r)));
      return;
    }
    if (oldKey && newKey) {
      if (newKey in meta) return;
      const nextMeta: Record<string, FrontmatterValue> = {};
      for (const k of Object.keys(meta)) {
        if (k === oldKey) nextMeta[newKey] = meta[k];
        else nextMeta[k] = meta[k];
      }
      onChange(writeMetadata(frontmatter, nextMeta));
      setRows((arr) => arr.map((r) => (r.id === rowId ? { ...r, key: newKey } : r)));
    }
  };

  const setValue = (key: string, value: string) => {
    if (!key) return;
    const meta = readMetadata(frontmatter);
    const normalized = LIST_VALUED_KEYS.has(key)
      ? value
          .split(/[,\n]/)
          .map((s) => s.trim())
          .filter(Boolean)
      : value;
    onChange(writeMetadata(frontmatter, { ...meta, [key]: normalized }));
  };

  const removeRow = (rowId: string, key: string) => {
    const meta = readMetadata(frontmatter);
    if (key && key in meta) {
      const nextMeta = { ...meta };
      delete nextMeta[key];
      onChange(writeMetadata(frontmatter, nextMeta));
    }
    setRows((arr) => arr.filter((r) => r.id !== rowId));
  };

  const addRow = () => {
    setRows((arr) => [...arr, { id: nextRowId(), key: '' }]);
  };

  const getValue = (key: string): string => {
    if (!key) return '';
    const v = readMetadata(frontmatter)[key];
    if (v == null) return '';
    if (Array.isArray(v)) return v.join(', ');
    return String(v);
  };

  return (
    <CollapsibleGroup label={t('frontmatterEditor.group.metadata')} count={metaKeys.length} defaultOpen>
      {(open) =>
        open ? (
          <div className="fm-grid fm-grid-custom">
            {rows.length === 0 && (
              <p className="fm-custom-empty">
                {t('frontmatterEditor.emptyMetadata', { metadataKey: 'metadata:', targetsKey: 'targets' })}
              </p>
            )}
            {rows.map((row) => (
              <MetadataRowEditor
                key={row.id}
                row={row}
                value={getValue(row.key)}
                onCommitKey={(newKey) => commitKey(row.id, row.key, newKey)}
                onChangeValue={(v) => setValue(row.key, v)}
                onRemove={() => removeRow(row.id, row.key)}
              />
            ))}
            <button type="button" className="chip add" onClick={addRow}>
              <Plus size={12} strokeWidth={2.2} /> {t('frontmatterEditor.addField')}
            </button>
            {hint && <div className="fm-metadata-extras">{hint}</div>}
          </div>
        ) : null
      }
    </CollapsibleGroup>
  );
}

function MetadataRowEditor({
  row,
  value,
  onCommitKey,
  onChangeValue,
  onRemove,
}: {
  row: MetadataRow;
  value: string;
  onCommitKey: (newKey: string) => void;
  onChangeValue: (v: string) => void;
  onRemove: () => void;
}) {
  const t = useT();
  const isList = LIST_VALUED_KEYS.has(row.key);
  return (
    <div className="fm-custom-row">
      <Input
        className="mono fm-custom-key"
        placeholder="key"
        defaultValue={row.key}
        onBlur={(e) => onCommitKey(e.target.value.trim())}
      />
      <Input
        className="mono fm-custom-value"
        placeholder={isList ? 'claude, cursor' : 'value'}
        defaultValue={value}
        disabled={!row.key}
        onChange={(e) => onChangeValue(e.target.value)}
      />
      <button
        type="button"
        className="chip-x"
        onClick={onRemove}
        aria-label={t('frontmatterEditor.removeField')}
        title={t('frontmatterEditor.removeField')}
      >
        <X size={11} strokeWidth={2.4} />
      </button>
    </div>
  );
}
