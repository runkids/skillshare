import { useId, useState, type ReactNode } from 'react';
import { Folder, FolderPlus } from 'lucide-react';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';

/** An absolute path, or one under ~, as the Projects page takes a folder. */
const isFolder = (path: string) => /^(~($|[\\/])|\/|[A-Za-z]:[\\/])/.test(path);

interface Props {
  /** The mcp.projects roots. */
  roots: string[];
  /** The projects that use the server now. */
  saved: string[];
  /** The projects ticked so far. */
  picked: string[];
  onChange: (picked: string[]) => void;
  disabled?: boolean;
  /** A line beside the count. */
  hint?: string;
  /** Actions at the end of the last row. */
  children?: ReactNode;
}

/**
 * The projects a server that loads only in chosen projects goes to, as ticks. A folder that
 * is not in mcp.projects yet can be added; saving it adds the project.
 */
export default function MCPProjectPicker({
  roots,
  saved,
  picked,
  onChange,
  disabled = false,
  hint,
  children,
}: Props) {
  const t = useT();
  const inputId = useId();
  const [added, setAdded] = useState<string[]>([]);
  const [adding, setAdding] = useState<string | null>(null);
  const rows = [...roots, ...added.filter((x) => !roots.includes(x))];
  const folder = (adding ?? '').trim().replace(/(?<=.)[\\/]+$/, '');
  const invalid = folder !== '' && !isFolder(folder);

  const tick = (root: string, on: boolean) =>
    onChange(on ? [...picked, root] : picked.filter((x) => x !== root));
  const add = () => {
    if (!isFolder(folder)) return;
    // The same folder written another way is already a row: tick that one.
    const known = rows.find((x) => shortenHome(x) === shortenHome(folder));
    if (!known) setAdded((prev) => [...prev, folder]);
    if (!picked.includes(known ?? folder)) tick(known ?? folder, true);
    setAdding(null);
  };

  const status = (root: string) => {
    if (picked.includes(root))
      return saved.includes(root) ? (
        <span className="shrink-0 text-xs text-ink-3">{t('mcp.projectsPick.inUse')}</span>
      ) : (
        <span className="ss-tag warn shrink-0">{t('mcp.projectsPick.willAdd')}</span>
      );
    return saved.includes(root) ? (
      <span className="ss-tag warn shrink-0">{t('mcp.projectsPick.willRemove')}</span>
    ) : null;
  };

  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[13px] font-semibold text-ink">{t('mcp.projectsPick.title')}</span>
        <span className="ss-cnt">
          {picked.length} / {rows.length}
        </span>
        {hint && <span className="text-xs text-ink-3">{hint}</span>}
      </div>
      {rows.length > 0 && (
        <div className="ss-list">
          {rows.map((root) => (
            <div key={root} className="ss-r !min-h-[42px] gap-2.5" title={root}>
              <Checkbox
                label={shortenHome(root)}
                checked={picked.includes(root)}
                onChange={(on) => tick(root, on)}
                disabled={disabled}
                className="min-w-0 flex-1 font-mono font-normal"
              />
              {status(root)}
            </div>
          ))}
        </div>
      )}
      {adding !== null && (
        <div className="flex items-start gap-2">
          <div className="ss-fld min-w-0 flex-1">
            <label htmlFor={inputId} className="sr-only">
              {t('projects.add.folder')}
            </label>
            <span className={`ss-inp font-mono ${invalid ? 'err' : ''}`}>
              <Folder size={15} className="shrink-0 text-ink-3" />
              <input
                id={inputId}
                autoFocus
                value={adding}
                onChange={(e) => setAdding(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    add();
                  }
                  if (e.key === 'Escape') {
                    // The folder box closes, not the dialog around it.
                    e.stopPropagation();
                    setAdding(null);
                  }
                }}
                placeholder="~/work/my-project"
              />
            </span>
            <span className={`hp ${invalid ? '!text-bad' : ''}`}>
              {t('projects.add.folderHint')}
            </span>
          </div>
          <Button size="sm" variant="secondary" onClick={add} disabled={!isFolder(folder)}>
            {t('mcp.projectsPick.add')}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setAdding(null)}>
            {t('common.cancel')}
          </Button>
        </div>
      )}
      <div className="flex flex-wrap items-center gap-2">
        {adding === null && (
          <Button size="sm" variant="ghost" onClick={() => setAdding('')} disabled={disabled}>
            <FolderPlus size={14} />
            {t('mcp.projectsPick.addFolder')}
          </Button>
        )}
        {children && <div className="ml-auto flex items-center gap-2">{children}</div>}
      </div>
    </div>
  );
}
