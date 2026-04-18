import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Code2, LayoutGrid, Plus, X } from 'lucide-react';
import type { Frontmatter, FrontmatterValue } from '../../lib/frontmatter';
import { serializeFrontmatter } from '../../lib/frontmatter';
import { Input } from '../Input';
import SwitchToggle from './controls/SwitchToggle';
import SegmentedField from './controls/SegmentedField';
import CharBudget from './controls/CharBudget';

const DESC_BUDGET = 1536;

type FieldType =
  | 'text'           // single-line input
  | 'multiline'      // textarea
  | 'array'          // tag chips
  | 'enum'           // segmented (Task 5 will swap to SegmentedField)
  | 'bool'           // toggle (Task 5 will swap to SwitchToggle)
  | 'conditional';   // shown only when another field has a value

interface FieldDef {
  key: string;
  label: string;
  hint: string;
  type: FieldType;
  required?: boolean;
  options?: string[];
  showWhen?: { key: string; value: string };
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

const GROUPS: GroupDef[] = [
  {
    id: 'identity',
    label: 'Identity',
    defaultOpen: true,
    fields: [
      {
        key: 'name',
        label: 'name',
        hint: 'Skill identifier. Used by /<name> invocation.',
        type: 'text',
        required: true,
      },
      {
        key: 'description',
        label: 'description',
        hint: 'One-line summary — shown in skill lists and routing.',
        type: 'multiline',
        required: true,
        rows: 5,
      },
      {
        key: 'when_to_use',
        label: 'when_to_use',
        hint: 'Trigger phrases or example requests. Shares the 1,536-char budget with description.',
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
        hint: 'Placeholder shown during autocomplete. e.g. [issue-number]',
        type: 'text',
      },
      {
        key: 'paths',
        label: 'paths',
        hint: 'Glob patterns that limit when this skill auto-activates.',
        type: 'array',
        arrayPlaceholder: 'src/**/*.ts',
        arrayItemLabel: 'path',
      },
      {
        key: 'disable-model-invocation',
        label: 'disable-model-invocation',
        hint: "Only the user can trigger via /name. Claude won't auto-load.",
        type: 'bool',
      },
      {
        key: 'user-invocable',
        label: 'user-invocable',
        hint: 'Hide from / menu. Background knowledge only — Claude can still load it.',
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
        hint: 'Tools Claude can use without per-call approval while this skill is active.',
        type: 'array',
        arrayPlaceholder: 'Tool(pattern:*)',
        arrayItemLabel: 'tool',
      },
      {
        key: 'context',
        label: 'context',
        hint: 'Set "fork" to run in a forked subagent context.',
        type: 'enum',
        options: ['', 'fork'],
      },
      {
        key: 'agent',
        label: 'agent',
        hint: 'Subagent type. Only used when context=fork.',
        type: 'conditional',
        showWhen: { key: 'context', value: 'fork' },
      },
      {
        key: 'shell',
        label: 'shell',
        hint: 'Shell for !`...` and ```! blocks.',
        type: 'enum',
        options: ['', 'bash', 'powershell'],
      },
    ],
  },
];

export const FM_FIELD_ORDER = GROUPS.flatMap((g) => g.fields.map((f) => f.key));

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
  const yaml = useMemo(
    () => (yamlMode ? serializeFrontmatter(frontmatter, FM_FIELD_ORDER) : ''),
    [frontmatter, yamlMode],
  );

  const setField = (key: string, value: string | string[] | boolean | null) => {
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
          <span>Frontmatter</span>
          <span className="fm-sub">YAML metadata · drives routing &amp; tool access</span>
        </div>
        <div className="seg-group">
          <button
            type="button"
            className={`seg-btn ${!yamlMode ? 'active' : ''}`}
            onClick={() => onToggleYaml(false)}
          >
            <LayoutGrid size={12} /> Fields
          </button>
          <button
            type="button"
            className={`seg-btn ${yamlMode ? 'active' : ''}`}
            onClick={() => onToggleYaml(true)}
          >
            <Code2 size={12} /> YAML
          </button>
        </div>
      </div>

      {!yamlMode ? (
        <div className="fm-groups">
          {GROUPS.map((group) => (
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

function FrontmatterGroup({
  group,
  frontmatter,
  setField,
}: {
  group: GroupDef;
  frontmatter: Frontmatter;
  setField: (key: string, value: string | string[] | boolean | null) => void;
}) {
  const [open, setOpen] = useState(group.defaultOpen);
  const isConditionalVisible = (f: FieldDef) =>
    f.type !== 'conditional' || (f.showWhen != null && frontmatter[f.showWhen.key] === f.showWhen.value);
  const visibleFields = group.fields.filter(isConditionalVisible);
  const pinned = !group.defaultOpen
    ? visibleFields.filter((f) => isFieldSet(f.key, frontmatter))
    : [];

  return (
    <section className={`fm-group ${open ? 'open' : 'closed'}`}>
      <button type="button" className="fm-group-head" onClick={() => setOpen(!open)}>
        <span className="fm-group-caret">{open ? '▾' : '▸'}</span>
        <span className="fm-group-label">{group.label}</span>
        <span className="fm-group-count">
          {visibleFields.length}
          {!open && pinned.length > 0 ? ` · ${pinned.length} set` : ''}
        </span>
      </button>
      {!open && pinned.length > 0 && (
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
      )}
      {open && (
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
      )}
    </section>
  );
}

function FrontmatterField({
  def,
  frontmatter,
  setField,
}: {
  def: FieldDef;
  frontmatter: Frontmatter;
  setField: (key: string, value: string | string[] | boolean | null) => void;
}) {
  const value = frontmatter[def.key];
  const arr: string[] = Array.isArray(value) ? value.map((v) => String(v ?? '')) : [];
  const isArray = def.type === 'array';
  const isConditionalText = def.type === 'conditional';

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
        {def.type === 'enum' && def.options ? (
          <SegmentedField
            value={typeof value === 'string' ? value : ''}
            onChange={(next) => setField(def.key, next || null)}
            options={def.options.map((o) => ({
              value: o,
              label: o || 'inherit',
            }))}
          />
        ) : def.type === 'bool' ? (
          <SwitchToggle
            checked={value === true}
            onChange={(next) => setField(def.key, next ? true : null)}
            label={value === true ? 'enabled' : 'disabled'}
          />
        ) : isArray ? (
          <div className="tool-chips">
            {arr.map((item, i) => (
              <span className="chip" key={i}>
                <input
                  className="chip-input"
                  value={item}
                  placeholder={def.arrayPlaceholder ?? ''}
                  onChange={(e) => {
                    const next = [...arr];
                    next[i] = e.target.value;
                    setField(def.key, next);
                  }}
                />
                <button
                  type="button"
                  className="chip-x"
                  onClick={() => {
                    const next = arr.filter((_, idx) => idx !== i);
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
              onClick={() => setField(def.key, [...arr, ''])}
            >
              <Plus size={12} strokeWidth={2.2} /> {def.arrayItemLabel ?? 'item'}
            </button>
          </div>
        ) : def.type === 'multiline' ? (
          <textarea
            className="fm-input"
            rows={def.rows ?? 2}
            value={typeof value === 'string' ? value : ''}
            onChange={(e) => setField(def.key, e.target.value)}
            placeholder={`set ${def.label}…`}
          />
        ) : (
          <input
            type="text"
            className="fm-input"
            value={typeof value === 'string' ? value : ''}
            onChange={(e) => setField(def.key, e.target.value)}
            placeholder={
              isConditionalText
                ? 'Explore / Plan / general-purpose'
                : `set ${def.label}…`
            }
          />
        )}
      </div>
    </div>
  );
}

interface MetadataRow {
  id: string;
  key: string;
}

// Metadata keys that must serialize as YAML lists (not scalar strings).
// Backend parsers like ParseFrontmatterList only accept array values.
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
  const metadata = readMetadata(frontmatter);
  const metaKeys = Object.keys(metadata);
  const [open, setOpen] = useState(true);
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
    setOpen(true);
  };

  const getValue = (key: string): string => {
    if (!key) return '';
    const v = readMetadata(frontmatter)[key];
    if (v == null) return '';
    if (Array.isArray(v)) return v.join(', ');
    return String(v);
  };

  return (
    <section className={`fm-group ${open ? 'open' : 'closed'}`}>
      <button type="button" className="fm-group-head" onClick={() => setOpen(!open)}>
        <span className="fm-group-caret">{open ? '▾' : '▸'}</span>
        <span className="fm-group-label">Metadata</span>
        <span className="fm-group-count">{metaKeys.length}</span>
      </button>
      {open && (
        <div className="fm-grid fm-grid-custom">
          {rows.length === 0 && (
            <p className="fm-custom-empty">
              No metadata entries. Add keys under <code>metadata:</code> like{' '}
              <code>targets</code>.
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
            <Plus size={12} strokeWidth={2.2} /> field
          </button>
          {hint && <div className="fm-metadata-extras">{hint}</div>}
        </div>
      )}
    </section>
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
        aria-label="Remove"
        title="Remove field"
      >
        <X size={11} strokeWidth={2.4} />
      </button>
    </div>
  );
}
