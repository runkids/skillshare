import { useEffect, useRef, useState } from 'react';
import { ArrowDownUp, Check } from 'lucide-react';
import { useT } from '../../i18n';

interface Choice {
  value: string;
  label: string;
}

interface Section {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Choice[];
}

interface Toggles {
  label: string;
  options: { label: string; checked: boolean; onChange: () => void }[];
}

interface ArrangeMenuProps {
  /** Left out where grouping does not apply (the tree view). */
  group?: Section;
  sort: Section;
  /** What the rows show; left out where the view has nothing to switch. */
  display?: Toggles;
}

/** Group and sort behind one icon button; both change rarely, so they share one menu. */
export default function ArrangeMenu({ group, sort, display }: ArrangeMenuProps) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const label = group ? t('resources.toolbar.arrange') : sort.label;

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

  const section = (s: Section) => (
    <>
      <span className="px-[9px] pt-1.5 pb-1 text-xs font-semibold text-ink-3">{s.label}</span>
      {s.options.map((o) => {
        const on = o.value === s.value;
        return (
          <button
            key={o.value}
            type="button"
            role="menuitemradio"
            aria-checked={on}
            className={on ? 'text-ink' : 'text-ink-2'}
            onClick={() => { s.onChange(o.value); setOpen(false); }}
          >
            <span className="flex-1">{o.label}</span>
            {on && <Check size={14} />}
          </button>
        );
      })}
    </>
  );

  return (
    <div ref={ref} className="relative shrink-0">
      <button
        type="button"
        className={`ss-ib !w-[34px] !h-[34px] ${open ? 'bg-sel text-sel-ink' : ''}`}
        aria-label={label}
        title={label}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <ArrowDownUp size={16} />
      </button>
      {open && (
        <div role="menu" aria-label={label} className="ss-menu absolute right-0 top-full mt-1 z-50 !w-[200px] animate-dropdown-in">
          {group && section(group)}
          {group && <hr />}
          {section(sort)}
          {display && (
            <>
              <hr />
              <span className="px-[9px] pt-1.5 pb-1 text-xs font-semibold text-ink-3">{display.label}</span>
              {display.options.map((o) => (
                <button key={o.label} type="button" role="menuitemcheckbox" aria-checked={o.checked} className="text-ink" onClick={o.onChange}>
                  <span className="flex-1">{o.label}</span>
                  <span className={`ss-sw sm ${o.checked ? 'on' : ''}`} aria-hidden><i /></span>
                </button>
              ))}
            </>
          )}
        </div>
      )}
    </div>
  );
}
