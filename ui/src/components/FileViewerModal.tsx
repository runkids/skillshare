import { Component, Suspense, lazy, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { ChevronDown, ChevronUp, X } from 'lucide-react';
import CodeMirror from '@uiw/react-codemirror';
import { json } from '@codemirror/lang-json';
import { yaml } from '@codemirror/lang-yaml';
import { python } from '@codemirror/lang-python';
import { javascript } from '@codemirror/lang-javascript';
import { EditorView } from '@codemirror/view';
import CopyButton from './CopyButton';
import Button from './Button';
import ContentStatsBar from './ContentStatsBar';
import IconButton from './IconButton';
import SkillFrontmatterWorkspace, { type FrontmatterWorkspaceEntry } from './SkillFrontmatterWorkspace';
import Spinner from './Spinner';
import DialogShell from './DialogShell';
import ConfirmDialog from './ConfirmDialog';
import { api, type SkillFileContent } from '../api/client';
import { handTheme } from '../lib/codemirror-theme';
import { getAdditionalFrontmatterEntries, getReferenceFrontmatterEntries, SKILL_FRONTMATTER_FIELDS } from '../lib/skillFrontmatter';
import {
  parseSkillMarkdown,
  renameSkillFrontmatterField,
  serializeFrontmatterEditorValue,
  updateSkillFrontmatterField,
} from '../lib/skillMarkdown';
import { Textarea } from './Input';
import type { SkillMarkdownEditorSurface } from './SkillMarkdownEditor';
import { useToast } from './Toast';

const SkillMarkdownEditor = lazy(() => import('./SkillMarkdownEditor'));

type SaveIndicatorState = 'idle' | 'saving' | 'saved' | 'error';
type FrontmatterCardInputState = {
  key?: string;
  value?: string;
};

type FrontmatterCardInputMap = Record<string, FrontmatterCardInputState>;

type WorkspaceSlotState = {
  id: string;
  key: string;
};

const AUTOSAVE_DELAY_MS = 250;
const SAVE_STATUS_RESET_MS = 1800;
const BOOLEAN_FRONTMATTER_KEYS = new Set(['disable-model-invocation', 'user-invocable']);
const STRUCTURED_FRONTMATTER_KEYS = new Set(['hooks', 'metadata']);
const BUILT_IN_FRONTMATTER_ORDER = SKILL_FRONTMATTER_FIELDS.map((field) => field.key);

function getBuiltInFrontmatterKeys(frontmatter: Record<string, unknown>) {
  return getReferenceFrontmatterEntries(frontmatter)
    .filter((entry) => entry.isSet)
    .map((entry) => entry.key);
}

function buildWorkspaceInputMap(key: string, value = ''): FrontmatterCardInputState {
  return { key, value };
}

function orderBuiltInKeys(keys: Iterable<string>) {
  const keySet = new Set(keys);
  return BUILT_IN_FRONTMATTER_ORDER.filter((key) => keySet.has(key));
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

function isStructuredFrontmatterEntry(key: string, value: unknown): boolean {
  return (
    Array.isArray(value)
    || (typeof value === 'object' && value !== null)
    || STRUCTURED_FRONTMATTER_KEYS.has(key)
  );
}

interface FileViewerModalProps {
  skillName: string;
  filepath: string;
  sourcePath?: string;
  onClose: () => void;
}

export default function FileViewerModal({ skillName, filepath, sourcePath, onClose }: FileViewerModalProps) {
  const fullPath = sourcePath ? `${sourcePath}/${filepath}` : filepath;
  const { toast } = useToast();
  const [data, setData] = useState<SkillFileContent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [savedContent, setSavedContent] = useState('');
  const [draftContent, setDraftContent] = useState('');
  const [editorSurface, setEditorSurface] = useState<SkillMarkdownEditorSurface>('rich');
  const [editorLoadError, setEditorLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [saveState, setSaveState] = useState<SaveIndicatorState>('idle');
  const [isOpening, setIsOpening] = useState(false);
  const [showDiscardDialog, setShowDiscardDialog] = useState(false);
  const [showFrontmatter, setShowFrontmatter] = useState(false);
  const [frontmatterInputValues, setFrontmatterInputValues] = useState<FrontmatterCardInputMap>({});
  const [frontmatterErrors, setFrontmatterErrors] = useState<Record<string, string>>({});
  const [workspaceSlots, setWorkspaceSlots] = useState<WorkspaceSlotState[]>([]);
  const autosaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const saveStatusTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const queuedAutosaveContentRef = useRef<string | null>(null);
  const pendingSuccessToastRef = useRef(false);
  const draftContentRef = useRef('');
  const savedContentRef = useRef('');
  const isSavingRef = useRef(false);
  const closeAfterSaveRef = useRef(false);
  const frontmatterInputValuesRef = useRef<FrontmatterCardInputMap>({});
  const workspaceSlotsRef = useRef<WorkspaceSlotState[]>([]);
  const hasFrontmatterValidationErrorsRef = useRef(false);
  const hasPendingFrontmatterEditsRef = useRef(false);
  const isMarkdownFile = filepath.toLowerCase().endsWith('.md');
  const isDirty = draftContent !== savedContent;
  const parsedMarkdown = useMemo(
    () => (isMarkdownFile ? parseSkillMarkdown(draftContent) : null),
    [draftContent, isMarkdownFile],
  );
  const canonicalBuiltInKeys = useMemo(
    () => (isMarkdownFile && parsedMarkdown ? getBuiltInFrontmatterKeys(parsedMarkdown.frontmatter) : []),
    [isMarkdownFile, parsedMarkdown],
  );
  const canonicalAdditionalFrontmatterEntries = useMemo(
    () => (isMarkdownFile && parsedMarkdown ? getAdditionalFrontmatterEntries(parsedMarkdown.frontmatter) : []),
    [isMarkdownFile, parsedMarkdown],
  );
  const workspaceSlotEntries = useMemo(() => {
    if (!isMarkdownFile) return [];
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
  }, [canonicalAdditionalFrontmatterEntries, canonicalBuiltInKeys, frontmatterInputValues, isMarkdownFile, workspaceSlots]);
  const workspaceBuiltInEntries = useMemo<FrontmatterWorkspaceEntry[]>(() => {
    const grouped = workspaceSlotEntries.reduce<FrontmatterWorkspaceEntry[]>((entries, slot) => {
      const inputState = frontmatterInputValues[slot.id];
      const displayKey = (inputState?.key ?? slot.key).trim();
      if (!BUILT_IN_FRONTMATTER_ORDER.includes(displayKey)) {
        return entries;
      }
      const canonicalValue = displayKey && parsedMarkdown ? parsedMarkdown.frontmatter[displayKey] : undefined;
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
  }, [frontmatterErrors, frontmatterInputValues, parsedMarkdown, workspaceSlotEntries]);
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
      const canonicalValue = displayKey && parsedMarkdown ? parsedMarkdown.frontmatter[displayKey] : undefined;

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
  ), [frontmatterErrors, frontmatterInputValues, parsedMarkdown, workspaceSlotEntries]);
  const hasConfiguredFrontmatter = canonicalBuiltInKeys.length > 0 || canonicalAdditionalFrontmatterEntries.length > 0;
  const hasPendingFrontmatterEdits = useMemo(() => workspaceSlotEntries.some((slot) => {
    const inputState = frontmatterInputValues[slot.id];
    if (!inputState || !parsedMarkdown) {
      return false;
    }

    const displayKey = inputState.key ?? slot.key;
    const canonicalValue = displayKey ? serializeFrontmatterEditorValue(parsedMarkdown.frontmatter[displayKey]) : '';
    const displayValue = inputState.value ?? canonicalValue;

    return displayKey !== slot.key || displayValue !== canonicalValue;
  }), [frontmatterInputValues, parsedMarkdown, workspaceSlotEntries]);
  const hasFrontmatterValidationErrors = Object.keys(frontmatterErrors).length > 0;
  const hasUnsavedChanges = isDirty || hasPendingFrontmatterEdits;

  const clearAutosaveTimer = () => {
    if (autosaveTimerRef.current) {
      clearTimeout(autosaveTimerRef.current);
      autosaveTimerRef.current = null;
    }
  };

  const queueSaveStateReset = () => {
    if (saveStatusTimerRef.current) {
      clearTimeout(saveStatusTimerRef.current);
    }

    saveStatusTimerRef.current = setTimeout(() => {
      setSaveState((current) => (current === 'saved' ? 'idle' : current));
    }, SAVE_STATUS_RESET_MS);
  };

  const persistQueuedContent = async () => {
    if (!isMarkdownFile || isSavingRef.current || hasFrontmatterValidationErrorsRef.current) return;

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
      const response = await api.saveSkillFile(skillName, filepath, requestContent);
      savedContentRef.current = response.content;
      setSavedContent(response.content);

      if (draftContentRef.current === requestContent) {
        draftContentRef.current = response.content;
        setDraftContent(response.content);
        syncWorkspaceStateFromContent(response.content);
      }

      queuedAutosaveContentRef.current = draftContentRef.current === requestContent
        ? response.content
        : draftContentRef.current;
      setData((current) => (current ? { ...current, content: response.content } : current));
      setEditorLoadError(null);
      setEditorSurface('rich');
      setSaveState('saved');
      queueSaveStateReset();
      if (closeAfterSaveRef.current && !hasPendingFrontmatterEditsRef.current) {
        closeAfterSaveRef.current = false;
        onClose();
        return;
      }
      if (pendingSuccessToastRef.current) {
        toast('Saved', 'success');
        pendingSuccessToastRef.current = false;
      }
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
  };

  const scheduleAutosave = (nextContent: string, delay = AUTOSAVE_DELAY_MS) => {
    if (!isMarkdownFile) return;

    queuedAutosaveContentRef.current = nextContent;
    setSaveError(null);
    if (saveState !== 'saving') {
      setSaveState('idle');
    }
    clearAutosaveTimer();
    autosaveTimerRef.current = setTimeout(() => {
      void persistQueuedContent();
    }, delay);
  };

  const flushAutosave = async (nextContent = draftContentRef.current) => {
    if (!isMarkdownFile) return;

    queuedAutosaveContentRef.current = nextContent;
    clearAutosaveTimer();
    await persistQueuedContent();
  };

  const maybeToastRecentSave = () => {
    if (saveState === 'saved' && draftContentRef.current === savedContentRef.current) {
      pendingSuccessToastRef.current = false;
      setSaveState('idle');
      toast('Saved', 'success');
      return true;
    }

    return false;
  };

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
    hasPendingFrontmatterEditsRef.current = hasPendingFrontmatterEdits;
  }, [hasPendingFrontmatterEdits]);

  useEffect(() => {
    isSavingRef.current = isSaving;
  }, [isSaving]);

  useEffect(() => {
    if (!closeAfterSaveRef.current || isSaving) {
      return;
    }

    if (draftContent === savedContent && !hasPendingFrontmatterEdits && !saveError) {
      closeAfterSaveRef.current = false;
      onClose();
      return;
    }

    if ((saveError || hasPendingFrontmatterEdits) && (draftContent !== savedContent || hasPendingFrontmatterEdits)) {
      closeAfterSaveRef.current = false;
      setShowDiscardDialog(true);
    }
  }, [draftContent, hasPendingFrontmatterEdits, isSaving, onClose, saveError, savedContent]);

  useEffect(() => {
    setLoading(true);
    setError(null);
    setSaveError(null);
    setSaveState('idle');
    setEditorSurface('rich');
    setEditorLoadError(null);
    setIsOpening(false);
    setShowFrontmatter(false);
    setFrontmatterInputValues({});
    setFrontmatterErrors({});
    setWorkspaceSlots([]);
    pendingSuccessToastRef.current = false;
    clearAutosaveTimer();
    queuedAutosaveContentRef.current = null;
    closeAfterSaveRef.current = false;
    setShowDiscardDialog(false);
    api
      .getSkillFile(skillName, filepath)
      .then((response) => {
        setData(response);
        setSavedContent(response.content);
        setDraftContent(response.content);
        savedContentRef.current = response.content;
        draftContentRef.current = response.content;
        const nextSlots = buildWorkspaceSlotsFromContent(response.content);
        workspaceSlotsRef.current = nextSlots;
        setWorkspaceSlots(nextSlots);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));

    return () => {
      clearAutosaveTimer();
      if (saveStatusTimerRef.current) {
        clearTimeout(saveStatusTimerRef.current);
      }
    };
  }, [filepath, skillName]);

  const cmExtensions = useMemo(() => {
    if (!data) return [];
    const exts = [EditorView.lineWrapping, EditorView.editable.of(false), ...handTheme];
    if (data.contentType === 'application/json') exts.push(json());
    else if (data.contentType === 'text/yaml') exts.push(yaml());
    // Infer language from filename extension
    const ext = filepath.split('.').pop()?.toLowerCase();
    if (ext === 'py') exts.push(python());
    else if (ext === 'js' || ext === 'mjs' || ext === 'cjs') exts.push(javascript());
    else if (ext === 'ts' || ext === 'mts' || ext === 'cts') exts.push(javascript({ typescript: true }));
    else if (ext === 'jsx') exts.push(javascript({ jsx: true }));
    else if (ext === 'tsx') exts.push(javascript({ jsx: true, typescript: true }));
    return exts;
  }, [data, filepath]);

  const syncWorkspaceStateFromContent = (content: string) => {
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
  };

  const clearFrontmatterError = (entryId: string) => {
    setFrontmatterErrors((current) => {
      if (!(entryId in current)) {
        return current;
      }

      const next = { ...current };
      delete next[entryId];
      return next;
    });
  };

  const clearFrontmatterInput = (entryId: string) => {
    setFrontmatterInputValues((current) => {
      if (!(entryId in current)) {
        return current;
      }

      const next = { ...current };
      delete next[entryId];
      frontmatterInputValuesRef.current = next;
      return next;
    });
  };

  const updateWorkspaceSlot = (entryId: string, nextKey: string) => {
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
  };

  const removeWorkspaceSlot = (entryId: string) => {
    setWorkspaceSlots((current) => {
      const next = current.filter((entry) => entry.id !== entryId);
      workspaceSlotsRef.current = next;
      return next;
    });
  };

  const nextCustomEntryId = () => `custom-entry-${Math.random().toString(36).slice(2, 10)}`;

  const handleFrontmatterValueChange = (entryId: string, rawValue: string) => {
    const workspaceEntry = workspaceSlotEntries.find((entry) => entry.id === entryId);
    const key = (frontmatterInputValues[entryId]?.key ?? workspaceEntry?.key ?? '').trim();

    setFrontmatterInputValues((current) => {
      const next = mergeFrontmatterInputState(current, entryId, { value: rawValue });
      frontmatterInputValuesRef.current = next;
      return next;
    });

    if (!key || !parsedMarkdown) {
      if (rawValue.trim().length === 0) {
        clearFrontmatterError(entryId);
      } else {
        setFrontmatterErrors((current) => ({ ...current, [entryId]: 'Custom frontmatter key is required.' }));
      }
      return;
    }

    try {
      const nextContent = updateSkillFrontmatterField(draftContent, key, rawValue, parsedMarkdown.frontmatter[key]);
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
  };

  const handleFrontmatterFieldBlur = (entryId: string) => {
    if (frontmatterErrors[entryId]) {
      return;
    }

    if (maybeToastRecentSave()) {
      return;
    }
    pendingSuccessToastRef.current = true;
    void flushAutosave();
  };

  const handleAddBuiltInField = (key: string) => {
    const entryId = nextCustomEntryId();
    updateWorkspaceSlot(entryId, key);
    setFrontmatterInputValues((current) => {
      const next = mergeFrontmatterInputState(current, entryId, buildWorkspaceInputMap(key, ''));
      frontmatterInputValuesRef.current = next;
      return next;
    });
    clearFrontmatterError(entryId);
  };

  const handleAddCustomField = () => {
    const entryId = nextCustomEntryId();
    updateWorkspaceSlot(entryId, '');
    setFrontmatterInputValues((current) => {
      const next = mergeFrontmatterInputState(current, entryId, buildWorkspaceInputMap('', ''));
      frontmatterInputValuesRef.current = next;
      return next;
    });
    clearFrontmatterError(entryId);
  };

  const handleRemoveFrontmatterField = (entryId: string) => {
    const workspaceEntry = workspaceSlotEntries.find((entry) => entry.id === entryId);
    if (!workspaceEntry || !parsedMarkdown) {
      return;
    }

    const currentKey = (frontmatterInputValues[entryId]?.key ?? workspaceEntry.key).trim();
    const nextContent = currentKey
      ? updateSkillFrontmatterField(draftContent, currentKey, '', parsedMarkdown.frontmatter[currentKey])
      : draftContent;
    draftContentRef.current = nextContent;
    setDraftContent(nextContent);
    syncWorkspaceStateFromContent(nextContent);
    if (nextContent !== draftContent) {
      scheduleAutosave(nextContent);
    }
    removeWorkspaceSlot(entryId);
    clearFrontmatterInput(entryId);
    clearFrontmatterError(entryId);
    setSaveError(null);
  };

  const handleFrontmatterFieldKeyChange = (entryId: string, nextKeyInput: string) => {
    const workspaceEntry = workspaceSlotEntries.find((entry) => entry.id === entryId);
    if (!workspaceEntry || !parsedMarkdown) {
      return;
    }

    setFrontmatterInputValues((current) => {
      const next = mergeFrontmatterInputState(current, entryId, { key: nextKeyInput });
      frontmatterInputValuesRef.current = next;
      return next;
    });

    const currentKey = (frontmatterInputValues[entryId]?.key ?? workspaceEntry.key).trim();
    const pendingValue = frontmatterInputValues[entryId]?.value
      ?? serializeFrontmatterEditorValue(currentKey ? parsedMarkdown.frontmatter[currentKey] : undefined);
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
      if (currentKey && Object.prototype.hasOwnProperty.call(parsedMarkdown.frontmatter, currentKey)) {
        nextContent = renameSkillFrontmatterField(draftContent, currentKey, nextKey);
      } else if (pendingValue.trim().length > 0) {
        nextContent = updateSkillFrontmatterField(draftContent, nextKey, pendingValue, parsedMarkdown.frontmatter[nextKey]);
      }

      updateWorkspaceSlot(entryId, nextKey);
      draftContentRef.current = nextContent;
      setDraftContent(nextContent);
      syncWorkspaceStateFromContent(nextContent);
      if (nextContent !== draftContent) {
        scheduleAutosave(nextContent);
      }
      clearFrontmatterError(entryId);
      setSaveError(null);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Invalid frontmatter key.';
      setFrontmatterErrors((current) => ({ ...current, [entryId]: message }));
    }
  };

  const handleOpenFile = async () => {
    if (isOpening || loading || !!error) {
      return;
    }

    setIsOpening(true);
    try {
      await api.openSkillFile(skillName, filepath);
    } catch (e: unknown) {
      const message = (e as Error).message;
      setSaveError(message);
      toast(message, 'error');
    } finally {
      setIsOpening(false);
    }
  };

  const requestClose = async () => {
    if (isSavingRef.current) {
      closeAfterSaveRef.current = true;
      return;
    }

    if (!isMarkdownFile || !hasUnsavedChanges) {
      onClose();
      return;
    }

    if (saveError) {
      setShowDiscardDialog(true);
      return;
    }

    closeAfterSaveRef.current = true;
    await flushAutosave();
    if (!closeAfterSaveRef.current) {
      return;
    }

    if (draftContentRef.current === savedContentRef.current && !hasPendingFrontmatterEditsRef.current) {
      closeAfterSaveRef.current = false;
      onClose();
      return;
    }

    closeAfterSaveRef.current = false;
    setShowDiscardDialog(true);
  };

  return (
    <>
      <DialogShell
        open={true}
        onClose={() => {
          void requestClose();
        }}
        maxWidth="3xl"
        padding="none"
        preventClose={isSaving}
        className="max-h-[85vh] flex flex-col overflow-hidden"
      >
        <div className="flex items-center justify-between mb-3 px-6 pt-6">
          <h3
            className="font-bold text-pencil truncate font-mono flex items-center gap-1.5"
            style={{ fontSize: '0.95rem' }}
          >
            {filepath}
            <CopyButton
              value={fullPath}
              title="Copy file path"
              copiedLabel="Path copied!"
              copiedLabelClassName="text-xs font-normal"
            />
          </h3>
          <div className="ml-2 flex shrink-0 items-center gap-2">
            {isMarkdownFile ? (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setShowFrontmatter((current) => !current)}
                disabled={loading || !!error}
              >
                {showFrontmatter ? <ChevronUp size={14} strokeWidth={2.5} /> : <ChevronDown size={14} strokeWidth={2.5} />}
                {showFrontmatter ? 'Hide frontmatter' : 'Show frontmatter'}
              </Button>
            ) : null}
            {!isMarkdownFile ? (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => void handleOpenFile()}
                disabled={loading || !!error || isOpening}
              >
                {isOpening ? <Spinner size="sm" /> : null}
                Open
              </Button>
            ) : null}
            <IconButton
              icon={<X size={16} strokeWidth={2.5} />}
              label="Close"
              size="md"
              onMouseDown={() => {
                if (isMarkdownFile && hasUnsavedChanges) {
                  closeAfterSaveRef.current = true;
                }
              }}
              onClick={() => {
                void requestClose();
              }}
              disabled={isSaving}
              className="shrink-0"
            />
          </div>
        </div>

        <div className="overflow-auto flex-1 min-h-0 px-6 pb-6">
          {loading && (
            <div className="py-12 flex justify-center">
              <Spinner size="md" />
            </div>
          )}

          {error && (
            <div className="py-8 text-center">
              <p className="text-danger">
                {error}
              </p>
            </div>
          )}

          {data && !loading && (
            <>
              {isMarkdownFile ? (
                <>
                  {saveError ? (
                    <div
                      role="alert"
                      className="mb-4 rounded-[var(--radius-md)] border border-danger/40 bg-danger/5 px-4 py-3 text-sm text-danger"
                    >
                      {saveError}
                    </div>
                  ) : null}
                  {showFrontmatter ? (
                    <section className="mb-4 rounded-[var(--radius-lg)] border border-muted-dark/50 bg-surface/80 p-4">
                      {!hasConfiguredFrontmatter ? (
                        <p className="mb-4 rounded-[var(--radius-sm)] border border-muted/80 bg-paper/60 px-3 py-2 text-sm text-pencil-light">
                          No frontmatter fields are set yet. The workspace below lets you add built-in or custom entries without leaving the modal.
                        </p>
                      ) : null}

                      <SkillFrontmatterWorkspace
                        mode="edit"
                        builtInEntries={workspaceBuiltInEntries}
                        additionalEntries={workspaceAdditionalEntries}
                        referenceExcludeKeys={[]}
                        onAddBuiltInField={handleAddBuiltInField}
                        onAddCustomField={handleAddCustomField}
                        onRemoveField={handleRemoveFrontmatterField}
                        onChangeFieldValue={handleFrontmatterValueChange}
                        onChangeFieldKey={handleFrontmatterFieldKeyChange}
                        onBlurFieldValue={handleFrontmatterFieldBlur}
                        onBlurFieldKey={handleFrontmatterFieldBlur}
                      />
                    </section>
                  ) : null}
                  <ContentStatsBar
                    content={draftContent}
                    description={parsedMarkdown?.manifest.description}
                    body={parsedMarkdown?.markdown}
                    fileCount={1}
                    license={parsedMarkdown?.manifest.license}
                    className="mb-4"
                  />
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
                    <MarkdownEditorPanel
                      value={draftContent}
                      surface={editorSurface}
                      isDirty={isDirty}
                      editorLoadError={editorLoadError}
                      onChange={(next) => {
                        draftContentRef.current = next;
                        setDraftContent(next);
                        syncWorkspaceStateFromContent(next);
                        setSaveError(null);
                        scheduleAutosave(next);
                      }}
                      onSurfaceChange={setEditorSurface}
                      onEditorLoadError={setEditorLoadError}
                    />
                  </div>
                </>
              ) : (
                <>
                  {saveError ? (
                    <div
                      role="alert"
                      className="mb-4 rounded-[var(--radius-md)] border border-danger/40 bg-danger/5 px-4 py-3 text-sm text-danger"
                    >
                      {saveError}
                    </div>
                  ) : null}
                  <CodeMirror
                    value={data.content}
                    extensions={cmExtensions}
                    theme="none"
                    readOnly
                    editable={false}
                    basicSetup={{
                      lineNumbers: true,
                      foldGutter: true,
                      highlightActiveLine: false,
                      bracketMatching: true,
                      autocompletion: false,
                    }}
                  />
                </>
              )}
            </>
          )}
        </div>
      </DialogShell>

      <ConfirmDialog
        open={showDiscardDialog}
        onConfirm={() => {
          closeAfterSaveRef.current = false;
          setShowDiscardDialog(false);
          onClose();
        }}
        onCancel={() => {
          closeAfterSaveRef.current = false;
          setShowDiscardDialog(false);
        }}
        title="Discard changes?"
        message="You have unsaved changes in this file. Discard them and close the modal?"
        confirmText="Discard Changes"
        cancelText="Keep Editing"
        variant="danger"
      />
    </>
  );
}

type MarkdownEditorPanelProps = {
  editorLoadError: string | null;
  isDirty: boolean;
  onChange: (value: string) => void;
  onEditorLoadError: (message: string) => void;
  onSurfaceChange: (surface: SkillMarkdownEditorSurface) => void;
  surface: SkillMarkdownEditorSurface;
  value: string;
};

function MarkdownEditorPanel({
  editorLoadError,
  isDirty,
  onChange,
  onEditorLoadError,
  onSurfaceChange,
  surface,
  value,
}: MarkdownEditorPanelProps) {
  if (editorLoadError) {
    return (
      <RawMarkdownFallback
        errorMessage={editorLoadError}
        onChange={onChange}
        value={value}
      />
    );
  }

  return (
    <EditorLoadBoundary
      onError={(message) => onEditorLoadError(message ?? 'Markdown editor unavailable. Raw mode is active.')}
      resetKey="markdown-editor"
    >
      <Suspense fallback={<EditorFallback />}>
        <SkillMarkdownEditor
          value={value}
          onChange={onChange}
          onSave={() => {}}
          onDiscard={() => {}}
          onSurfaceChange={onSurfaceChange}
          surface={surface}
          mode="edit"
          isDirty={isDirty}
          showToolbar={false}
          surfaceStyle="inline"
        />
      </Suspense>
    </EditorLoadBoundary>
  );
}

type RawMarkdownFallbackProps = {
  errorMessage: string;
  onChange: (value: string) => void;
  value: string;
};

function RawMarkdownFallback({
  errorMessage,
  onChange,
  value,
}: RawMarkdownFallbackProps) {
  return (
    <section className="flex flex-col gap-3">
      <div
        role="alert"
        className="rounded-[var(--radius-md)] border border-danger/40 bg-danger/5 px-4 py-3 text-sm text-danger"
      >
        {errorMessage}
      </div>

      <div className="rounded-[var(--radius-lg)] border-2 border-muted bg-surface p-3">
        <Textarea
          aria-label="Raw markdown editor"
          value={value}
          onChange={(event) => onChange(event.currentTarget.value)}
          spellCheck={false}
          className="prose-hand min-h-[28rem] w-full font-mono"
        />
      </div>
    </section>
  );
}

class EditorLoadBoundary extends Component<{
  children: ReactNode;
  onError: (message?: string) => void;
  resetKey: string;
}, { hasError: boolean }> {
  state = { hasError: false };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error: unknown) {
    const message = error instanceof Error ? error.message : undefined;
    this.props.onError(message);
  }

  componentDidUpdate(prevProps: Readonly<{ children: ReactNode; onError: (message?: string) => void; resetKey: string }>) {
    if (prevProps.resetKey !== this.props.resetKey && this.state.hasError) {
      this.setState({ hasError: false });
    }
  }

  render() {
    if (this.state.hasError) {
      return null;
    }

    return this.props.children;
  }
}

function editorShellClassName(isSaving: boolean, stretch = false) {
  return [
    'transition-opacity duration-150',
    stretch ? 'h-full min-h-0' : '',
    isSaving ? 'opacity-70 pointer-events-none' : '',
  ].filter(Boolean).join(' ');
}

function EditorFallback() {
  return (
    <div className="rounded-[var(--radius-lg)] border-2 border-muted bg-surface p-6">
      <div className="flex items-center justify-center py-10">
        <Spinner size="md" />
      </div>
    </div>
  );
}
