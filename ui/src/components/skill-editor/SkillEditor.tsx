import { Fragment, useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { TriangleAlert } from 'lucide-react';
import { api, ApiError, type Skill } from '../../api/client';
import { composeSkillMarkdown, parseSkillMarkdown, type Frontmatter } from '../../lib/frontmatter';
import { highlightLines } from '../../lib/highlight';
import { parseRemoteURL } from '../../lib/parseRemoteURL';
import { formatTrackedRepoName } from '../../lib/resourceNames';
import { useI18n, useT } from '../../i18n';
import Button from '../Button';
import ConfirmDialog from '../ConfirmDialog';
import MarkdownView from '../MarkdownView';
import PageHeader from '../PageHeader';
import SegmentedControl from '../SegmentedControl';
import { useToast } from '../Toast';
import DiffView from './DiffView';
import FrontmatterEditor, { DESC_BUDGET } from './FrontmatterEditor';

type Pane = 'edit' | 'split' | 'preview';

const TOKEN_BUDGET = 5000;

/** ATX headings outside code fences, with their level and line index in the body. */
function outline(body: string) {
  const out: { text: string; line: number; level: number }[] = [];
  let fence = false;
  body.split('\n').forEach((line, i) => {
    if (/^\s*```/.test(line)) fence = !fence;
    const m = !fence && line.match(/^(#{1,6})\s+(.+?)\s*$/);
    if (m) out.push({ text: m[2].replace(/`([^`]+)`/g, '$1').replace(/\*+/g, ''), line: i, level: m[1].length });
  });
  return out;
}

/** Maps a scroll position through [from, to] anchor pairs, linear between neighbours. */
function interpolate(pairs: [number, number][], value: number, from: 0 | 1) {
  const to = from === 0 ? 1 : 0;
  for (let i = 1; i < pairs.length; i++) {
    const [a, b] = [pairs[i - 1], pairs[i]];
    if (value <= b[from]) return a[to] + ((value - a[from]) / (b[from] - a[from] || 1)) * (b[to] - a[to]);
  }
  return pairs[pairs.length - 1][to];
}

/** Moves a root `targets` list under `metadata.targets`, where sync reads it. */
function migrateRootTargets(fm: Frontmatter): Frontmatter {
  if (!('targets' in fm)) return fm;
  const v = fm.targets;
  const list = Array.isArray(v)
    ? v.map((x) => String(x))
    : v == null || String(v).trim() === ''
      ? []
      : String(v).split(',').map((s) => s.trim()).filter(Boolean);
  // An empty root `targets` stays so it is not deleted while the user is still typing it
  if (list.length === 0) return fm;
  const next = { ...fm };
  delete next.targets;
  const meta = next.metadata && typeof next.metadata === 'object' && !Array.isArray(next.metadata)
    ? { ...(next.metadata as Record<string, unknown>) }
    : {};
  meta.targets = list;
  next.metadata = meta as Frontmatter[string];
  return next;
}

interface Props {
  resource: Skill;
  docName: string;
  initialContent: string;
  onBack: () => void;
  onSaved: (next: string) => void;
}

export default function SkillEditor({ resource, docName, initialContent, onBack, onSaved }: Props) {
  const t = useT();
  const { locale } = useI18n();
  const { toast } = useToast();
  const navigate = useNavigate();
  const initial = useMemo(() => parseSkillMarkdown(initialContent), [initialContent]);
  const [fm, setFm] = useState<Frontmatter>(() => migrateRootTargets({ ...initial.frontmatter }));
  const [fmDirty, setFmDirty] = useState(false);
  const [body, setBody] = useState(initial.body);
  const [source, setSource] = useState(resource.source ?? '');
  const [pane, setPane] = useState<Pane>('edit');
  const [yaml, setYaml] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [leaveTo, setLeaveTo] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const rowsRef = useRef<HTMLDivElement>(null);
  const previewRef = useRef<HTMLDivElement>(null);
  const syncing = useRef<{ from: 'ed' | 'pv'; timer: number } | null>(null);
  const deferredBody = useDeferredValue(body);

  const kindName = resource.kind === 'agent' ? 'agent' : 'skill';
  const listPath = resource.kind === 'agent' ? '/agents' : '/skills';
  const dirty = fmDirty || body !== initial.body;
  const sourceChanged = source !== (resource.source ?? '');
  const pending = dirty || sourceChanged;
  const next = composeSkillMarkdown(migrateRootTargets(fm), body, undefined, fmDirty ? undefined : initial.rawFrontmatter);

  const save = async () => {
    setSaving(true);
    try {
      if (sourceChanged) await api.updateSkillSource(resource.flatName, source.trim(), resource.kind);
      if (dirty) await api.saveSkillContent(resource.flatName, next, resource.kind);
      setReviewing(false);
      toast(t('skillEditor.toast.saved', { name: resource.name }), 'success');
      onSaved(next);
    } catch (err) {
      toast(t('skillEditor.toast.saveFailed', { error: err instanceof ApiError ? err.message : String(err) }), 'error');
    } finally {
      setSaving(false);
    }
  };

  const requestSave = () => {
    if (!pending || saving) return;
    const used = String(fm.description ?? '').length + String(fm.when_to_use ?? '').length;
    if (used > DESC_BUDGET) {
      toast(t('skillEditor.toast.descTooLong', { count: used, budget: DESC_BUDGET }), 'error');
      return;
    }
    // A source change alone has no file diff to review
    if (!dirty) void save();
    else setReviewing(true);
  };

  /** `null` target means back to the detail view. */
  const leave = (to: string | null) => {
    if (pending) setLeaveTo(to ?? '');
    else if (to) navigate(to);
    else onBack();
  };

  // Re-registered every render so the handler sees current state
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey;
      const k = e.key.toLowerCase();
      if (mod && k === 's') {
        e.preventDefault();
        requestSave();
      } else if (mod && k === 'p') {
        e.preventDefault();
        setPane((p) => (p === 'edit' ? 'split' : p === 'split' ? 'preview' : 'edit'));
      } else if (e.key === 'Escape' && !reviewing && leaveTo === null) {
        leave(null);
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  });

  const jump = useCallback((line: number) => {
    const ta = textareaRef.current;
    if (!ta) return;
    const pos = body.split('\n').slice(0, line).join('\n').length + (line > 0 ? 1 : 0);
    ta.focus({ preventScroll: true });
    ta.setSelectionRange(pos, pos);
    const row = rowsRef.current?.children[line] as HTMLElement | undefined;
    if (row && scrollRef.current) scrollRef.current.scrollTop = row.offsetTop - 14;
  }, [body]);

  const tokens = Math.round((String(fm.description ?? '').length + body.length) / 4);
  const compact = new Intl.NumberFormat(locale, { notation: 'compact', maximumFractionDigits: 1 });
  const allHeadings = useMemo(() => outline(body), [body]);
  const headings = allHeadings.filter((h) => h.level === 2);

  // Split view keeps both panes on the same section: headings anchor the two scroll positions
  const syncScroll = (from: 'ed' | 'pv') => {
    const ed = scrollRef.current;
    const pv = previewRef.current;
    const rows = rowsRef.current;
    if (pane !== 'split' || !ed || !pv || !rows) return;
    if (syncing.current && syncing.current.from !== from) return;
    const edMax = ed.scrollHeight - ed.clientHeight;
    const pvMax = pv.scrollHeight - pv.clientHeight;
    if (edMax <= 0 || pvMax <= 0) return;
    const pairs: [number, number][] = [[0, 0]];
    const els = pv.querySelectorAll<HTMLElement>('h1, h2, h3, h4, h5, h6');
    // ponytail: headings pair up by order; setext or quoted headings break the count, then only the ends anchor
    if (els.length === allHeadings.length) {
      allHeadings.forEach((h, i) => {
        const left = Math.min((rows.children[h.line] as HTMLElement).offsetTop, edMax);
        const right = Math.min(els[i].offsetTop, pvMax);
        const last = pairs[pairs.length - 1];
        if (left > last[0] && right > last[1]) pairs.push([left, right]);
      });
    }
    pairs.push([edMax, pvMax]);
    if (syncing.current) window.clearTimeout(syncing.current.timer);
    // The echo scroll event from the other pane is ignored until this side goes quiet
    syncing.current = { from, timer: window.setTimeout(() => { syncing.current = null; }, 120) };
    if (from === 'ed') pv.scrollTop = interpolate(pairs, ed.scrollTop, 0);
    else ed.scrollTop = interpolate(pairs, pv.scrollTop, 1);
  };
  const bodyLines = useMemo(() => highlightLines(body, 'md'), [body]);
  const gutter = `calc(30px + ${String(bodyLines.length).length}ch)`;
  const repo = resource.isInRepo
    ? parseRemoteURL(resource.repoUrl)?.ownerRepo ?? formatTrackedRepoName(resource.relPath.split('/')[0])
    : '';

  return (
    <div className="animate-fade-in">
      <PageHeader
        crumbs={[
          { label: t(resource.kind === 'agent' ? 'layout.nav.agents' : 'layout.nav.skills'), onClick: () => leave(listPath) },
          { label: resource.name, onClick: () => leave(null) },
          { label: t('resourceDetail.actions.edit') },
        ]}
        title={resource.name}
        mono
        subtitle={t('skillEditor.editingFile', { file: docName })}
        actions={
          <>
            <Button variant="ghost" onClick={() => leave(null)} disabled={saving}>{t('skillEditor.discard')}</Button>
            <Button variant="primary" onClick={requestSave} disabled={!pending} loading={saving}>{t('skillEditor.saveButton')}</Button>
          </>
        }
      />

      {resource.isInRepo && (
        <div className="ss-note warn mb-5">
          <TriangleAlert size={16} />
          <div className="flex-1"><b>{t(`skillEditor.tracked.${kindName}`)}</b> {t('skillEditor.tracked.body', { repo })}</div>
        </div>
      )}

      <div className="flex flex-col gap-10">
        <section>
          <FrontmatterEditor frontmatter={fm} onChange={(v) => { setFm(v); setFmDirty(true); }} yaml={yaml} onYaml={setYaml} />
        </section>

        <section className="flex min-w-0 flex-col gap-3">
          <div className="flex h-8 items-center gap-3">
            <h2 className="ss-h2">{t('skillEditor.body')}</h2>
            <span className={`ss-st ${tokens > TOKEN_BUDGET ? 'warn' : 'ok'}`}>
              {t(tokens > TOKEN_BUDGET ? 'skillEditor.overBudget' : 'skillEditor.underBudget', { tokens: compact.format(tokens), budget: compact.format(TOKEN_BUDGET) })}
            </span>
            <span className="flex-1" />
            <SegmentedControl
              value={pane}
              onChange={setPane}
              options={[
                { value: 'edit', label: t('skillEditor.pane.edit') },
                { value: 'split', label: t('skillEditor.pane.split') },
                { value: 'preview', label: t('skillEditor.pane.preview') },
              ]}
            />
          </div>
          <div className={`grid h-[min(72vh,780px)] min-h-[320px] gap-4 ${pane === 'split' ? 'grid-cols-2' : 'grid-cols-1'}`}>
            {pane !== 'preview' && (
              // A transparent textarea sits over a highlighted copy with the same font and wrapping,
              // so both grow together and the outer box is the only scroller
              <div ref={scrollRef} className="ss-code ss-ed min-h-0 !overflow-auto !p-0" onScroll={() => syncScroll('ed')}>
                <div className="relative min-h-full">
                  <div ref={rowsRef} aria-hidden className="py-3.5 pr-4">
                    {bodyLines.map((node, i) => (
                      <div key={i} className="grid" style={{ gridTemplateColumns: `${gutter} minmax(0,1fr)` }}>
                        <span className="ln !mr-0 pl-4 pr-3.5 text-right">{i + 1}</span>
                        <span className="whitespace-pre-wrap [overflow-wrap:anywhere]">{node}</span>
                      </div>
                    ))}
                  </div>
                  <textarea
                    ref={textareaRef}
                    aria-label={t('skillEditor.body')}
                    value={body}
                    spellCheck={false}
                    className="absolute inset-0 h-full w-full resize-none overflow-hidden whitespace-pre-wrap bg-transparent py-3.5 pr-4 outline-none [overflow-wrap:anywhere]"
                    style={{ paddingLeft: gutter }}
                    onChange={(e) => setBody(e.target.value)}
                  />
                </div>
              </div>
            )}
            {pane !== 'edit' && (
              <div ref={previewRef} className="ss-box relative min-h-0 overflow-auto !shadow-none !px-[26px] !py-[22px]" onScroll={() => syncScroll('pv')}>
                <MarkdownView size="lg">{deferredBody}</MarkdownView>
              </div>
            )}
          </div>
          {headings.length > 0 && (
            <p className="min-w-0 truncate text-xs text-ink-3">
              {t('skillEditor.outline')}{' '}
              {headings.map((h, i) => (
                <Fragment key={h.line}>
                  {i > 0 && ' · '}
                  <button type="button" className="cursor-pointer hover:text-ink" onClick={() => jump(h.line)}>{h.text}</button>
                </Fragment>
              ))}
            </p>
          )}
          {/\$ARGUMENTS\b/.test(body) && (
            <p className="-mt-1 text-xs text-ink-3"><span className="font-mono">$ARGUMENTS</span> {t('skillEditor.argumentsHint')}</p>
          )}
        </section>

        <label className="ss-fld">
          <span className="ss-h2">{t('skillEditor.sourceLabel')}</span>
          <input className="ss-inp w-full font-mono outline-none" value={source} placeholder={t('skillEditor.sourcePlaceholder')} onChange={(e) => setSource(e.target.value)} />
          <span className="hp">{t('skillEditor.sourceHint')}</span>
        </label>
      </div>

      {reviewing && <DiffView oldText={initialContent} newText={next} saving={saving} onConfirm={save} onCancel={() => setReviewing(false)} />}
      <ConfirmDialog
        open={leaveTo !== null}
        title={t('skillEditor.discardConfirmTitle')}
        message={t('skillEditor.leaveMessage', { name: resource.name })}
        confirmText={t('skillEditor.discardButton')}
        cancelText={t('skillEditor.keepEditingButton')}
        variant="danger"
        onConfirm={() => {
          const to = leaveTo;
          setLeaveTo(null);
          if (to) navigate(to);
          else onBack();
        }}
        onCancel={() => setLeaveTo(null)}
      />
    </div>
  );
}
