import { useId, type ReactNode } from 'react';
import { Folder, Globe, ToggleRight, X } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import { useT, plural } from '../../i18n';
import { relativeTo, shortenHome } from '../../lib/paths';
import { targetLabel } from './mcpView';

interface Props {
  root: string;
  /** Global servers that load here, of those shown. */
  global: { on: number; total: number };
  /** Shared servers this project turned on, of all shared servers. */
  shared: { on: number; total: number };
  /** Servers defined in this project that load. */
  own: number;
  /** The Agent files sync writes for this project, with their paths. */
  files: string[];
  paths: Record<string, string>;
  onClose: () => void;
}

// One fixed drawing, scrolled sideways on a narrow screen rather than squeezed.
const W = 920;
const H = 340;
const PROJECT = { x: 330, y: 122, w: 200, h: 96 };
const AGENTS_X = 620;

function Source({
  top,
  icon,
  title,
  line,
}: {
  top: number;
  icon: ReactNode;
  title: string;
  line: string;
}) {
  return (
    <div
      className="absolute left-4 flex h-16 w-[224px] items-center gap-2.5 rounded-lg border border-line bg-surface px-4 text-[13px] leading-[18px] text-ink-2"
      style={{ top }}
    >
      <span className="flex shrink-0">{icon}</span>
      <span className="min-w-0">
        <b className="block truncate text-sm text-ink">{title}</b>
        {line}
      </span>
    </div>
  );
}

/** How the three kinds of server reach this project, and which files sync writes. */
export default function MCPFlowDialog({ root, global, shared, own, files, paths, onClose }: Props) {
  const t = useT();
  // useId holds colons, which url(#…) would not match.
  const head = `flow-head-${useId().replace(/:/g, '')}`;
  const total = global.on + shared.on + own;
  // An arrow with nothing flowing along it is drawn faint and dashed.
  const arrows = [
    { d: `M240 72 H285 V158 H${PROJECT.x - 6}`, n: global.on },
    { d: `M240 170 H${PROJECT.x - 6}`, n: shared.on },
    { d: `M240 288 H285 V182 H${PROJECT.x - 6}`, n: own },
    { d: `M${PROJECT.x + PROJECT.w} 170 H${AGENTS_X - 6}`, n: total },
  ];
  const title = t('mcp.flow.title');
  return (
    <DialogShell open onClose={onClose} maxWidth="5xl" padding="none" ariaLabel={title}>
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
        <IconButton
          icon={<X size={16} />}
          label={t('common.close')}
          className="ml-auto"
          onClick={onClose}
        />
      </div>
      <div className="db">
        <div className="overflow-x-auto rounded-lg border border-line p-2">
          <div className="relative mx-auto" style={{ width: W, height: H }}>
            <div className="absolute left-0 top-0 h-[216px] w-[256px] rounded-lg bg-sunken" />
            <span className="absolute left-4 top-2.5 text-xs font-medium text-ink-3">
              {t('mcp.flow.defined')}
            </span>
            <svg
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 text-ink-2"
              width={W}
              height={H}
              fill="none"
            >
              <defs>
                <marker
                  id={head}
                  markerWidth="8"
                  markerHeight="8"
                  refX="1"
                  refY="4"
                  orient="auto"
                  markerUnits="userSpaceOnUse"
                >
                  <path d="M0 0 L8 4 L0 8 Z" fill="currentColor" />
                </marker>
              </defs>
              {arrows.map((a) => (
                <path
                  key={a.d}
                  d={a.d}
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeDasharray={a.n > 0 ? undefined : '6 6'}
                  opacity={a.n > 0 ? 1 : 0.35}
                  markerEnd={`url(#${head})`}
                />
              ))}
            </svg>
            <Source
              top={40}
              icon={<Globe size={16} />}
              title={t('mcp.projects.globalServers')}
              line={t('mcp.flow.global', global)}
            />
            <Source
              top={138}
              icon={<ToggleRight size={16} />}
              title={t('mcp.projects.shared')}
              line={t('mcp.flow.shared', shared)}
            />
            <Source
              top={256}
              icon={<Folder size={16} />}
              title={t('mcp.projects.onlyHere')}
              line={t('mcp.flow.own', { count: own })}
            />
            <div
              className="absolute flex flex-col items-center justify-center rounded-lg border-2 border-ink bg-surface px-4 text-center text-[13px] text-ink-2"
              style={{ left: PROJECT.x, top: PROJECT.y, width: PROJECT.w, height: PROJECT.h }}
            >
              <b
                className="max-w-full truncate font-mono text-sm text-ink"
                title={shortenHome(root)}
              >
                {root.split(/[\\/]/).filter(Boolean).pop() ?? root}
              </b>
              {t(plural('mcp.flow.loads', total), { count: total })}
            </div>
            <div
              className="absolute top-0 flex flex-col rounded-lg border border-line bg-surface px-3.5 py-3 text-[13px]"
              style={{ left: AGENTS_X, width: W - AGENTS_X, height: H }}
            >
              <span className="flex items-baseline gap-1.5 pb-1.5">
                <b className="text-sm">{t('mcp.flow.files')}</b>
                <span className="ss-cnt">{files.length}</span>
              </span>
              <div className="-mx-1 flex min-h-0 flex-col overflow-y-auto px-1">
                {files.map((x) => (
                  <span key={x} className="flex h-[30px] shrink-0 items-center gap-2">
                    <AgentIcon target={x} size={14} />
                    <span className="whitespace-nowrap">{targetLabel(x)}</span>
                    <span
                      className="min-w-0 flex-1 truncate text-right font-mono text-[11px] text-ink-3"
                      title={paths[x]}
                    >
                      {relativeTo(root, paths[x])}
                    </span>
                  </span>
                ))}
              </div>
            </div>
          </div>
        </div>
        <p className="text-xs text-ink-3">{t('mcp.flow.caption')}</p>
      </div>
    </DialogShell>
  );
}
