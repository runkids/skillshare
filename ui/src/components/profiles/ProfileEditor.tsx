import { useEffect, useId, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { api, type ProfileDefinition, type ProfilePreviewResult } from '../../api/client';
import { useT } from '../../i18n';
import { invalidate } from '../../lib/queryEvents';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import ConfirmDialog from '../ConfirmDialog';
import DialogShell from '../DialogShell';
import { Input } from '../Input';
import SyncPreviewModal from '../SyncPreviewModal';
import PatternInput from '../targets/PatternInput';
import { profileReason } from './profileView';

export default function ProfileEditor({ profile, active = false, onCreated }: {
  profile?: ProfileDefinition;
  active?: boolean;
  onCreated?: (name: string) => void;
}) {
  const t = useT();
  const id = useId();
  const navigate = useNavigate();
  const client = useQueryClient();
  const creating = !profile;
  const [name, setName] = useState(profile?.name ?? '');
  const [description, setDescription] = useState(profile?.description ?? '');
  const [specific, setSpecific] = useState(creating || profile?.skills != null);
  const [include, setInclude] = useState(profile?.skills?.include ?? []);
  const [exclude, setExclude] = useState(profile?.skills?.exclude ?? []);
  const selection = useMemo(() => specific ? { include, exclude } : undefined, [specific, include, exclude]);
  const body = useMemo(() => ({ description, ...(selection && { skills: selection }) }), [description, selection]);
  const serialized = JSON.stringify(body);
  const [saved, setSaved] = useState(serialized);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<ProfilePreviewResult | null>(null);
  const [previewError, setPreviewError] = useState('');
  const [previewing, setPreviewing] = useState(true);
  const [renaming, setRenaming] = useState(false);
  const [newName, setNewName] = useState(name);
  const [deleting, setDeleting] = useState(false);
  const [needsSync, setNeedsSync] = useState(false);
  const [syncOpen, setSyncOpen] = useState(false);
  const dirty = creating ? !!name.trim() : serialized !== saved;

  useEffect(() => {
    let stale = false;
    const timer = setTimeout(() => {
      setPreviewing(true);
      setPreviewError('');
      void api.profiles.preview(selection ? { skills: selection } : {}).then((result) => {
        if (!stale) setPreview(result);
      }).catch((cause: unknown) => {
        if (!stale) setPreviewError(profileReason(cause instanceof Error ? cause.message : String(cause), t, 'preview'));
      }).finally(() => { if (!stale) setPreviewing(false); });
    }, 250);
    return () => { stale = true; clearTimeout(timer); };
  }, [selection, t]);

  const fail = (cause: unknown, candidate = name.trim()) => setError(profileReason(cause instanceof Error ? cause.message : String(cause), t, candidate));
  const save = async () => {
    setBusy(true);
    setError('');
    try {
      const result = creating ? await api.profiles.create({ name: name.trim(), ...body }) : await api.profiles.replace(name, body);
      setSaved(serialized);
      setNeedsSync(result.needsSync);
      await invalidate(client, 'profilesChanged');
      if (creating) onCreated?.(name.trim());
    } catch (cause: unknown) { fail(cause); }
    finally { setBusy(false); }
  };
  const rename = async () => {
    setBusy(true);
    setError('');
    try {
      await api.profiles.replace(name, { name: newName.trim() });
      await invalidate(client, 'profilesChanged');
      setRenaming(false);
      navigate(`/profiles/${encodeURIComponent(newName.trim())}`);
    } catch (cause: unknown) { fail(cause, newName.trim()); }
    finally { setBusy(false); }
  };
  const remove = async () => {
    setBusy(true);
    setError('');
    try {
      await api.profiles.delete(name);
      await invalidate(client, 'profilesChanged');
      navigate('/profiles');
    } catch (cause: unknown) { fail(cause); }
    finally { setBusy(false); setDeleting(false); }
  };

  return <div className="flex flex-col gap-5">
    {!renaming && error && <div className="ss-note bad" role="alert">{error}</div>}
    <div className="ss-split !h-auto flex-col gap-5 md:flex-row">
      <div className="lp !p-0 w-full md:w-1/2 flex flex-col gap-5">
        {creating && <Input label={t('profileEdit.name')} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} autoFocus />}
        <Input label={t('profileEdit.description')} value={description} onChange={(e) => setDescription(e.target.value)} disabled={busy} />
        {!creating && profile.skills === null && <Checkbox label={t('profileEdit.specific')} checked={specific} onChange={setSpecific} disabled={busy} />}
        {!specific && <p className="text-sm text-ink-3">{t('profileEdit.passThrough')}</p>}
        {specific && <>
          <div className="ss-fld"><label htmlFor={`${id}-include`}>{t('profileEdit.include')}</label><PatternInput id={`${id}-include`} patterns={include} onChange={setInclude} disabled={busy} /></div>
          <div className="ss-fld"><label htmlFor={`${id}-exclude`}>{t('profileEdit.exclude')}</label><PatternInput id={`${id}-exclude`} patterns={exclude} onChange={setExclude} disabled={busy} /></div>
          <p className="text-xs text-ink-3">{t('profileEdit.patternHint')}</p>
          {include.length === 0 && exclude.length > 0 && <div className="ss-note inf">{t('profileEdit.inert')}</div>}
        </>}
        <div className="flex flex-wrap items-center gap-2">
          <Button onClick={() => void save()} disabled={!dirty || busy} loading={busy}>{t(creating ? 'profileEdit.create' : 'profileEdit.save')}</Button>
          {!creating && <>
            <Button variant="secondary" disabled={busy || !dirty} onClick={() => {
              const draft: typeof body = JSON.parse(saved);
              setDescription(draft.description); setSpecific(!!draft.skills);
              setInclude(draft.skills?.include ?? []); setExclude(draft.skills?.exclude ?? []); setError('');
            }}>{t('common.cancel')}</Button>
            <Button variant="secondary" onClick={() => { setError(''); setNewName(name); setRenaming(true); }} disabled={busy || dirty}>{t('profileEdit.rename')}</Button>
            <Button variant="danger" onClick={() => setDeleting(true)} disabled={busy || active} title={active ? t('profileEdit.activeDelete', { name }) : undefined}>{t('profileEdit.delete')}</Button>
          </>}
      </div>
      </div>
      <div className="rp !flex-none md:!flex-1">
        <section aria-label={t('profileEdit.selected')} className="flex flex-col gap-2">
          <h2 className="ss-h2">{t('profileEdit.selected')}</h2>
          {previewing && <p className="text-sm text-ink-3">{t('targetDetail.loadingPreview')}</p>}
          {previewError && <div className="ss-note bad">{previewError}</div>}
          {preview && <>
            <p className="text-xs text-ink-3">{t('profiles.count', { count: preview.selected.length })}</p>
            <div className="ss-list max-h-64 overflow-y-auto">{preview.selected.map((skill) => <div key={skill} className="ss-r font-mono text-sm">{skill}</div>)}</div>
            {preview.selected.length === 0 && <p className="text-sm text-ink-3">{t('profileEdit.noneSelected')}</p>}
            {preview.unmatched.length > 0 && <div className="ss-note warn">{t('profileEdit.unmatched', { patterns: preview.unmatched.join(', ') })}</div>}
          </>}
        </section>
      </div>
    </div>
    {needsSync && <div className="ss-note warn flex-wrap"><span className="flex-1">{t('profileEdit.needsSync')}</span><Button variant="secondary" onClick={() => setSyncOpen(true)}>{t('profiles.syncNow')}</Button></div>}
    <DialogShell open={renaming} onClose={() => setRenaming(false)} preventClose={busy} ariaLabel={t('profileEdit.rename')} maxWidth="sm">
      <div className="flex flex-col gap-4"><h2 className="ss-h2">{t('profileEdit.rename')}</h2>
        {error && <div className="ss-note bad" role="alert">{error}</div>}
        <Input label={t('profileEdit.name')} value={newName} onChange={(e) => setNewName(e.target.value)} disabled={busy} />
        <div className="flex justify-end gap-2"><Button variant="secondary" onClick={() => setRenaming(false)} disabled={busy}>{t('common.cancel')}</Button><Button onClick={() => void rename()} disabled={busy || !newName.trim() || newName.trim() === name} loading={busy}>{t('profileEdit.rename')}</Button></div>
      </div>
    </DialogShell>
    <ConfirmDialog open={deleting} onCancel={() => setDeleting(false)} onConfirm={() => void remove()} loading={busy} title={t('profileEdit.delete')} confirmText={t('profileEdit.delete')} message={t('profileEdit.deleteConfirm', { name })} variant="danger" />
    {syncOpen && <SyncPreviewModal open onClose={() => setSyncOpen(false)} kind="skill" profilePending={needsSync} />}
  </div>;
}
