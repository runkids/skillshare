import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import {
  AlignLeft,
  ArrowLeft,
  Check,
  Code2,
  Copy,
  ExternalLink,
  Eye,
  Files,
  Pencil,
  Target as TargetIcon,
  Type,
  X,
  Zap,
} from 'lucide-react';
import { useToast } from '../Toast';
import ConfirmDialog from '../ConfirmDialog';
import FrontmatterEditor from './FrontmatterEditor';
import { useT } from '../../i18n';
import Outline, { parseOutline, type HeadingItem } from './Outline';
import DiffView from './DiffView';
import {
  composeSkillMarkdown,
  parseSkillMarkdown,
  type Frontmatter,
} from '../../lib/frontmatter';
import { api, ApiError } from '../../api/client';
import Button from '../Button';
import EditorSegment from './controls/EditorSegment';
import './styles.css';

type PreviewMode = 'edit' | 'split' | 'preview';

export interface EditorTarget {
  id: string;
  name: string;
  status: 'ok' | 'pending' | 'off';
}

interface SkillEditorProps {
  skillName: string;
  displayName: string;
  kind: 'skill' | 'agent';
  path: string;
  tracked?: boolean;
  initialContent: string;
  fileCount: number;
  derived: {
    path: string;
    source?: string;
    version?: string;
    branch?: string;
    license?: string;
  };
  availableTargets: EditorTarget[];
  onBack: () => void;
  onSaved: (nextContent: string) => void;
}

export default function SkillEditor({
  skillName,
  displayName,
  kind,
  path: _path,
  tracked = false,
  initialContent,
  fileCount,
  derived,
  availableTargets,
  onBack,
  onSaved,
}: SkillEditorProps) {
  const { toast } = useToast();
  const t = useT();
  const initial = useMemo(() => parseSkillMarkdown(initialContent), [initialContent]);

  const [draftFrontmatter, setDraftFrontmatter] = useState<Frontmatter>(() =>
    migrateRootTargets({ ...initial.frontmatter })
  );
  const [draftBody, setDraftBody] = useState<string>(() => initial.body);
  const [yamlMode, setYamlMode] = useState(false);
  const [previewMode, setPreviewMode] = useState<PreviewMode>('split');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [showDiff, setShowDiff] = useState(false);
  const [activeSlug, setActiveSlug] = useState<string | null>(null);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);

  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const nameInputRef = useRef<HTMLInputElement | null>(null);
  const previewRef = useRef<HTMLDivElement | null>(null);
  const syncOriginRef = useRef<'ta' | 'preview' | null>(null);
  const syncResetTimerRef = useRef<number | null>(null);
  const syncRafRef = useRef<number | null>(null);
  const leftAnchorsRef = useRef<Map<number, number>>(new Map());

  const scheduleSyncReset = () => {
    if (syncResetTimerRef.current) window.clearTimeout(syncResetTimerRef.current);
    syncResetTimerRef.current = window.setTimeout(() => {
      syncOriginRef.current = null;
    }, 80);
  };

  const headings = useMemo(() => parseOutline(draftBody), [draftBody]);

  const getTextareaLineHeight = (ta: HTMLTextAreaElement) => {
    const cs = window.getComputedStyle(ta);
    const lh = parseFloat(cs.lineHeight);
    if (!Number.isFinite(lh) || lh <= 0) {
      return parseFloat(cs.fontSize) * 1.75 || 24;
    }
    return lh;
  };

  const recomputeLeftAnchors = useCallback(() => {
    const ta = textareaRef.current;
    if (!ta || headings.length === 0) {
      leftAnchorsRef.current = new Map();
      return;
    }
    leftAnchorsRef.current = measureTextareaLineTops(
      ta,
      headings.map((h) => h.line)
    );
  }, [headings]);

  useEffect(() => {
    const id = window.setTimeout(() => recomputeLeftAnchors(), 120);
    return () => window.clearTimeout(id);
  }, [recomputeLeftAnchors, draftBody, previewMode]);

  useEffect(() => {
    const ta = textareaRef.current;
    if (!ta || typeof ResizeObserver === 'undefined') return;
    const ro = new ResizeObserver(() => recomputeLeftAnchors());
    ro.observe(ta);
    return () => ro.disconnect();
  }, [recomputeLeftAnchors]);

  const buildSyncAnchors = (): Array<[number, number]> | null => {
    const ta = textareaRef.current;
    const pv = previewRef.current;
    if (!ta || !pv) return null;
    const taMax = Math.max(ta.scrollHeight - ta.clientHeight, 0);
    const pvMax = Math.max(pv.scrollHeight - pv.clientHeight, 0);
    if (taMax === 0 || pvMax === 0) return null;
    const lh = getTextareaLineHeight(ta);
    const leftCache = leftAnchorsRef.current;
    const nodes = Array.from(pv.querySelectorAll<HTMLElement>('[data-slug]'));
    const bySlug = new Map<string, HTMLElement>();
    for (const n of nodes) {
      const s = n.dataset.slug;
      if (s && !bySlug.has(s)) bySlug.set(s, n);
    }
    const pairs: Array<[number, number]> = [[0, 0]];
    for (const h of headings) {
      const el = bySlug.get(h.slug);
      if (!el) continue;
      const measured = leftCache.get(h.line);
      const leftRaw = measured != null ? measured : h.line * lh;
      const left = Math.min(leftRaw, taMax);
      const right = Math.min(el.offsetTop, pvMax);
      pairs.push([left, right]);
    }
    pairs.push([taMax, pvMax]);
    pairs.sort((a, b) => a[0] - b[0] || a[1] - b[1]);
    const dedup: Array<[number, number]> = [];
    for (const p of pairs) {
      const last = dedup[dedup.length - 1];
      if (!last || p[0] > last[0] + 0.5) dedup.push(p);
    }
    return dedup;
  };

  const interpolateSync = (
    pairs: Array<[number, number]>,
    value: number,
    reverse: boolean
  ): number => {
    const src = reverse ? 1 : 0;
    const dst = reverse ? 0 : 1;
    if (value <= pairs[0][src]) return pairs[0][dst];
    const last = pairs[pairs.length - 1];
    if (value >= last[src]) return last[dst];
    for (let i = 0; i < pairs.length - 1; i++) {
      const a = pairs[i];
      const b = pairs[i + 1];
      if (value >= a[src] && value <= b[src]) {
        const denom = b[src] - a[src];
        const ratio = denom === 0 ? 0 : (value - a[src]) / denom;
        return a[dst] + ratio * (b[dst] - a[dst]);
      }
    }
    return last[dst];
  };

  const scheduleSync = (origin: 'ta' | 'preview') => {
    if (syncOriginRef.current && syncOriginRef.current !== origin) return;
    if (syncRafRef.current != null) return;
    syncRafRef.current = window.requestAnimationFrame(() => {
      syncRafRef.current = null;
      const ta = textareaRef.current;
      const pv = previewRef.current;
      if (!ta || !pv) return;
      const pairs = buildSyncAnchors();
      if (!pairs) return;
      syncOriginRef.current = origin;
      if (origin === 'ta') {
        pv.scrollTop = interpolateSync(pairs, ta.scrollTop, false);
      } else {
        ta.scrollTop = interpolateSync(pairs, pv.scrollTop, true);
      }
      scheduleSyncReset();
    });
  };

  useEffect(() => {
    return () => {
      if (syncRafRef.current != null) window.cancelAnimationFrame(syncRafRef.current);
      if (syncResetTimerRef.current != null) window.clearTimeout(syncResetTimerRef.current);
    };
  }, []);

  const handleTextareaScroll = () => {
    if (syncOriginRef.current === 'preview') return;
    scheduleSync('ta');
  };

  const handlePreviewScroll = () => {
    if (syncOriginRef.current === 'ta') return;
    scheduleSync('preview');
  };

  const requestSave = useCallback(() => {
    if (!dirty) return;
    const descLen = String(draftFrontmatter.description ?? '').length;
    const wtuLen = String(draftFrontmatter.when_to_use ?? '').length;
    if (descLen + wtuLen > 1536) {
      toast(
        `Description + when_to_use exceed 1,536 chars (${descLen + wtuLen}). Trim either field.`,
        'error',
      );
      return;
    }
    setShowDiff(true);
  }, [dirty, draftFrontmatter, toast]);

  const commitSave = useCallback(async () => {
    setSaving(true);
    const next = composeSkillMarkdown(migrateRootTargets(draftFrontmatter), draftBody);
    try {
      await api.saveSkillContent(skillName, next, kind);
      setDirty(false);
      setShowDiff(false);
      toast(t('skillEditor.toast.saved', { name: skillName }), 'success');
      onSaved(next);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : String(err);
      toast(t('skillEditor.toast.saveFailed', { error: msg }), 'error');
    } finally {
      setSaving(false);
    }
  }, [draftFrontmatter, draftBody, skillName, kind, toast, onSaved]);

  const discardAndExit = useCallback(() => {
    setShowDiscardConfirm(false);
    onBack();
  }, [onBack]);

  const cancelEdit = useCallback(() => {
    if (dirty) {
      setShowDiscardConfirm(true);
      return;
    }
    discardAndExit();
  }, [dirty, discardAndExit]);

  const openInEditor = useCallback(async () => {
    try {
      const resp = await api.openSkillInEditor(skillName, { kind });
      toast(t('resourceDetail.toast.openedIn', { editor: resp.editor }), 'info');
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : String(err);
      toast(t('skillEditor.toast.openInEditorFailed', { error: msg }), 'error');
    }
  }, [skillName, kind, toast]);

  const copyPath = useCallback(() => {
    void navigator.clipboard?.writeText(derived.path);
    toast(t('skillEditor.toast.pathCopied'), 'info');
  }, [derived.path, toast]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const k = e.key.toLowerCase();
      const modifier = e.metaKey || e.ctrlKey;
      if (modifier && k === 's') {
        e.preventDefault();
        requestSave();
      } else if (modifier && k === 'p') {
        e.preventDefault();
        setPreviewMode((m) => (m === 'edit' ? 'split' : m === 'split' ? 'preview' : 'edit'));
      } else if (e.key === 'Escape' && !showDiff) {
        cancelEdit();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [requestSave, cancelEdit, showDiff]);

  const stats = useMemo(() => {
    const tokensDesc = Math.round(String(draftFrontmatter.description ?? '').length / 4);
    const tokensBody = Math.round(draftBody.length / 4);
    const totalTokens = tokensDesc + tokensBody;
    let words = 0;
    let lines = 1;
    let inWord = false;
    for (let i = 0; i < draftBody.length; i++) {
      const c = draftBody.charCodeAt(i);
      if (c === 10) lines++;
      const isSpace = c === 32 || c === 9 || c === 10 || c === 13;
      if (!isSpace) {
        if (!inWord) words++;
        inWord = true;
      } else {
        inWord = false;
      }
    }
    return { tokensDesc, tokensBody, totalTokens, words, lines, overBudget: totalTokens > 5000 };
  }, [draftFrontmatter.description, draftBody]);
  const { tokensDesc, tokensBody, totalTokens, words, lines, overBudget } = stats;

  const deferredBody = useDeferredValue(draftBody);

  const argsCount = useMemo(() => {
    const matches = draftBody.match(/\$ARGUMENTS\b/g);
    return matches ? matches.length : 0;
  }, [draftBody]);

  const patchFrontmatter = useCallback((next: Frontmatter) => {
    setDraftFrontmatter(next);
    setDirty(true);
  }, []);

  const patchBody = useCallback((next: string) => {
    setDraftBody(next);
    setDirty(true);
  }, []);

  const jumpToHeading = useCallback(
    (heading: HeadingItem) => {
      setActiveSlug(heading.slug);
      const ta = textareaRef.current;
      if (ta && (previewMode === 'edit' || previewMode === 'split')) {
        const beforeLines = draftBody.split('\n').slice(0, heading.line);
        const pos = beforeLines.join('\n').length + (heading.line > 0 ? 1 : 0);
        ta.focus();
        ta.setSelectionRange(pos, pos + heading.level + 1 + heading.text.length);
        ta.scrollTop = Math.max(0, (heading.line - 2) * 22);
      }
    },
    [draftBody, previewMode]
  );

  const markdownComponents: Components = useMemo(
    () => ({
      p: ({ children }) => <p>{highlightArgs(children)}</p>,
      li: ({ children }) => <li>{highlightArgs(children)}</li>,
      h1: ({ children }) => <h1 data-slug={slugifyChildren(children)}>{children}</h1>,
      h2: ({ children }) => <h2 data-slug={slugifyChildren(children)}>{children}</h2>,
      h3: ({ children }) => <h3 data-slug={slugifyChildren(children)}>{children}</h3>,
      h4: ({ children }) => <h4 data-slug={slugifyChildren(children)}>{children}</h4>,
      h5: ({ children }) => <h5 data-slug={slugifyChildren(children)}>{children}</h5>,
      h6: ({ children }) => <h6 data-slug={slugifyChildren(children)}>{children}</h6>,
    }),
    []
  );

  return (
    <div className="ss-skill-editor">
      <div className="mode-strip editing">
        <button type="button" className="back-btn" onClick={cancelEdit} aria-label={t('common.back')}>
          <ArrowLeft size={16} strokeWidth={2.2} />
        </button>
        <div className="title-row">
          <h1 className="title mono">{displayName}</h1>
          <span className="kind-badge">{kind.toUpperCase()}</span>
          {tracked && <span className="tracked-badge">{t('skillEditor.trackedBadge')}</span>}
          {dirty && <span className="dirty-pill">{t('skillEditor.dirtyPill')}</span>}
        </div>
        <div className="mode-actions">
          <Button variant="ghost" size="sm" onClick={openInEditor}>
            <ExternalLink size={14} /> {t('skillEditor.openInEditor')}
          </Button>
          <Button variant="ghost" size="sm" onClick={cancelEdit} disabled={saving}>
            <X size={14} /> {t('skillEditor.cancelButton')}
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={requestSave}
            disabled={!dirty || saving}
            loading={saving}
          >
            <Check size={14} /> {t('skillEditor.saveButton')}
            <span className="kbd-hint">⌘S</span>
          </Button>
        </div>
      </div>

      <div className="main-inner">
        <div className="content editing">
          <article className="doc">
            <section className="doc-hero">
              <div className="doc-kicker">
                <Pencil size={14} strokeWidth={2.2} />
                <span>{t('skillEditor.editingLabel')}</span>
                <span className="doc-kicker-sep">·</span>
                <span className="mono">{derived.path}</span>
                <button
                  type="button"
                  className="copy-btn"
                  onClick={copyPath}
                  title={t('skillEditor.copyPath')}
                >
                  <Copy size={12} strokeWidth={2.2} />
                </button>
                {derived.branch && (
                  <>
                    <span className="doc-kicker-sep">·</span>
                    <span className="mono kicker-meta">{derived.branch}</span>
                  </>
                )}
                {derived.version && (
                  <>
                    <span className="doc-kicker-sep">·</span>
                    <span className="mono kicker-meta">v{derived.version}</span>
                  </>
                )}
                {derived.license && (
                  <>
                    <span className="doc-kicker-sep">·</span>
                    <span className="mono kicker-meta">{derived.license}</span>
                  </>
                )}
                <span className="doc-kicker-sep">·</span>
                {dirty ? (
                  <span className="kicker-status dirty">{t('skillEditor.unsavedChanges')}</span>
                ) : (
                  <span className="kicker-status">{t('skillEditor.noChanges')}</span>
                )}
              </div>
              <div
                className="doc-hero-clickable"
                role="button"
                tabIndex={0}
                onClick={() => {
                  nameInputRef.current?.focus();
                  nameInputRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    nameInputRef.current?.focus();
                    nameInputRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
                  }
                }}
                title={t('skillEditor.titleClickHint')}
              >
                <h1 className="doc-title-display mono" data-empty={!draftFrontmatter.name}>
                  {String(draftFrontmatter.name ?? '') || 'untitled-skill'}
                </h1>
                <p className="doc-desc-display" data-empty={!draftFrontmatter.description}>
                  {String(draftFrontmatter.description ?? '') ||
                    t('skillEditor.descPlaceholder')}
                </p>
              </div>
            </section>

            <FrontmatterEditor
              frontmatter={draftFrontmatter}
              onChange={patchFrontmatter}
              yamlMode={yamlMode}
              onToggleYaml={setYamlMode}
              metadataHint={
                <p className="fm-metadata-tip">
                  <TargetIcon size={12} strokeWidth={2.2} />
                  <span>
                    <strong>{t('skillEditor.metadataHint.tip')}</strong>{' '}
                    {t('skillEditor.metadataHint.text')}
                    {availableTargets.length > 0 && (
                      <>
                        {' '}{t('skillEditor.metadataHint.available')} <code>{availableTargets.map((tgt) => tgt.name).join(', ')}</code>.
                      </>
                    )}
                  </span>
                </p>
              }
            />
          </article>
        </div>

        <div className="stats-bar">
          <span className={`stat${overBudget ? ' over' : ''}`}>
            <Zap size={14} strokeWidth={2.5} />{t('skillEditor.statsTokens', { total: totalTokens.toLocaleString() })}
            <span className="sub">
              {t('skillEditor.statsTokensSub', { desc: tokensDesc, body: tokensBody })}
            </span>
            {overBudget && (
              <span className="budget-warn" title={t('skillEditor.argsBudgetWarning')}>⚠ {t('skillEditor.statsBudget')}</span>
            )}
          </span>
          <span className="stat">
            <Type size={14} />
            {t('skillEditor.statsWords', { count: words.toLocaleString() })}
          </span>
          <span className="stat">
            <AlignLeft size={14} />
            {t('skillEditor.statsLines', { count: lines.toLocaleString() })}
          </span>
          <span className="stat">
            <Files size={14} />
            {t('skillEditor.statsFiles', { count: fileCount })}
          </span>
          <div className="stats-bar-actions">
            <EditorSegment<PreviewMode>
              value={previewMode}
              onChange={setPreviewMode}
              title="⌘P to cycle"
              options={[
                { value: 'edit', label: <><Pencil size={12} /> {t('skillEditor.pane.edit')}</> },
                { value: 'split', label: <><Code2 size={12} /> {t('skillEditor.pane.split')}</> },
                { value: 'preview', label: <><Eye size={12} /> {t('skillEditor.pane.preview')}</> },
              ]}
            />
            <Outline
              markdown={deferredBody}
              activeSlug={activeSlug}
              onJump={jumpToHeading}
            />
          </div>
        </div>

        <section className={`doc-body-edit pmode-${previewMode}`}>
          {previewMode !== 'preview' && (
            <div className="editor-pane">
              <div className="pane-head">
                <span>{t('skillEditor.pane.body')}</span>
                <span className="hint">
                  <span className="hint-group">
                    <kbd className="kbd-hint">⌘S</kbd>
                    <span className="hint-label">{t('skillEditor.pane.saveHint')}</span>
                  </span>
                  <span className="hint-group">
                    <kbd className="kbd-hint">⌘P</kbd>
                    <span className="hint-label">{t('skillEditor.pane.toggleHint')}</span>
                  </span>
                  <span className="hint-group">
                    <kbd className="kbd-hint">Esc</kbd>
                    <span className="hint-label">{t('skillEditor.pane.cancelHint')}</span>
                  </span>
                </span>
              </div>
              <div className="ta-wrap">
                <textarea
                  ref={textareaRef}
                  className="md-textarea"
                  value={draftBody}
                  onChange={(e) => patchBody(e.target.value)}
                  onScroll={handleTextareaScroll}
                  spellCheck={false}
                />
                {argsCount > 0 && (
                  <div
                    className="args-hint"
                    title={t('skillEditor.argsTitle')}
                  >
                    <span className="args-token-pill">$ARGUMENTS</span>
                    <span>{t('skillEditor.argsHint', { count: argsCount })}</span>
                  </div>
                )}
              </div>
            </div>
          )}
          {previewMode !== 'edit' && (
            <div className="editor-pane">
              <div className="pane-head">
                <span>{t('skillEditor.pane.preview')}</span>
                <span className="hint">{t('skillEditor.pane.previewLive')}</span>
              </div>
              <div
                ref={previewRef}
                className="md-preview md-view"
                onScroll={handlePreviewScroll}
              >
                <Markdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                  {deferredBody}
                </Markdown>
              </div>
            </div>
          )}
        </section>
      </div>

      {showDiff && (
        <DiffView
          open
          oldText={initialContent}
          newText={composeSkillMarkdown(migrateRootTargets(draftFrontmatter), draftBody)}
          onConfirm={commitSave}
          onCancel={() => setShowDiff(false)}
          saving={saving}
        />
      )}

      <ConfirmDialog
        open={showDiscardConfirm}
        title={t('skillEditor.discardConfirmTitle')}
        message={
          <p>
            {t('skillEditor.discardConfirmMessage', { skillName })}
          </p>
        }
        confirmText={t('skillEditor.discardButton')}
        cancelText={t('skillEditor.keepEditingButton')}
        variant="danger"
        onConfirm={discardAndExit}
        onCancel={() => setShowDiscardConfirm(false)}
      />
    </div>
  );
}

function migrateRootTargets(fm: Frontmatter): Frontmatter {
  if (!('targets' in fm)) return fm;
  const t = fm.targets;
  const list = Array.isArray(t)
    ? t.map((x) => String(x))
    : t == null || String(t).trim() === ''
      ? []
      : String(t).split(',').map((s) => s.trim()).filter(Boolean);
  // Leave an empty root `targets` in place so users can type it as a custom key
  // without it being silently deleted mid-edit. Only migrate once it has a value.
  if (list.length === 0) return fm;
  const next = { ...fm };
  delete next.targets;
  const meta =
    next.metadata && typeof next.metadata === 'object' && !Array.isArray(next.metadata)
      ? { ...(next.metadata as Record<string, unknown>) }
      : {};
  meta.targets = list;
  next.metadata = meta as Frontmatter[string];
  return next;
}

function highlightArgs(children: React.ReactNode): React.ReactNode {
  if (typeof children === 'string') return highlightArgsInString(children);
  if (Array.isArray(children)) {
    return children.map((c, i) =>
      typeof c === 'string' ? <span key={i}>{highlightArgsInString(c)}</span> : c
    );
  }
  return children;
}

function measureTextareaLineTops(
  ta: HTMLTextAreaElement,
  targetLines: number[]
): Map<number, number> {
  const out = new Map<number, number>();
  if (targetLines.length === 0) return out;
  const cs = window.getComputedStyle(ta);
  const mirror = document.createElement('div');
  mirror.style.position = 'absolute';
  mirror.style.visibility = 'hidden';
  mirror.style.pointerEvents = 'none';
  mirror.style.top = '0';
  mirror.style.left = '-9999px';
  mirror.style.boxSizing = cs.boxSizing;
  mirror.style.width = ta.clientWidth + 'px';
  mirror.style.paddingTop = cs.paddingTop;
  mirror.style.paddingRight = cs.paddingRight;
  mirror.style.paddingBottom = cs.paddingBottom;
  mirror.style.paddingLeft = cs.paddingLeft;
  mirror.style.borderTopWidth = '0';
  mirror.style.borderRightWidth = '0';
  mirror.style.borderBottomWidth = '0';
  mirror.style.borderLeftWidth = '0';
  mirror.style.fontFamily = cs.fontFamily;
  mirror.style.fontSize = cs.fontSize;
  mirror.style.fontWeight = cs.fontWeight;
  mirror.style.fontStyle = cs.fontStyle;
  mirror.style.lineHeight = cs.lineHeight;
  mirror.style.letterSpacing = cs.letterSpacing;
  mirror.style.tabSize = cs.tabSize;
  mirror.style.whiteSpace = 'pre-wrap';
  mirror.style.overflowWrap = cs.overflowWrap || 'break-word';
  mirror.style.wordBreak = cs.wordBreak;
  mirror.style.textIndent = cs.textIndent;

  const lines = ta.value.split('\n');
  const wantedSet = new Set(targetLines);
  const markers = new Map<number, HTMLSpanElement>();
  for (const ln of wantedSet) {
    const m = document.createElement('span');
    m.textContent = '\u200B';
    markers.set(ln, m);
  }

  for (let i = 0; i < lines.length; i++) {
    const marker = markers.get(i);
    if (marker) mirror.appendChild(marker);
    mirror.appendChild(document.createTextNode(lines[i] || '\u200B'));
    if (i < lines.length - 1) mirror.appendChild(document.createTextNode('\n'));
  }

  document.body.appendChild(mirror);
  for (const ln of wantedSet) {
    const m = markers.get(ln);
    if (m) out.set(ln, m.offsetTop);
  }
  document.body.removeChild(mirror);
  return out;
}

function slugifyChildren(children: React.ReactNode): string {
  const parts: string[] = [];
  const walk = (n: React.ReactNode) => {
    if (n == null || typeof n === 'boolean') return;
    if (typeof n === 'string' || typeof n === 'number') {
      parts.push(String(n));
      return;
    }
    if (Array.isArray(n)) {
      n.forEach(walk);
      return;
    }
    if (typeof n === 'object' && 'props' in (n as { props?: unknown })) {
      walk((n as { props: { children?: React.ReactNode } }).props?.children);
    }
  };
  walk(children);
  return parts
    .join('')
    .toLowerCase()
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\*+/g, '')
    .replace(/[^\w\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-');
}

function highlightArgsInString(s: string): React.ReactNode {
  const parts = s.split(/(\$ARGUMENTS\b)/g);
  if (parts.length === 1) return s;
  return parts.map((p, i) =>
    p === '$ARGUMENTS' ? (
      <span key={i} className="arg-token">
        $ARGUMENTS
      </span>
    ) : (
      p
    )
  );
}

