import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown } from 'lucide-react';
import { useT, plural } from '../../i18n';
import { useMenuPresence } from '../../hooks/usePresence';

/** The file tabs that don't fit in the tab strip, behind a "+N files" menu. */
export default function FileTabMenu({ files }: { files: { id: string; label: string; to: string }[] }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const menu = useMenuPresence(open);
  const ref = useRef<HTMLDivElement>(null);
  const label = t(plural('targetFiles.more', files.length), { count: files.length });

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

  return (
    <div ref={ref} className="relative">
      <button type="button" className="inline-flex items-center gap-1 hover:text-ink" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(!open)}>
        {label}<ChevronDown size={13} />
      </button>
      {menu.shown && (
        <div role="menu" aria-label={label} {...menu.exitProps} className={`ss-menu absolute left-0 top-full z-50 mt-1 !w-60 origin-top-left ${menu.motion}`}>
          {files.map((f) => (
            <Link key={f.id} to={f.to} replace role="menuitem" title={f.label} onClick={() => setOpen(false)}>
              <span className="min-w-0 flex-1 truncate font-mono">{f.label}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
