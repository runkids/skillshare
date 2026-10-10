import { useState, type AnimationEvent } from 'react';
import { useT } from '../i18n';
import { newFolderPath, type FolderOption } from '../lib/moveFolders';
import Button from './Button';
import NewFolderFields from './NewFolderFields';

interface Options {
  kind: 'skill' | 'agent';
  folders: FolderOption[];
  rootCount: number;
  /** The skill the link names are shown with, when known. */
  skill: string | null;
  disabledPaths?: string[];
  /** "Use this folder" with a legal path. */
  onUse: (path: string) => void;
  /** Back, or the dialog's own way out of the step. */
  onBack: () => void;
}

/**
 * The new-folder step of the folder picker, for a dialog to show in place of its form: the header text
 * and the body and footer to render. `start(parent)` seeds it with the folder that was selected.
 */
export function useNewFolderStep({ kind, folders, rootCount, skill, disabledPaths, onUse, onBack }: Options) {
  const t = useT();
  const [draft, setDraft] = useState({ parent: '', name: '' });
  // Which way the dialog's content just moved, until its slide-in has played.
  const [dir, setDir] = useState<'fwd' | 'back' | null>(null);
  const path = newFolderPath(draft.parent, draft.name);
  const back = () => {
    setDir('back');
    onBack();
  };
  const use = () => {
    if (!path) return;
    setDir('back');
    onUse(path);
  };
  return {
    start: (parent: string) => {
      setDraft({ parent, name: '' });
      setDir('fwd');
    },
    back,
    /**
     * For the dialog's header, body and footer, each keyed by the step it shows: the slide-in class
     * and the handler that clears it, so a later change of content does not slide too.
     */
    motion: {
      className: dir === 'fwd' ? 'animate-step-fwd' : dir === 'back' ? 'animate-step-back' : '',
      // A child's own animation (a dropdown, a spinner) bubbles up here; only the slide-in counts.
      onAnimationEnd: (e: AnimationEvent<HTMLElement>) => {
        if (e.target === e.currentTarget) setDir(null);
      },
    },
    title: t('folderPicker.newTitle'),
    sub: t('folderPicker.newHint'),
    body: (
      <NewFolderFields
        kind={kind}
        folders={folders}
        rootCount={rootCount}
        disabledPaths={disabledPaths}
        parent={draft.parent}
        name={draft.name}
        skill={skill}
        onParent={(parent) => setDraft((d) => ({ ...d, parent }))}
        onName={(name) => setDraft((d) => ({ ...d, name }))}
        onSubmit={use}
      />
    ),
    foot: (
      <>
        <Button variant="secondary" onClick={back}>{t('folderPicker.back')}</Button>
        <Button variant="primary" disabled={!path} onClick={use}>{t('folderPicker.use')}</Button>
      </>
    ),
  };
}
