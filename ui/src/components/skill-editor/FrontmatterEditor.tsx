import { useEffect, useState, type ReactNode } from 'react';
import { ChevronDown, Plus, X } from 'lucide-react';
import { serializeFrontmatter, type Frontmatter, type FrontmatterValue } from '../../lib/frontmatter';
import { useT } from '../../i18n';
import Button from '../Button';
import CodeView from '../CodeView';
import SegmentedControl from '../SegmentedControl';
import { SkillContextMenu } from '../TargetMenu';

export const DESC_BUDGET = 1536;

type FieldType = 'text' | 'multiline' | 'list' | 'bool' | 'enum';
type Group = 'identity' | 'invocation' | 'execution';

interface FieldDef {
  key: string;
  group: Group;
  type: FieldType;
  hint: string;
  options?: string[];
  showWhen?: { key: string; value: string };
}

const FIELDS: FieldDef[] = [
  { key: 'name', group: 'identity', type: 'text', hint: 'name' },
  { key: 'description', group: 'identity', type: 'multiline', hint: 'description' },
  { key: 'when_to_use', group: 'identity', type: 'multiline', hint: 'whenToUse' },
  { key: 'argument-hint', group: 'invocation', type: 'text', hint: 'argumentHint' },
  { key: 'paths', group: 'invocation', type: 'list', hint: 'paths' },
  { key: 'disable-model-invocation', group: 'invocation', type: 'bool', hint: 'disableModelInvocation' },
  { key: 'user-invocable', group: 'invocation', type: 'bool', hint: 'userInvocable' },
  { key: 'allowed-tools', group: 'execution', type: 'list', hint: 'allowedTools' },
  { key: 'context', group: 'execution', type: 'enum', options: ['', 'fork'], hint: 'context' },
  { key: 'agent', group: 'execution', type: 'text', hint: 'agent', showWhen: { key: 'context', value: 'fork' } },
  { key: 'shell', group: 'execution', type: 'enum', options: ['', 'bash', 'powershell'], hint: 'shell' },
];
const KNOWN = new Set(FIELDS.map((f) => f.key));
const GROUPS: Group[] = ['identity', 'invocation', 'execution'];
const LIST_METADATA = new Set(['targets']);

function readMetadata(fm: Frontmatter): Record<string, FrontmatterValue> {
  const raw = fm.metadata;
  return raw && typeof raw === 'object' && !Array.isArray(raw) ? (raw as Record<string, FrontmatterValue>) : {};
}

function withMetadata(fm: Frontmatter, meta: Record<string, FrontmatterValue>): Frontmatter {
  const next = { ...fm };
  if (Object.keys(meta).length === 0) delete next.metadata;
  else next.metadata = meta as Frontmatter[string];
  return next;
}

/** Splits on commas outside parentheses, so `Bash(git add:*, git commit:*)` stays one entry. */
function splitList(text: string): string[] {
  const out: string[] = [];
  let depth = 0;
  let cur = '';
  for (const ch of text) {
    if (ch === '(') depth++;
    if (ch === ')') depth = Math.max(0, depth - 1);
    if ((ch === ',' || ch === '\n') && depth === 0) {
      out.push(cur);
      cur = '';
    } else {
      cur += ch;
    }
  }
  out.push(cur);
  return out.map((s) => s.trim()).filter(Boolean);
}

const listText = (v: FrontmatterValue | undefined) => (Array.isArray(v) ? v.join(', ') : v == null ? '' : String(v));

interface Props {
  frontmatter: Frontmatter;
  onChange: (next: Frontmatter) => void;
  yaml: boolean;
  onYaml: (yaml: boolean) => void;
}

export default function FrontmatterEditor({ frontmatter, onChange, yaml, onYaml }: Props) {
  const t = useT();
  // Rows stay on screen while their value is cleared; only the remove button hides them
  const [shown, setShown] = useState(() => new Set(Object.keys(frontmatter).filter((k) => KNOWN.has(k))));
  const [addingMeta, setAddingMeta] = useState(false);
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);
  const meta = readMetadata(frontmatter);
  const otherRoot = Object.keys(frontmatter).filter((k) => !KNOWN.has(k) && k !== 'metadata');

  const set = (key: string, value: FrontmatterValue) => {
    const next = { ...frontmatter };
    if (value == null || value === '' || (Array.isArray(value) && value.length === 0)) delete next[key];
    else next[key] = value;
    onChange(next);
  };
  const setMeta = (key: string, value: FrontmatterValue | undefined) => {
    const next = { ...meta };
    if (value === undefined) delete next[key];
    else next[key] = value;
    onChange(withMetadata(frontmatter, next));
  };
  const hide = (key: string) => {
    setShown((prev) => {
      const next = new Set(prev);
      next.delete(key);
      return next;
    });
    set(key, null);
  };

  const visible = (f: FieldDef) => shown.has(f.key) && (!f.showWhen || frontmatter[f.showWhen.key] === f.showWhen.value);
  const unset = FIELDS.filter((f) => !shown.has(f.key) && (!f.showWhen || frontmatter[f.showWhen.key] === f.showWhen.value));
  const budget = String(frontmatter.description ?? '').length + String(frontmatter.when_to_use ?? '').length;

  const control = (f: FieldDef) => {
    const value = frontmatter[f.key];
    switch (f.type) {
      case 'bool':
        return (
          <button type="button" role="switch" aria-checked={value === true} aria-label={f.key} className={`ss-sw mt-[7px] ${value === true ? 'on' : ''}`} onClick={() => set(f.key, value === true ? false : true)}>
            <i />
          </button>
        );
      case 'enum':
        return (
          <SegmentedControl
            className="w-fit"
            value={String(value ?? '')}
            onChange={(v) => set(f.key, v)}
            options={f.options!.map((o) => ({ value: o, label: o || t('frontmatterEditor.default') }))}
          />
        );
      case 'list':
        return <ListInput value={value} onChange={(v) => set(f.key, v)} label={f.key} />;
      case 'multiline':
        return (
          <textarea
            aria-label={f.key}
            className="ss-inp area max-h-[420px] w-full resize-y outline-none !min-h-[140px] [field-sizing:content]"
            value={String(value ?? '')}
            onChange={(e) => set(f.key, e.target.value)}
          />
        );
      default:
        return <input aria-label={f.key} className="ss-inp w-full font-mono outline-none" value={String(value ?? '')} onChange={(e) => set(f.key, e.target.value)} />;
    }
  };

  const hintFor = (f: FieldDef) => {
    const hint = t(`frontmatterEditor.field.${f.hint}.hint`);
    return f.key === 'description' ? `${hint} · ${budget} / ${DESC_BUDGET}` : hint;
  };

  if (yaml) {
    return (
      <div className="flex flex-col gap-3">
        <Header yaml={yaml} onYaml={onYaml} />
        <CodeView content={serializeFrontmatter(frontmatter)} lang="yaml" />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <Header yaml={yaml} onYaml={onYaml} />
      {GROUPS.map((g) => {
        const fields = FIELDS.filter((f) => f.group === g && visible(f));
        if (fields.length === 0) return null;
        return (
          <FieldGroup key={g} label={t(`frontmatterEditor.group.${g}`)}>
            {fields.map((f) => (
              <Row key={f.key} name={f.key} hint={hintFor(f)} onRemove={() => hide(f.key)}>{control(f)}</Row>
            ))}
          </FieldGroup>
        );
      })}
      {(otherRoot.length > 0 || Object.keys(meta).length > 0 || addingMeta) && (
        <FieldGroup label={t('frontmatterEditor.group.metadata')}>
          {otherRoot.map((key) => (
            <Row key={key} name={key} onRemove={() => set(key, null)}>
              {/* Clearing the value keeps the key, so the row does not vanish while typing */}
              {Array.isArray(frontmatter[key])
                ? <ListInput value={frontmatter[key]} onChange={(v) => onChange({ ...frontmatter, [key]: v })} label={key} />
                : <input aria-label={key} className="ss-inp w-full font-mono outline-none" value={listText(frontmatter[key])} onChange={(e) => onChange({ ...frontmatter, [key]: e.target.value })} />}
            </Row>
          ))}
          {Object.keys(meta).map((key) => (
            <Row key={key} name={`metadata.${key}`} hint={key === 'targets' ? t('frontmatterEditor.field.targets.hint') : undefined} onRemove={() => setMeta(key, undefined)}>
              {LIST_METADATA.has(key) || Array.isArray(meta[key])
                ? <ListInput value={meta[key]} onChange={(v) => setMeta(key, v)} label={key} />
                : <input aria-label={key} className="ss-inp w-full font-mono outline-none" value={listText(meta[key])} onChange={(e) => setMeta(key, e.target.value)} />}
            </Row>
          ))}
          {addingMeta && (
            <div className="grid grid-cols-[150px_minmax(0,1fr)_30px] items-start gap-3">
              <input
                autoFocus
                aria-label={t('frontmatterEditor.metadataKey')}
                placeholder="metadata.key"
                className="ss-inp w-full font-mono outline-none"
                onBlur={(e) => {
                  const key = e.target.value.replace(/^metadata\./, '').trim();
                  if (key && !(key in meta)) setMeta(key, LIST_METADATA.has(key) ? [] : '');
                  setAddingMeta(false);
                }}
                onKeyDown={(e) => { if (e.key === 'Enter') e.currentTarget.blur(); }}
              />
              <span className="pt-[9px] text-xs text-ink-3">{t('frontmatterEditor.metadataKeyHint')}</span>
              <button type="button" className="ss-ib" aria-label={t('frontmatterEditor.removeField')} onMouseDown={(e) => e.preventDefault()} onClick={() => setAddingMeta(false)}>
                <X size={15} />
              </button>
            </div>
          )}
        </FieldGroup>
      )}
      <div className="flex min-w-0 items-center gap-3">
        <Button variant="secondary" size="sm" onClick={(e) => {
          const r = e.currentTarget.getBoundingClientRect();
          setMenu({ x: r.left, y: r.bottom + 4 });
        }}>
          <Plus size={14} />
          {t('frontmatterEditor.add')}
          <ChevronDown size={14} />
        </Button>
        {unset.length > 0 && <span className="truncate text-xs text-ink-3">{unset.map((f) => f.key).join(' · ')}</span>}
      </div>
      <SkillContextMenu
        open={!!menu}
        anchorPoint={menu ?? undefined}
        onClose={() => setMenu(null)}
        items={[
          ...unset.map((f) => ({ key: f.key, label: f.key, onSelect: () => setShown((prev) => new Set(prev).add(f.key)) })),
          { key: '__metadata', label: t('frontmatterEditor.customMetadata'), icon: <Plus size={14} />, onSelect: () => setAddingMeta(true) },
        ]}
      />
    </div>
  );
}

function Header({ yaml, onYaml }: { yaml: boolean; onYaml: (yaml: boolean) => void }) {
  const t = useT();
  return (
    <div className="flex h-8 items-center justify-between">
      <h2 className="ss-h2">{t('frontmatterEditor.title')}</h2>
      <SegmentedControl
        value={yaml ? 'yaml' : 'fields'}
        onChange={(v) => onYaml(v === 'yaml')}
        options={[{ value: 'fields', label: t('frontmatterEditor.viewFields') }, { value: 'yaml', label: t('frontmatterEditor.viewYaml') }]}
      />
    </div>
  );
}

function FieldGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <div className="mt-1.5 flex items-center gap-2.5">
        <span className="text-xs font-semibold uppercase tracking-[.05em] text-ink-3">{label}</span>
        <span className="flex-1" style={{ borderTop: 'var(--sep)' }} />
      </div>
      {children}
    </>
  );
}

function Row({ name, hint, onRemove, children }: { name: string; hint?: string; onRemove: () => void; children: ReactNode }) {
  const t = useT();
  return (
    <div className="grid grid-cols-[150px_minmax(0,1fr)_30px] items-start gap-3">
      <span className="break-all pt-[9px] font-mono text-[13px]">{name}</span>
      <div className="ss-fld !gap-1">
        {children}
        {hint && <span className="hp">{hint}</span>}
      </div>
      <button type="button" className="ss-ib" aria-label={t('frontmatterEditor.removeField')} onClick={onRemove}>
        <X size={15} />
      </button>
    </div>
  );
}

/** Keeps the typed text while focused so a trailing comma survives until the next entry. */
function ListInput({ value, onChange, label }: { value: FrontmatterValue | undefined; onChange: (v: string[]) => void; label: string }) {
  const [text, setText] = useState(() => listText(value));
  const [focused, setFocused] = useState(false);
  useEffect(() => {
    if (!focused) setText(listText(value));
  }, [value, focused]);
  return (
    <input
      aria-label={label}
      className="ss-inp w-full font-mono outline-none"
      value={text}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      onChange={(e) => {
        setText(e.target.value);
        onChange(splitList(e.target.value));
      }}
    />
  );
}
