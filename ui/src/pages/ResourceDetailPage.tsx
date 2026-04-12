import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react';
import { useBlocker, useBeforeUnload, useNavigate, useParams, useSearchParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  ArrowUpRight,
  BookOpen,
  Braces,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  Eye,
  EyeOff,
  File,
  FileCode2,
  FileText,
  FolderOpen,
  Link2,
  RefreshCw,
  Settings,
  ShieldCheck,
  Target,
  Trash2,
} from 'lucide-react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import Badge from '../components/Badge';
import Button from '../components/Button';
import Card from '../components/Card';
import ConfirmDialog from '../components/ConfirmDialog';
import ContentStatsBar from '../components/ContentStatsBar';
import CopyButton from '../components/CopyButton';
import IconButton from '../components/IconButton';
import { Input, Textarea } from '../components/Input';
import KindBadge from '../components/KindBadge';
import SkillFrontmatterWorkspace, { type FrontmatterWorkspaceEntry } from '../components/SkillFrontmatterWorkspace';
import SkillMarkdownEditor, { type SkillMarkdownEditorSurface } from '../components/SkillMarkdownEditor';
import { createSkillMarkdownComponents } from '../components/SkillMarkdownComponents';
import { SkillDetailSkeleton } from '../components/Skeleton';
import SourceBadge from '../components/SourceBadge';
import Spinner from '../components/Spinner';
import { useToast } from '../components/Toast';
import { BlockStamp, RiskMeter } from '../components/audit';
import { radius, shadows } from '../design';
import { useSyncMatrix } from '../hooks/useSyncMatrix';
import { api, type Skill } from '../api/client';
import { clearAuditCache } from '../lib/auditCache';
import {
  SKILL_FRONTMATTER_FIELDS,
  getAdditionalFrontmatterEntries,
  getReferenceFrontmatterEntries,
} from '../lib/skillFrontmatter';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { formatTrackedRepoName, formatSkillDisplayName } from '../lib/resourceNames';
import {
  parseSkillMarkdown,
  renameSkillFrontmatterField,
  serializeFrontmatterEditorValue,
  updateSkillFrontmatterField,
} from '../lib/skillMarkdown';
import { severityBadgeVariant } from '../lib/severity';

const FileViewerModal = lazy(() => import('../components/FileViewerModal'));

type FrontmatterCardInputState = {
  key?: string;
  value?: string;
};

type FrontmatterCardInputMap = Record<string, FrontmatterCardInputState>;

type WorkspaceSlotState = {
  id: string;
  key: string;
};

type SaveIndicatorState = 'idle' | 'saving' | 'saved' | 'error';

const MANIFEST_FRONTMATTER_KEYS = new Set(['name', 'description']);
const BOOLEAN_FRONTMATTER_KEYS = new Set(['disable-model-invocation', 'user-invocable']);
const STRUCTURED_FRONTMATTER_KEYS = new Set(['hooks', 'metadata']);
const BUILT_IN_FRONTMATTER_ORDER = SKILL_FRONTMATTER_FIELDS
  .map((field) => field.key)
  .filter((key) => !MANIFEST_FRONTMATTER_KEYS.has(key));
const FRONTMATTER_STATUS_RESET_MS = 1800;
const AUTOSAVE_DELAY_MS = 250;

function getFileIcon(filename: string): { icon: typeof File; className: string } {
  if (filename === 'SKILL.md') return { icon: FileText, className: 'text-blue' };
  if (/\.(ts|tsx|js|jsx|go|py|rs|rb|sh|bash)$/i.test(filename)) return { icon: FileCode2, className: 'text-pencil-light' };
  if (/\.json$/i.test(filename)) return { icon: Braces, className: 'text-pencil-light' };
  if (/\.(yaml|yml|toml)$/i.test(filename)) return { icon: Settings, className: 'text-pencil-light' };
  if (/\.md$/i.test(filename)) return { icon: BookOpen, className: 'text-pencil-light' };
  if (filename.endsWith('/')) return { icon: FolderOpen, className: 'text-warning' };
  return { icon: File, className: 'text-pencil-light' };
}

function getBuiltInFrontmatterKeys(frontmatter: Record<string, unknown>) {
  return getReferenceFrontmatterEntries(frontmatter)
    .filter((entry) => !MANIFEST_FRONTMATTER_KEYS.has(entry.key) && entry.isSet)
    .map((entry) => entry.key);
}

function buildWorkspaceInputMap(key: string, value = ''): FrontmatterCardInputState {
  return { key, value };
}

function sortWorkspaceSlotsByKey(entries: WorkspaceSlotState[], inputValues: FrontmatterCardInputMap): WorkspaceSlotState[] {
  return [...entries].sort((left, right) => {
    const leftKey = (inputValues[left.id]?.key ?? left.key).trim();
    const rightKey = (inputValues[right.id]?.key ?? right.key).trim();
    if (!leftKey && !rightKey) return left.id.localeCompare(right.id);
    if (!leftKey) return 1;
    if (!rightKey) return -1;
    return leftKey.localeCompare(rightKey);
  });
}

function buildWorkspaceSlotsFromContent(content: string): WorkspaceSlotState[] {
  const parsed = parseSkillMarkdown(content);
  return [
    ...orderBuiltInKeys(getBuiltInFrontmatterKeys(parsed.frontmatter)).map((key) => ({
      id: `slot:${key}`,
      key,
    })),
    ...getAdditionalFrontmatterEntries(parsed.frontmatter).map((entry) => ({
      id: `slot:${entry.key}`,
      key: entry.key,
    })),
  ];
}

function isStructuredFrontmatterEntry(key: string, value: unknown): boolean {
  return (
    Array.isArray(value)
    || (typeof value === 'object' && value !== null)
    || STRUCTURED_FRONTMATTER_KEYS.has(key)
  );
}

function orderBuiltInKeys(keys: Iterable<string>) {
  const keySet = new Set(keys);
  return BUILT_IN_FRONTMATTER_ORDER.filter((key) => keySet.has(key));
}

function mergeFrontmatterInputState(
  current: FrontmatterCardInputMap,
  entryId: string,
  next: Partial<FrontmatterCardInputState>,
): FrontmatterCardInputMap {
  const existing = current[entryId] ?? {};
  const merged = { ...existing, ...next };

  if (merged.key === undefined && merged.value === undefined) {
    if (!(entryId in current)) {
      return current;
    }

    const nextState = { ...current };
    delete nextState[entryId];
    return nextState;
  }

  return { ...current, [entryId]: merged };
}

export default function ResourceDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const requestedKind = searchParams.get('kind') === 'agent'
    ? 'agent'
    : searchParams.get('kind') === 'skill'
      ? 'skill'
      : undefined;
  const { toast } = useToast();

  const { data, isPending, error } = useQuery({
    queryKey: [...queryKeys.skills.detail(name!), requestedKind],
    queryFn: () => api.getResource(name!, requestedKind),
    staleTime: staleTimes.skills,
    enabled: !!name,
  });
  const allSkills = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const skillKind = data?.resource.kind;
  const auditQuery = useQuery({
    queryKey: [...queryKeys.audit.skill(name!), skillKind],
    queryFn: () => api.auditSkill(name!, skillKind),
    staleTime: staleTimes.auditSkill,
    enabled: !!name && !!skillKind,
  });
  const diffQuery = useQuery({
    queryKey: queryKeys.diff(),
    queryFn: () => api.diff(),
    staleTime: staleTimes.diff,
  });

  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [blockedMessage, setBlockedMessage] = useState<string | null>(null);
  const [viewingFile, setViewingFile] = useState<string | null>(null);
  const [savedContent, setSavedContent] = useState('');
  const [draftContent, setDraftContent] = useState('');
  const [editorSurface, setEditorSurface] = useState<SkillMarkdownEditorSurface>('rich');
  const [saveError, setSaveError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [saveState, setSaveState] = useState<SaveIndicatorState>('idle');
  const [showFrontmatterGuide, setShowFrontmatterGuide] = useState(false);
  const [frontmatterInputValues, setFrontmatterInputValues] = useState<FrontmatterCardInputMap>({});
  const [frontmatterErrors, setFrontmatterErrors] = useState<Record<string, string>>({});
  const [workspaceSlots, setWorkspaceSlots] = useState<WorkspaceSlotState[]>([]);
  const descriptionTextareaRef = useRef<HTMLTextAreaElement | null>(null);
  const lastLoadedResourceRef = useRef<string | null>(null);
  const customEntryIdRef = useRef(0);
  const lastAppliedQueryContentRef = useRef<string | null>(null);
  const pendingSavedContentRef = useRef<string | null>(null);
  const pendingSuccessToastRef = useRef(false);
  const autosaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const saveStatusTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const queuedAutosaveContentRef = useRef<string | null>(null);
  const draftContentRef = useRef('');
  const savedContentRef = useRef('');
  const frontmatterInputValuesRef = useRef<FrontmatterCardInputMap>({});
  const workspaceSlotsRef = useRef<WorkspaceSlotState[]>([]);
  const hasFrontmatterValidationErrorsRef = useRef(false);
  const isSavingRef = useRef(false);

  const resource = data?.resource;
  const skillMdContent = data?.skillMdContent ?? '';
  const files = data?.files ?? [];
  const canEditPrimaryContent = resource?.kind === 'skill';
  const previewSource = canEditPrimaryContent ? draftContent : skillMdContent;
  const parsedDoc = useMemo(() => parseSkillMarkdown(previewSource), [previewSource]);
  const isDirty = canEditPrimaryContent && draftContent !== savedContent;
  const renderedMarkdown = parsedDoc.markdown.trim() ? parsedDoc.markdown : previewSource;
  const canonicalBuiltInKeys = useMemo(
    () => getBuiltInFrontmatterKeys(parsedDoc.frontmatter),
    [parsedDoc.frontmatter],
  );
  const canonicalAdditionalFrontmatterEntries = useMemo(
    () => getAdditionalFrontmatterEntries(parsedDoc.frontmatter),
    [parsedDoc.frontmatter],
  );
  const workspaceSlotEntries = useMemo(() => {
    const canonicalKeys = new Set([
      ...canonicalBuiltInKeys,
      ...canonicalAdditionalFrontmatterEntries.map((entry) => entry.key),
    ]);
    const currentByKey = new Map(workspaceSlots.map((slot) => [slot.key, slot]));
    const canonicalSlots = [
      ...canonicalBuiltInKeys.map((key) => currentByKey.get(key) ?? { id: `slot:${key}`, key }),
      ...canonicalAdditionalFrontmatterEntries.map(({ key }) => currentByKey.get(key) ?? { id: `slot:${key}`, key }),
    ];
    const localOnlySlots = workspaceSlots.filter((slot) => {
      const displayKey = (frontmatterInputValues[slot.id]?.key ?? slot.key).trim();
      if (displayKey.length === 0) {
        return true;
      }
      return !canonicalKeys.has(displayKey);
    });

    const seenIds = new Set<string>();
    const merged: WorkspaceSlotState[] = [];
    for (const slot of [...canonicalSlots, ...localOnlySlots]) {
      if (seenIds.has(slot.id)) continue;
      seenIds.add(slot.id);
      merged.push(slot);
    }
    return merged;
  }, [canonicalAdditionalFrontmatterEntries, canonicalBuiltInKeys, frontmatterInputValues, workspaceSlots]);
  const workspaceBuiltInEntries = useMemo<FrontmatterWorkspaceEntry[]>(() => {
    const grouped = workspaceSlotEntries.reduce<FrontmatterWorkspaceEntry[]>((entries, slot) => {
        const inputState = frontmatterInputValues[slot.id];
        const displayKey = (inputState?.key ?? slot.key).trim();
        if (!BUILT_IN_FRONTMATTER_ORDER.includes(displayKey)) {
          return entries;
        }
        const canonicalValue = displayKey ? parsedDoc.frontmatter[displayKey] : undefined;
        entries.push({
          id: slot.id,
          key: displayKey,
          value: inputState?.value ?? serializeFrontmatterEditorValue(canonicalValue),
          isBoolean: BOOLEAN_FRONTMATTER_KEYS.has(displayKey),
          isStructured: isStructuredFrontmatterEntry(displayKey, canonicalValue),
          isCustom: false,
          error: frontmatterErrors[slot.id] ?? null,
        });
        return entries;
      }, []);

    const groupedByKey = new Map(grouped.map((entry) => [entry.key, entry]));
    return orderBuiltInKeys(grouped.map((entry) => entry.key))
      .map((key) => groupedByKey.get(key))
      .filter((entry): entry is FrontmatterWorkspaceEntry => Boolean(entry));
  }, [frontmatterErrors, frontmatterInputValues, parsedDoc.frontmatter, workspaceSlotEntries]);
  const workspaceAdditionalEntries = useMemo<FrontmatterWorkspaceEntry[]>(() => (
    sortWorkspaceSlotsByKey(
      workspaceSlotEntries.filter((slot) => {
        const displayKey = (frontmatterInputValues[slot.id]?.key ?? slot.key).trim();
        return displayKey.length === 0 || !BUILT_IN_FRONTMATTER_ORDER.includes(displayKey);
      }),
      frontmatterInputValues,
    ).map((slot) => {
      const inputState = frontmatterInputValues[slot.id];
      const displayKey = inputState?.key ?? slot.key;
      const canonicalValue = displayKey ? parsedDoc.frontmatter[displayKey] : undefined;

      return {
        id: slot.id,
        key: displayKey,
        value: inputState?.value ?? serializeFrontmatterEditorValue(canonicalValue),
        isBoolean: typeof canonicalValue === 'boolean',
        isStructured: isStructuredFrontmatterEntry(displayKey, canonicalValue),
        isCustom: true,
        error: frontmatterErrors[slot.id] ?? null,
      };
    })
  ), [frontmatterErrors, frontmatterInputValues, parsedDoc.frontmatter, workspaceSlotEntries]);
  const hasConfiguredFrontmatter = canonicalBuiltInKeys.length > 0 || canonicalAdditionalFrontmatterEntries.length > 0;
  const hasPendingFrontmatterEdits = useMemo(() => workspaceSlotEntries.some((slot) => {
    const inputState = frontmatterInputValues[slot.id];
    if (!inputState) {
      return false;
    }

    const displayKey = inputState.key ?? slot.key;
    const canonicalValue = displayKey ? serializeFrontmatterEditorValue(parsedDoc.frontmatter[displayKey]) : '';
    const displayValue = inputState.value ?? canonicalValue;

    return displayKey !== slot.key || displayValue !== canonicalValue;
  }), [frontmatterInputValues, parsedDoc.frontmatter, workspaceSlotEntries]);
  const hasFrontmatterValidationErrors = Object.keys(frontmatterErrors).length > 0;
  const hasUnsavedChanges = Boolean(isDirty || hasPendingFrontmatterEdits || isSaving);
  const blocker = useBlocker(hasUnsavedChanges);

  const skillMaps = useMemo(() => {
    const resources = allSkills.data?.resources ?? [];
    const byName = new Map<string, Skill>();
    const byFlat = new Map<string, Skill>();
    for (const item of resources) {
      byName.set(item.name, item);
      byFlat.set(item.flatName, item);
    }
    return { byName, byFlat };
  }, [allSkills.data]);

  const mdComponents = useMemo(() => createSkillMarkdownComponents({
    renderLink: ({ href, children, props }) => {
      if (!href || !resource) {
        return (
          <a href={href} target="_blank" rel="noopener noreferrer" {...props}>
            {children}
          </a>
        );
      }

      if (!href.startsWith('http') && !href.startsWith('#')) {
        const resolved = skillMaps.byName.get(href);
        if (resolved) {
          return (
            <Link
              to={`/resources/${encodeURIComponent(resolved.flatName)}`}
              className="link-subtle inline-flex items-center gap-0.5"
            >
              {children}
              <ArrowUpRight size={12} strokeWidth={2.5} className="shrink-0" />
            </Link>
          );
        }

        const childFlat = `${resource.flatName}__${href.replace(/\//g, '__')}`;
        const childResource = skillMaps.byFlat.get(childFlat);
        if (childResource) {
          return (
            <Link
              to={`/resources/${encodeURIComponent(childResource.flatName)}`}
              className="link-subtle inline-flex items-center gap-0.5"
            >
              {children}
              <ArrowUpRight size={12} strokeWidth={2.5} className="shrink-0" />
            </Link>
          );
        }

        const matchedFile = files.find((file) => file === href || file.endsWith(`/${href}`));
        if (matchedFile) {
          return (
            <Button
              variant="link"
              onClick={() => setViewingFile(matchedFile)}
              className="link-subtle inline-flex items-center gap-0.5"
              style={{ font: 'inherit' }}
            >
              {children}
            </Button>
          );
        }
      }

      return (
        <a href={href} target="_blank" rel="noopener noreferrer" {...props}>
          {children}
        </a>
      );
    },
  }), [files, resource, skillMaps.byFlat, skillMaps.byName]);

  function nextCustomEntryId() {
    customEntryIdRef.current += 1;
    return `custom-entry-${customEntryIdRef.current}`;
  }

  function syncWorkspaceStateFromContent(content: string) {
    const canonicalSlots = buildWorkspaceSlotsFromContent(content);
    const canonicalKeySet = new Set(canonicalSlots.map((slot) => slot.key));
    const nextInputValues = frontmatterInputValuesRef.current;
    const preservedSlots = workspaceSlotsRef.current.filter((slot) => {
      const inputState = nextInputValues[slot.id];
      const displayKey = (inputState?.key ?? slot.key).trim();

      if (displayKey.length === 0) {
        return true;
      }

      return !canonicalKeySet.has(displayKey);
    });

    const seenIds = new Set<string>();
    const merged: WorkspaceSlotState[] = [];
    for (const slot of [...canonicalSlots, ...preservedSlots]) {
      if (seenIds.has(slot.id)) continue;
      seenIds.add(slot.id);
      merged.push(slot);
    }
    workspaceSlotsRef.current = merged;
    setWorkspaceSlots(merged);
  }

  function resetWorkspaceStateFromContent(content: string) {
    const nextSlots = buildWorkspaceSlotsFromContent(content);
    workspaceSlotsRef.current = nextSlots;
    setWorkspaceSlots(nextSlots);
  }

  function clearAutosaveTimer() {
    if (autosaveTimerRef.current) {
      clearTimeout(autosaveTimerRef.current);
      autosaveTimerRef.current = null;
    }
  }

  function queueSaveStateReset() {
    if (saveStatusTimerRef.current) {
      clearTimeout(saveStatusTimerRef.current);
    }
    saveStatusTimerRef.current = setTimeout(() => {
      setSaveState((current) => (current === 'saved' ? 'idle' : current));
    }, FRONTMATTER_STATUS_RESET_MS);
  }

  async function persistQueuedContent() {
    if (!canEditPrimaryContent || !name) return;
    if (hasFrontmatterValidationErrorsRef.current || isSavingRef.current) return;

    const nextContent = queuedAutosaveContentRef.current ?? draftContentRef.current;
    if (nextContent === savedContentRef.current) {
      pendingSuccessToastRef.current = false;
      if (!saveError) {
        setSaveState('saved');
        queueSaveStateReset();
      }
      return;
    }

    const requestContent = nextContent;
    setIsSaving(true);
    isSavingRef.current = true;
    setSaveState('saving');
    setSaveError(null);

    try {
      const response = await api.saveSkillFile(currentResource.flatName, 'SKILL.md', requestContent);
      pendingSavedContentRef.current = response.content;
      savedContentRef.current = response.content;
      setSavedContent(response.content);

      if (draftContentRef.current === requestContent) {
        draftContentRef.current = response.content;
        setDraftContent(response.content);
        setEditorSurface('rich');
        syncWorkspaceStateFromContent(response.content);
      }

      queuedAutosaveContentRef.current = draftContentRef.current === requestContent
        ? response.content
        : draftContentRef.current;

      setSaveState('saved');
      queueSaveStateReset();
      if (pendingSuccessToastRef.current) {
        toast('Saved', 'success');
        pendingSuccessToastRef.current = false;
      }
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name) });
    } catch (e: unknown) {
      const message = (e as Error).message;
      setSaveError(message);
      setSaveState('error');
      pendingSuccessToastRef.current = false;
      toast(message, 'error');
    } finally {
      setIsSaving(false);
      isSavingRef.current = false;

      if (
        queuedAutosaveContentRef.current
        && queuedAutosaveContentRef.current !== requestContent
        && queuedAutosaveContentRef.current !== savedContentRef.current
      ) {
        void persistQueuedContent();
      }
    }
  }

  function scheduleAutosave(nextContent: string, delay = AUTOSAVE_DELAY_MS) {
    if (!canEditPrimaryContent) return;
    queuedAutosaveContentRef.current = nextContent;
    setSaveError(null);
    if (saveState !== 'saving') {
      setSaveState('idle');
    }
    clearAutosaveTimer();
    autosaveTimerRef.current = setTimeout(() => {
      void persistQueuedContent();
    }, delay);
  }

  async function flushAutosave(nextContent = draftContentRef.current) {
    if (!canEditPrimaryContent) return;
    queuedAutosaveContentRef.current = nextContent;
    clearAutosaveTimer();
    await persistQueuedContent();
  }

  function maybeToastRecentSave() {
    if (saveState === 'saved' && draftContentRef.current === savedContentRef.current) {
      pendingSuccessToastRef.current = false;
      setSaveState('idle');
      toast('Saved', 'success');
      return true;
    }

    return false;
  }

  useEffect(() => {
    draftContentRef.current = draftContent;
  }, [draftContent]);

  useEffect(() => {
    savedContentRef.current = savedContent;
  }, [savedContent]);

  useEffect(() => {
    frontmatterInputValuesRef.current = frontmatterInputValues;
  }, [frontmatterInputValues]);

  useEffect(() => {
    workspaceSlotsRef.current = workspaceSlots;
  }, [workspaceSlots]);

  useEffect(() => {
    hasFrontmatterValidationErrorsRef.current = hasFrontmatterValidationErrors;
  }, [hasFrontmatterValidationErrors]);

  useEffect(() => {
    isSavingRef.current = isSaving;
  }, [isSaving]);

  useEffect(() => {
    const textarea = descriptionTextareaRef.current;
    if (!textarea) return;

    textarea.style.height = '0px';
    textarea.style.height = `${textarea.scrollHeight}px`;
  }, [parsedDoc.manifest.description]);

  useEffect(() => () => {
    clearAutosaveTimer();
    if (saveStatusTimerRef.current) {
      clearTimeout(saveStatusTimerRef.current);
    }
  }, []);

  useEffect(() => {
    if (!resource) return;

    const resourceKey = `${resource.kind}:${resource.flatName}`;
    const resourceChanged = lastLoadedResourceRef.current !== resourceKey;
    lastLoadedResourceRef.current = resourceKey;

    if (resourceChanged) {
      clearAutosaveTimer();
      pendingSavedContentRef.current = null;
      pendingSuccessToastRef.current = false;
      lastAppliedQueryContentRef.current = skillMdContent;
      savedContentRef.current = skillMdContent;
      draftContentRef.current = skillMdContent;
      setSavedContent(skillMdContent);
      setDraftContent(skillMdContent);
      setEditorSurface('rich');
      setSaveError(null);
      setSaveState('idle');
      setShowFrontmatterGuide(false);
      setFrontmatterInputValues({});
      setFrontmatterErrors({});
      resetWorkspaceStateFromContent(skillMdContent);
      return;
    }

    if (pendingSavedContentRef.current) {
      if (skillMdContent === pendingSavedContentRef.current) {
        pendingSavedContentRef.current = null;
        lastAppliedQueryContentRef.current = skillMdContent;
      }
      return;
    }

    if (!hasUnsavedChanges && skillMdContent !== lastAppliedQueryContentRef.current) {
      lastAppliedQueryContentRef.current = skillMdContent;
      savedContentRef.current = skillMdContent;
      draftContentRef.current = skillMdContent;
      setSavedContent(skillMdContent);
      setDraftContent(skillMdContent);
      setSaveError(null);
      setSaveState('idle');
      setFrontmatterInputValues({});
      setFrontmatterErrors({});
      resetWorkspaceStateFromContent(skillMdContent);
    }
  }, [hasUnsavedChanges, resource, savedContent, skillMdContent]);

  useBeforeUnload(
    (event) => {
      if (!hasUnsavedChanges) return;
      event.preventDefault();
      event.returnValue = '';
    },
    { capture: true },
  );

  if (isPending) return <SkillDetailSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="py-8 text-center">
        <p className="text-lg text-danger">Failed to load resource</p>
        <p className="mt-1 text-sm text-pencil-light">{error.message}</p>
      </Card>
    );
  }
  if (!resource) return null;
  const currentResource = resource;
  function resolveFileResource(filePath: string): Skill | undefined {
    if (/\.[a-z]+$/i.test(filePath) && !filePath.endsWith('.md')) return undefined;
    const flat = `${currentResource.flatName}__${filePath.replace(/\//g, '__')}`;
    return skillMaps.byFlat.get(flat);
  }

  function clearFrontmatterError(entryId: string) {
    setFrontmatterErrors((current) => {
      if (!(entryId in current)) {
        return current;
      }

      const next = { ...current };
      delete next[entryId];
      return next;
    });
  }

  function clearFrontmatterInput(entryId: string) {
    setFrontmatterInputValues((current) => {
      if (!(entryId in current)) {
        return current;
      }

      const next = { ...current };
      delete next[entryId];
      frontmatterInputValuesRef.current = next;
      return next;
    });
  }

  function updateWorkspaceSlot(entryId: string, nextKey: string) {
    setWorkspaceSlots((current) => {
      const hasEntry = current.some((entry) => entry.id === entryId);
      if (!hasEntry) {
        const next = [...current, { id: entryId, key: nextKey }];
        workspaceSlotsRef.current = next;
        return next;
      }

      const next = current.map((entry) => (
        entry.id === entryId ? { ...entry, key: nextKey } : entry
      ));
      workspaceSlotsRef.current = next;
      return next;
    });
  }

  function removeWorkspaceSlot(entryId: string) {
    setWorkspaceSlots((current) => {
      const next = current.filter((entry) => entry.id !== entryId);
      workspaceSlotsRef.current = next;
      return next;
    });
  }

  function handleManifestFieldChange(key: 'name' | 'description', rawValue: string) {
    const nextContent = updateSkillFrontmatterField(draftContent, key, rawValue, parsedDoc.frontmatter[key]);
    draftContentRef.current = nextContent;
    setDraftContent(nextContent);
    setSaveError(null);
    scheduleAutosave(nextContent);
  }

  function handleManifestFieldBlur() {
    if (maybeToastRecentSave()) {
      return;
    }
    pendingSuccessToastRef.current = true;
    void flushAutosave();
  }

  function handleFrontmatterValueChange(entryId: string, rawValue: string) {
    const workspaceEntry = workspaceSlotEntries.find((entry) => entry.id === entryId);
    const key = (frontmatterInputValues[entryId]?.key ?? workspaceEntry?.key ?? '').trim();

    setFrontmatterInputValues((current) => {
      const next = mergeFrontmatterInputState(current, entryId, { value: rawValue });
      frontmatterInputValuesRef.current = next;
      return next;
    });

    if (!key) {
      if (rawValue.trim().length === 0) {
        clearFrontmatterError(entryId);
      } else {
        setFrontmatterErrors((current) => ({ ...current, [entryId]: 'Custom frontmatter key is required.' }));
      }
      return;
    }

    try {
      const nextContent = updateSkillFrontmatterField(draftContent, key, rawValue, parsedDoc.frontmatter[key]);
      draftContentRef.current = nextContent;
      setDraftContent(nextContent);
      setSaveError(null);
      scheduleAutosave(nextContent);
      updateWorkspaceSlot(entryId, key);
      clearFrontmatterError(entryId);
    } catch (error) {
      const message = error instanceof Error ? error.message : `Invalid ${key} value.`;
      setFrontmatterErrors((current) => ({ ...current, [entryId]: message }));
    }
  }

  function handleFrontmatterFieldBlur(entryId: string) {
    if (frontmatterErrors[entryId]) {
      return;
    }

    if (maybeToastRecentSave()) {
      return;
    }
    pendingSuccessToastRef.current = true;
    void flushAutosave();
  }

  function handleAddBuiltInField(key: string) {
    const entryId = nextCustomEntryId();
    updateWorkspaceSlot(entryId, key);
    setFrontmatterInputValues((current) => {
      const next = mergeFrontmatterInputState(current, entryId, buildWorkspaceInputMap(key, ''));
      frontmatterInputValuesRef.current = next;
      return next;
    });
    clearFrontmatterError(entryId);
  }

  function handleAddCustomField() {
    const id = nextCustomEntryId();
    updateWorkspaceSlot(id, '');
    setFrontmatterInputValues((current) => {
      const next = mergeFrontmatterInputState(current, id, buildWorkspaceInputMap('', ''));
      frontmatterInputValuesRef.current = next;
      return next;
    });
    clearFrontmatterError(id);
  }

  function handleRemoveFrontmatterField(entryId: string) {
    const workspaceEntry = workspaceSlotEntries.find((entry) => entry.id === entryId);
    if (!workspaceEntry) {
      return;
    }

    const currentKey = (frontmatterInputValues[entryId]?.key ?? workspaceEntry.key).trim();
    const nextContent = currentKey
      ? updateSkillFrontmatterField(draftContent, currentKey, '', parsedDoc.frontmatter[currentKey])
      : draftContent;
    draftContentRef.current = nextContent;
    setDraftContent(nextContent);
    if (nextContent !== draftContent) {
      scheduleAutosave(nextContent);
    }
    removeWorkspaceSlot(entryId);
    clearFrontmatterInput(entryId);
    clearFrontmatterError(entryId);
    setSaveError(null);
  }

  function handleFrontmatterFieldKeyChange(entryId: string, nextKeyInput: string) {
    const workspaceEntry = workspaceSlotEntries.find((entry) => entry.id === entryId);
    if (!workspaceEntry) {
      return;
    }

    setFrontmatterInputValues((current) => {
      const next = mergeFrontmatterInputState(current, entryId, { key: nextKeyInput });
      frontmatterInputValuesRef.current = next;
      return next;
    });

    const currentKey = (frontmatterInputValues[entryId]?.key ?? workspaceEntry.key).trim();
    const pendingValue = frontmatterInputValues[entryId]?.value
      ?? serializeFrontmatterEditorValue(currentKey ? parsedDoc.frontmatter[currentKey] : undefined);
    const nextKey = nextKeyInput.trim();
    if (nextKey.length === 0) {
      updateWorkspaceSlot(entryId, '');
      if (pendingValue.trim().length > 0 || currentKey.length > 0) {
        setFrontmatterErrors((current) => ({ ...current, [entryId]: 'Frontmatter key is required.' }));
      } else {
        clearFrontmatterError(entryId);
      }
      return;
    }

    const hasDuplicateKey = workspaceSlotEntries.some((entry) => (
      entry.id !== entryId
      && (frontmatterInputValues[entry.id]?.key ?? entry.key).trim() === nextKey
    ));
    if (hasDuplicateKey) {
      setFrontmatterErrors((current) => ({ ...current, [entryId]: `${nextKey} already exists.` }));
      return;
    }

    try {
      let nextContent = draftContent;
      if (currentKey && Object.prototype.hasOwnProperty.call(parsedDoc.frontmatter, currentKey)) {
        nextContent = renameSkillFrontmatterField(draftContent, currentKey, nextKey);
      } else if (pendingValue.trim().length > 0) {
        nextContent = updateSkillFrontmatterField(draftContent, nextKey, pendingValue, parsedDoc.frontmatter[nextKey]);
      }

      updateWorkspaceSlot(entryId, nextKey);
      draftContentRef.current = nextContent;
      setDraftContent(nextContent);
      if (nextContent !== draftContent) {
        scheduleAutosave(nextContent);
      }
      clearFrontmatterError(entryId);
      setSaveError(null);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Invalid frontmatter key.';
      setFrontmatterErrors((current) => ({ ...current, [entryId]: message }));
    }
  }

  async function handleDelete() {
    setDeleting(true);
    try {
      if (currentResource.isInRepo) {
        const repoName = currentResource.relPath.split('/')[0];
        await api.deleteRepo(repoName);
        toast(`Repository "${formatTrackedRepoName(repoName)}" uninstalled.`, 'success');
      } else {
        await api.deleteResource(currentResource.flatName, currentResource.kind);
        toast(`${currentResource.kind === 'agent' ? 'Agent' : 'Skill'} "${currentResource.name}" uninstalled.`, 'success');
      }
      clearAuditCache(queryClient);
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      await queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      navigate('/resources');
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
      setDeleting(false);
      setConfirmDelete(false);
    }
  }

  async function handleUpdate(skipAudit = false) {
    setUpdating(true);
    setBlockedMessage(null);
    try {
      const resourceName = currentResource.isInRepo
        ? currentResource.relPath.split('/')[0]
        : currentResource.kind === 'agent'
          ? currentResource.flatName
          : currentResource.relPath;
      const res = await api.update({ name: resourceName, kind: currentResource.kind, skipAudit });
      const item = res.results[0];
      if (item?.action === 'updated') {
        const auditInfo = item.auditRiskLabel
          ? ` · Security: ${item.auditRiskLabel.toUpperCase()}${item.auditRiskScore ? ` (${item.auditRiskScore}/100)` : ''}`
          : '';
        toast(`Updated: ${formatSkillDisplayName(item.name)} — ${item.message}${auditInfo}`, 'success');
        clearAuditCache(queryClient);
        await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name!) });
        await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
        await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      } else if (item?.action === 'up-to-date') {
        toast(`${formatSkillDisplayName(item.name)} is already up to date.`, 'info');
      } else if (item?.action === 'blocked') {
        setBlockedMessage(item.message ?? 'Blocked by security audit — HIGH/CRITICAL findings detected');
      } else if (item?.action === 'error') {
        toast(item.message ?? 'Update failed', 'error');
      } else {
        toast(item?.message ?? 'Skipped', 'warning');
      }
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setUpdating(false);
    }
  }

  async function handleToggleDisabled() {
    setToggling(true);
    try {
      if (currentResource.disabled) {
        await api.enableResource(currentResource.flatName, currentResource.kind);
        toast(`Enabled: ${currentResource.name}`, 'success');
      } else {
        await api.disableResource(currentResource.flatName, currentResource.kind);
        toast(`Disabled: ${currentResource.name}`, 'success');
      }
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.detail(name!) });
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setToggling(false);
    }
  }

  return (
    <div className="animate-fade-in">
      <div className="sticky top-0 z-20 -mx-4 -mt-3 mb-2 flex items-center gap-3 bg-paper px-4 py-3 md:-mx-8 md:px-8">
        <IconButton
          icon={<ArrowLeft size={18} strokeWidth={2.5} />}
          label="Back to resources"
          size="lg"
          variant="outline"
          onClick={() => navigate('/resources')}
          className="bg-surface"
          style={{ boxShadow: shadows.sm }}
        />
        <div className="flex flex-wrap items-center gap-3">
          <h2 className="ss-detail-title text-2xl font-bold text-pencil md:text-3xl">
            {currentResource.name}
          </h2>
          <KindBadge kind={currentResource.kind} />
          {currentResource.disabled ? <Badge variant="danger">Disabled</Badge> : null}
          <SourceBadge type={currentResource.type} isInRepo={currentResource.isInRepo} />
          {currentResource.targets && currentResource.targets.length > 0 ? (
            <span className="inline-flex items-center gap-1">
              <Target size={13} strokeWidth={2.5} className="text-pencil-light" />
              {currentResource.targets.map((target) => (
                <Badge key={target} variant="default">{target}</Badge>
              ))}
            </span>
          ) : null}
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <Card>
            {currentResource.kind === 'skill' ? (
              <div
                className="ss-detail-manifest mb-4 border-2 border-dashed border-pencil-light/30 p-4 pt-5"
                style={{ borderRadius: radius.sm }}
              >
                <div className="space-y-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="text-sm uppercase tracking-wide text-muted-dark">Name</div>
                      <Input
                        aria-label="Skill name"
                        value={parsedDoc.manifest.name ?? ''}
                        onChange={(event) => handleManifestFieldChange('name', event.currentTarget.value)}
                        onBlur={handleManifestFieldBlur}
                        placeholder="Skill name"
                        uiSize="sm"
                        className="ss-detail-manifest-input mt-1 text-xl font-bold"
                      />
                    </div>
                  </div>

                  <div className="min-w-0">
                    <div className="text-sm uppercase tracking-wide text-muted-dark">Description</div>
                    <Textarea
                      ref={descriptionTextareaRef}
                      aria-label="Skill description"
                      value={parsedDoc.manifest.description ?? ''}
                      onChange={(event) => handleManifestFieldChange('description', event.currentTarget.value)}
                      onBlur={handleManifestFieldBlur}
                      placeholder="Describe when this skill should be used."
                      rows={1}
                      className="ss-detail-manifest-textarea mt-1 resize-none overflow-hidden"
                    />
                  </div>
                </div>
              </div>
            ) : (parsedDoc.manifest.name || parsedDoc.manifest.description || parsedDoc.manifest.license) ? (
              <div
                className="ss-detail-manifest mb-4 border-2 border-dashed border-pencil-light/30 p-4 pt-5"
                style={{ borderRadius: radius.sm }}
              >
                <dl className="space-y-2">
                  {parsedDoc.manifest.name ? (
                    <div>
                      <dt className="text-sm uppercase tracking-wide text-muted-dark">Name</dt>
                      <dd className="text-xl font-bold text-pencil">{parsedDoc.manifest.name}</dd>
                    </div>
                  ) : null}
                  {parsedDoc.manifest.description ? (
                    <div>
                      <dt className="text-sm uppercase tracking-wide text-muted-dark">Description</dt>
                      <dd className="text-base text-pencil">{parsedDoc.manifest.description}</dd>
                    </div>
                  ) : null}
                  {parsedDoc.manifest.license ? (
                    <div>
                      <dt className="text-sm uppercase tracking-wide text-muted-dark">License</dt>
                      <dd className="text-base text-pencil">{parsedDoc.manifest.license}</dd>
                    </div>
                  ) : null}
                </dl>
              </div>
            ) : null}

            {canEditPrimaryContent ? (
              <>
                <ContentStatsBar
                  content={previewSource}
                  description={parsedDoc.manifest.description}
                  body={parsedDoc.markdown}
                  fileCount={files.length}
                  license={parsedDoc.manifest.license}
                />

                <div className="ss-detail-content-boundary mb-4 mt-4 flex flex-wrap items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="text-sm uppercase tracking-wide text-muted-dark">File</div>
                    <div className="text-base font-semibold text-pencil">SKILL.md</div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => setShowFrontmatterGuide((current) => !current)}
                    >
                      {showFrontmatterGuide ? <ChevronUp size={14} strokeWidth={2.5} /> : <ChevronDown size={14} strokeWidth={2.5} />}
                      {showFrontmatterGuide ? 'Hide frontmatter' : 'Show frontmatter'}
                    </Button>
                  </div>
                </div>

                {showFrontmatterGuide ? (
                  <div className="mb-6 space-y-4 border-b border-dashed border-pencil-light/30 pb-6">
                    {!hasConfiguredFrontmatter ? (
                      <p className="rounded-[var(--radius-sm)] border border-muted/80 bg-paper/60 px-3 py-2 text-sm text-pencil-light">
                        No frontmatter fields are set yet. The workspace below lets you add built-in or custom entries without leaving the page.
                      </p>
                    ) : null}

                    <SkillFrontmatterWorkspace
                      mode="edit"
                      builtInEntries={workspaceBuiltInEntries}
                      additionalEntries={workspaceAdditionalEntries}
                      onAddBuiltInField={handleAddBuiltInField}
                      onAddCustomField={handleAddCustomField}
                      onRemoveField={handleRemoveFrontmatterField}
                      onChangeFieldValue={handleFrontmatterValueChange}
                      onChangeFieldKey={handleFrontmatterFieldKeyChange}
                      onBlurFieldValue={handleFrontmatterFieldBlur}
                      onBlurFieldKey={handleFrontmatterFieldBlur}
                    />
                  </div>
                ) : null}

                {saveError ? (
                  <div
                    role="alert"
                    className="mb-4 rounded-[var(--radius-md)] border border-danger/40 bg-danger/5 px-4 py-3 text-sm text-danger"
                  >
                    {saveError}
                  </div>
                ) : null}

                <div
                  className={editorShellClassName(isSaving)}
                  aria-busy={isSaving}
                  onBlurCapture={(event) => {
                    if (event.currentTarget.contains(event.relatedTarget as Node | null)) {
                      return;
                    }
                    if (maybeToastRecentSave()) {
                      return;
                    }
                    pendingSuccessToastRef.current = true;
                    void flushAutosave();
                  }}
                >
                  <SkillMarkdownEditor
                    value={draftContent}
                    onChange={(next) => {
                      draftContentRef.current = next;
                      setDraftContent(next);
                      syncWorkspaceStateFromContent(next);
                      setSaveError(null);
                      scheduleAutosave(next);
                    }}
                    onSave={() => {
                      void flushAutosave();
                    }}
                    onDiscard={() => {
                      void flushAutosave();
                    }}
                    onSurfaceChange={setEditorSurface}
                    surface={editorSurface}
                    mode="edit"
                    isDirty={Boolean(isDirty)}
                    showToolbar={false}
                    surfaceStyle="inline"
                  />
                </div>
              </>
            ) : (
              <>
                <ContentStatsBar
                  content={previewSource}
                  description={parsedDoc.manifest.description}
                  body={parsedDoc.markdown}
                  fileCount={files.length}
                  license={parsedDoc.manifest.license}
                />

                <div className="prose-hand mt-4">
                {renderedMarkdown ? (
                  <Markdown remarkPlugins={[remarkGfm]} components={mdComponents}>
                    {renderedMarkdown}
                  </Markdown>
                ) : (
                  <p className="py-8 text-center italic text-pencil-light">
                    No content available.
                  </p>
                )}
                </div>
              </>
            )}
          </Card>
        </div>

        <div className="space-y-5 lg:sticky lg:top-16 lg:max-h-[calc(100vh-5rem)] lg:self-start lg:overflow-y-auto lg:-mr-2 lg:pr-2">
          <Card className="ss-detail-pinned" overflow>
            <h3 className="ss-detail-heading mb-3 font-bold text-pencil">Metadata</h3>
            <dl className="space-y-2">
              <MetaItem
                label="Path"
                value={currentResource.relPath}
                mono
                copyable
                copyValue={currentResource.sourcePath}
                copiedLabel="Path copied!"
              />
              {currentResource.source ? <MetaItem label="Source" value={currentResource.source} mono /> : null}
              {currentResource.version ? <MetaItem label="Version" value={currentResource.version} mono /> : null}
              {currentResource.branch ? <MetaItem label="Branch" value={currentResource.branch} mono /> : null}
              {currentResource.installedAt ? (
                <MetaItem
                  label="Installed"
                  value={new Date(currentResource.installedAt).toLocaleDateString()}
                />
              ) : null}
              {currentResource.targets && currentResource.targets.length > 0 ? (
                <div className="flex items-baseline gap-3">
                  <dt className="min-w-[4.5rem] shrink-0 text-xs uppercase tracking-wider text-pencil-light">Targets</dt>
                  <dd className="flex min-w-0 flex-wrap gap-1.5">
                    {currentResource.targets.map((target) => (
                      <Badge key={target} variant="default">{target}</Badge>
                    ))}
                  </dd>
                </div>
              ) : null}
              {currentResource.repoUrl ? (
                <div className="flex items-baseline gap-3">
                  <dt className="min-w-[4.5rem] shrink-0 text-xs uppercase tracking-wider text-pencil-light">Repo</dt>
                  <dd className="min-w-0">
                    <a
                      href={currentResource.repoUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="link-subtle break-all text-sm"
                    >
                      <ExternalLink size={11} strokeWidth={2.5} className="mr-0.5 inline -mt-0.5" />
                      {currentResource.repoUrl.replace('https://', '').replace('.git', '')}
                    </a>
                  </dd>
                </div>
              ) : null}
            </dl>

            <div className="mt-4 flex flex-col gap-2 border-t border-dashed border-pencil-light/30 pt-4">
              <div className="flex gap-2">
                <Button
                  onClick={handleToggleDisabled}
                  disabled={toggling}
                  variant={currentResource.disabled ? 'primary' : 'secondary'}
                  size="sm"
                  className="flex-1"
                >
                  {toggling ? (
                    <Spinner size="sm" />
                  ) : currentResource.disabled ? (
                    <Eye size={14} strokeWidth={2.5} />
                  ) : (
                    <EyeOff size={14} strokeWidth={2.5} />
                  )}
                  {toggling
                    ? (currentResource.disabled ? 'Enabling...' : 'Disabling...')
                    : (currentResource.disabled ? 'Enable' : 'Disable')}
                </Button>
                {currentResource.isInRepo || currentResource.source ? (
                  <Button
                    onClick={() => handleUpdate()}
                    disabled={updating}
                    variant="secondary"
                    size="sm"
                    className="flex-1"
                  >
                    {updating ? <Spinner size="sm" /> : <RefreshCw size={14} strokeWidth={2.5} />}
                    {updating ? 'Updating...' : 'Update'}
                  </Button>
                ) : null}
              </div>
              <Button
                onClick={() => setConfirmDelete(true)}
                disabled={deleting}
                variant="danger"
                size="sm"
              >
                <Trash2 size={12} strokeWidth={2.5} />
                {deleting
                  ? 'Uninstalling...'
                  : currentResource.isInRepo
                    ? 'Uninstall Repo'
                    : 'Uninstall'}
              </Button>
            </div>
          </Card>

          {currentResource.kind !== 'agent' ? (
            <Card className="ss-detail-pinned" overflow>
              <h3 className="ss-detail-heading mb-3 flex items-center gap-2 font-bold text-pencil">
                <FileText size={16} strokeWidth={2.5} />
                Files ({files.length})
              </h3>
              {files.length > 0 ? (
                <ul className="max-h-80 space-y-1.5 overflow-y-auto">
                  {files.map((file) => {
                    const linkedResource = resolveFileResource(file);
                    const isSkillMd = file === 'SKILL.md';
                    const { icon: FileIcon, className: iconClass } = getFileIcon(file);

                    return (
                      <li
                        key={file}
                        className="flex items-center gap-2 truncate text-sm text-pencil-light"
                      >
                        <FileIcon size={14} strokeWidth={2} className={`shrink-0 ${iconClass}`} />
                        {linkedResource ? (
                          <Link
                            to={`/resources/${encodeURIComponent(linkedResource.flatName)}`}
                            className="font-mono link-subtle inline-flex items-center gap-1"
                            style={{ fontSize: '0.8125rem' }}
                            title={`View skill: ${linkedResource.name}`}
                          >
                            {file}
                            <ArrowUpRight size={11} strokeWidth={2.5} className="shrink-0" />
                          </Link>
                        ) : isSkillMd ? (
                          <span className="font-mono truncate">{file}</span>
                        ) : (
                          <Button
                            variant="link"
                            onClick={() => setViewingFile(file)}
                            className="font-mono link-subtle text-left truncate inline-flex items-center gap-1"
                            style={{ fontSize: '0.8125rem' }}
                            title={`View file: ${file}`}
                            aria-label={`Preview file ${file}`}
                          >
                            {file}
                          </Button>
                        )}
                      </li>
                    );
                  })}
                </ul>
              ) : (
                <p className="text-sm italic text-muted-dark">No files.</p>
              )}
            </Card>
          ) : null}

          <SecurityAuditCard auditQuery={auditQuery} />
          <TargetDistribution flatName={currentResource.flatName} kind={currentResource.kind} />
          <SyncStatusCard diffQuery={diffQuery} skillFlatName={currentResource.flatName} />
        </div>
      </div>

      {viewingFile ? (
        <Suspense fallback={null}>
          <FileViewerModal
            skillName={currentResource.flatName}
            filepath={viewingFile}
            sourcePath={currentResource.sourcePath}
            resourceKind={currentResource.kind}
            onClose={() => setViewingFile(null)}
          />
        </Suspense>
      ) : null}

      <ConfirmDialog
        open={blockedMessage !== null}
        title="Blocked by Security Audit"
        message={
          <>
            <p className="mb-2 text-sm text-danger">{blockedMessage}</p>
            <p className="text-sm text-pencil-light">Skip the audit and apply the update anyway?</p>
          </>
        }
        confirmText="Skip Audit & Update"
        variant="danger"
        loading={updating}
        onConfirm={() => {
          setBlockedMessage(null);
          void handleUpdate(true);
        }}
        onCancel={() => setBlockedMessage(null)}
      />

      <ConfirmDialog
        open={confirmDelete}
        title={currentResource.isInRepo ? 'Uninstall Repository' : `Uninstall ${currentResource.kind === 'agent' ? 'Agent' : 'Skill'}`}
        message={
          currentResource.isInRepo
            ? `Remove repository "${currentResource.relPath.split('/')[0]}"? This will move all skills in the repo to trash.`
            : `Uninstall ${currentResource.kind === 'agent' ? 'agent' : 'skill'} "${currentResource.name}"? It will be moved to trash and can be restored within 7 days.`
        }
        confirmText="Uninstall"
        variant="danger"
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
      />

      <ConfirmDialog
        open={blocker.state === 'blocked'}
        title="Unsaved Changes"
        message="You have unsaved changes that will be lost. Discard them?"
        confirmText="Discard"
        variant="danger"
        onConfirm={() => blocker.proceed?.()}
        onCancel={() => blocker.reset?.()}
      />
    </div>
  );
}

function editorShellClassName(isSaving: boolean, stretch = false): string {
  return [
    'transition-opacity',
    stretch ? 'h-full min-h-0' : '',
    isSaving ? 'pointer-events-none opacity-60' : '',
  ].filter(Boolean).join(' ');
}

function MetaItem({
  label,
  value,
  mono,
  copyable,
  copyValue,
  copiedLabel,
}: {
  label: string;
  value: string;
  mono?: boolean;
  copyable?: boolean;
  copyValue?: string;
  copiedLabel?: string;
}) {
  return (
    <div className="flex items-baseline gap-3">
      <dt className="min-w-[4.5rem] shrink-0 text-xs uppercase tracking-wider text-pencil-light">
        {label}
      </dt>
      <dd className={`min-w-0 break-all text-sm text-pencil${mono ? ' font-mono' : ''}`}>
        {value}
        {copyable ? (
          <CopyButton
            value={copyValue ?? value}
            copiedLabel={copiedLabel}
            className="ml-1 align-middle"
          />
        ) : null}
      </dd>
    </div>
  );
}

function SecurityAuditCard({
  auditQuery,
}: {
  auditQuery: ReturnType<typeof useQuery<Awaited<ReturnType<typeof api.auditSkill>>>>;
}) {
  if (auditQuery.isPending) {
    return (
      <Card variant="outlined">
        <div className="flex animate-pulse items-center gap-2">
          <ShieldCheck size={16} strokeWidth={2.5} className="text-pencil-light" />
          <span className="text-sm text-pencil-light">Scanning security...</span>
        </div>
      </Card>
    );
  }

  if (auditQuery.error || !auditQuery.data) return null;

  const { result } = auditQuery.data;
  const findingCounts = result.findings.reduce(
    (acc, finding) => {
      acc[finding.severity] = (acc[finding.severity] || 0) + 1;
      return acc;
    },
    {} as Record<string, number>,
  );

  return (
    <Card variant="outlined" className="ss-detail-pinned ss-detail-pinned-green">
      <h3 className="ss-detail-heading mb-3 flex items-center gap-2 font-bold text-pencil">
        <ShieldCheck size={16} strokeWidth={2.5} />
        Security
      </h3>
      <div className="space-y-3">
        <div className="flex flex-wrap items-stretch gap-2">
          <BlockStamp isBlocked={result.isBlocked} />
          <RiskMeter riskLabel={result.riskLabel} riskScore={result.riskScore} />
        </div>
        {result.findings.length > 0 ? (
          <div className="flex flex-wrap gap-1.5 border-t border-dashed border-pencil-light/30 pt-2">
            {Object.entries(findingCounts)
              .sort(([a], [b]) => sevOrder(a) - sevOrder(b))
              .map(([severity, count]) => (
                <Badge key={severity} variant={severityBadgeVariant(severity)}>
                  {count} {severity}
                </Badge>
              ))}
          </div>
        ) : (
          <p className="text-sm text-success">No security issues detected</p>
        )}
      </div>
    </Card>
  );
}

function sevOrder(severity: string): number {
  switch (severity) {
    case 'CRITICAL': return 0;
    case 'HIGH': return 1;
    case 'MEDIUM': return 2;
    case 'LOW': return 3;
    case 'INFO': return 4;
    default: return 5;
  }
}

function TargetDistribution({ flatName, kind }: { flatName: string; kind: 'skill' | 'agent' }) {
  const { getSkillTargets } = useSyncMatrix();
  const entries = getSkillTargets(flatName);

  if (entries.length === 0) return null;

  return (
    <Card className="ss-detail-pinned ss-detail-pinned-blue">
      <h3 className="ss-detail-heading mb-3 flex items-center gap-2 font-bold text-pencil">
        <Target size={16} strokeWidth={2.5} />
        Target Distribution
      </h3>
      <div className="space-y-3">
        {entries.map((entry) => (
          <div key={entry.target} className="border-b border-dashed border-pencil-light/30 pb-2 text-sm last:border-0 last:pb-0">
            <div className="flex items-center gap-2">
              <span
                className={`h-2 w-2 shrink-0 rounded-full ${
                  entry.status === 'synced' ? 'bg-success' :
                  entry.status === 'skill_target_mismatch' ? 'bg-warning' :
                  entry.status === 'na' ? 'bg-muted' : 'bg-danger'
                }`}
              />
              <Link
                to={`/targets/${encodeURIComponent(entry.target)}/filters?kind=${kind}`}
                className="truncate font-bold text-pencil hover:text-blue"
              >
                {entry.target}
              </Link>
            </div>
            <div className="mt-1 flex items-center justify-between pl-4">
              <span
                className={`text-xs ${
                  entry.status === 'synced' ? 'text-success' :
                  entry.status === 'skill_target_mismatch' ? 'text-warning' :
                  entry.status === 'na' ? 'text-muted-dark' : 'text-danger'
                }`}
              >
                {entry.status === 'synced' && '\u2713 Synced'}
                {entry.status === 'excluded' && `\u2717 Excluded (${entry.reason})`}
                {entry.status === 'not_included' && '\u2717 Not included'}
                {entry.status === 'skill_target_mismatch' && `Targets: ${entry.reason}`}
                {entry.status === 'na' && '\u2014 Symlink mode'}
              </span>
            </div>
          </div>
        ))}
      </div>
      <p className="mt-3 text-xs text-pencil-light">
        Filters only apply to merge/copy mode targets.{' '}
        <Link to="/targets" className="text-blue hover:underline">Manage targets →</Link>
      </p>
    </Card>
  );
}

function SyncStatusCard({
  diffQuery,
  skillFlatName,
}: {
  diffQuery: ReturnType<typeof useQuery<Awaited<ReturnType<typeof api.diff>>>>;
  skillFlatName: string;
}) {
  if (diffQuery.isPending || !diffQuery.data) return null;

  const targetStatuses: { name: string; status: 'linked' | 'missing' | 'excluded' | 'conflict' }[] = [];

  for (const diffTarget of diffQuery.data.diffs) {
    const item = diffTarget.items.find((diffItem) => diffItem.skill === skillFlatName);
    if (item) {
      const status = item.action === 'ok' || item.action === 'linked'
        ? 'linked'
        : item.action === 'excluded'
          ? 'excluded'
          : item.action === 'conflict' || item.action === 'broken'
            ? 'conflict'
            : 'missing';
      targetStatuses.push({ name: diffTarget.target, status });
      continue;
    }

    targetStatuses.push({ name: diffTarget.target, status: 'linked' });
  }

  if (targetStatuses.length === 0) return null;

  const statusDot: Record<string, string> = {
    linked: 'bg-success',
    missing: 'bg-warning',
    conflict: 'bg-danger',
    excluded: 'bg-muted-dark',
  };

  const statusLabel: Record<string, string> = {
    linked: 'linked',
    missing: 'not synced',
    conflict: 'conflict',
    excluded: 'excluded',
  };

  return (
    <Card variant="outlined" className="ss-detail-pinned ss-detail-pinned-cyan">
      <h3 className="ss-detail-heading mb-3 flex items-center gap-2 font-bold text-pencil">
        <Link2 size={16} strokeWidth={2.5} />
        Target Sync
      </h3>
      <ul className="space-y-1.5">
        {targetStatuses.map((target) => (
          <li key={target.name} className="flex items-center gap-2 text-sm">
            <span className={`h-2 w-2 shrink-0 rounded-full ${statusDot[target.status]}`} />
            <span className="font-mono text-[0.8125rem] font-medium text-pencil">{target.name}</span>
            <span className="text-xs text-pencil-light">{statusLabel[target.status]}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}
