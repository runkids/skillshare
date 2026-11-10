import { useEffect, useRef, useState } from 'react';
import { FilePlus, Plus, Upload } from 'lucide-react';
import Button from '../Button';
import { useT } from '../../i18n';
import { useMenuPresence } from '../../hooks/usePresence';

interface Props {
  /** Rejects with a message the menu shows under the field. */
  onAdd: (url: string) => Promise<void>;
  onCreate: () => void;
  onImport: (file: File) => void;
}

/** One entry point for every way a hub gets into the list. */
export default function AddHubMenu({ onAdd, onCreate, onImport }: Props) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const menu = useMenuPresence(open);
  const [url, setURL] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const urlInput = useRef<HTMLInputElement>(null);
  // autoFocus would miss a reopen during the fade-out, when the panel never unmounted.
  useEffect(() => { if (open) urlInput.current?.focus(); }, [open]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const add = async () => {
    setBusy(true);
    setError('');
    try {
      await onAdd(url);
      setURL('');
      setOpen(false);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div ref={ref} className="relative">
      <input
        ref={fileInput}
        type="file"
        accept=".json,application/json"
        className="hidden"
        aria-label={t('hubs.add.import')}
        onChange={(e) => {
          const file = e.target.files?.[0];
          e.target.value = '';
          if (file) onImport(file);
        }}
      />
      <button type="button" className="ss-btn w-full" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(!open)}>
        <Plus size={14} />
        {t('hubs.addOrCreate')}
      </button>
      {menu.shown && (
        <div role="menu" aria-label={t('hubs.addOrCreate')} {...menu.exitProps} className={`ss-menu absolute left-0 top-full z-50 mt-2 !w-[400px] origin-top-left ${menu.motion}`}>
          <form
            className="flex flex-col gap-2 px-2.5 pt-2.5 pb-3"
            onSubmit={(e) => { e.preventDefault(); void add(); }}
          >
            <label htmlFor="hub-add-url" className="text-[13px] font-semibold">{t('hubs.add.existing')}</label>
            <div className="flex gap-2">
              <span className="ss-inp min-w-0 flex-1">
                <input
                  id="hub-add-url"
                  ref={urlInput}
                  value={url}
                  disabled={busy}
                  placeholder={t('install.hubs.urlPlaceholder')}
                  onChange={(e) => { setURL(e.target.value); setError(''); }}
                />
              </span>
              <Button type="submit" loading={busy}>{t('install.hubs.addButton')}</Button>
            </div>
            {error ? <span className="text-[12.5px] text-bad">{error}</span> : <span className="text-xs text-ink-3">{t('hubs.add.hint')}</span>}
          </form>
          <hr />
          {[
            { icon: FilePlus, title: t('hubs.add.create'), hint: t('hubs.add.createHint'), run: onCreate },
            { icon: Upload, title: t('hubs.add.import'), hint: t('hubs.add.importHint'), run: () => fileInput.current?.click() },
          ].map(({ icon: Icon, title, hint, run }) => (
            <button key={title} type="button" role="menuitem" className="!h-auto !items-start !py-3" onClick={() => { setOpen(false); run(); }}>
              <Icon size={16} className="mt-0.5 shrink-0" />
              <span className="flex flex-col gap-0.5 text-start">
                <span className="text-[13.5px] font-semibold text-ink">{title}</span>
                <span className="text-[12.5px] font-normal text-ink-2">{hint}</span>
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
