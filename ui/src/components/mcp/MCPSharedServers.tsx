import { Pencil, ToggleRight } from 'lucide-react';
import type { MCPServer, mcpApi } from '../../api/mcp';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import IconButton from '../IconButton';
import ProjectSectionHead from './ProjectSectionHead';
import ProjectServerRow from './ProjectServerRow';
import { overrideParts, sharedInRoot, targetLabel, writes, type MCPChange } from './mcpView';

type MCPList = Awaited<ReturnType<typeof mcpApi.list>>;

interface Props {
  root: string;
  /** The MCP page's config, with every server and what each project changes in them. */
  source: MCPList['source'];
  /** The projects that use each shared server. */
  users: Record<string, string[]>;
  /** The shared servers this project uses. */
  use: string[];
  /** This project's own servers; one of the same name cannot also be used. */
  own: Record<string, MCPServer>;
  changes: MCPChange[];
  busy: boolean;
  onToggle: (name: string, on: boolean) => void;
  /** Opens what this project changes in a shared server it uses. */
  onOverride: (name: string) => void;
}

/** The servers defined once for the projects that use them, each with a switch for this project. */
export default function MCPSharedServers({
  root,
  source,
  users,
  use,
  own,
  changes,
  busy,
  onToggle,
  onOverride,
}: Props) {
  const t = useT();
  const names = Object.keys(users).sort();
  return (
    <section className="mt-3 flex flex-col">
      <ProjectSectionHead
        icon={<ToggleRight size={16} />}
        title={t('mcp.projects.shared')}
        count={names.length}
        chip={t('mcp.projects.chipOff')}
        hint={t('mcp.projects.sharedHint')}
      />
      {names.length > 0 ? (
        <div className="ss-list">
          {names.map((n) => {
            const server = source.servers[n];
            const on = use.includes(n);
            const taken = Boolean(own[n]);
            const others = users[n].filter((r) => r !== root);
            const parts =
              on && !taken ? overrideParts(t, source.projects?.[root]?.overrides?.[n], server) : [];
            const icons = on ? sharedInRoot(source, n, root).targets : [];
            const iconsLabel = t('mcp.projects.onIn', {
              targets: icons.map(targetLabel).join(', '),
            });
            return (
              <ProjectServerRow
                key={n}
                name={n}
                server={server}
                tags={
                  <>
                    {parts.length > 0 && (
                      <span className="ss-tag inf">{t('mcp.override.done')}</span>
                    )}
                    {changes.some((c) => c.name === n && writes(c)) && (
                      <span className="ss-tag warn">{t('plugins.pending')}</span>
                    )}
                  </>
                }
                aside={
                  !taken && others.length > 0
                    ? t('mcp.projects.alsoIn', { projects: others.map(shortenHome).join(', ') })
                    : undefined
                }
                icons={icons}
                iconsLabel={iconsLabel}
                checked={on}
                disabled={busy || (taken && !on)}
                onToggle={() => onToggle(n, !on)}
                action={
                  on &&
                  !taken && (
                    <IconButton
                      icon={<Pencil size={14} />}
                      label={t(parts.length > 0 ? 'mcp.override.edit' : 'mcp.override.add')}
                      size="sm"
                      disabled={busy}
                      onClick={() => onOverride(n)}
                    />
                  )
                }
              >
                {taken && <span className="text-ink-3">{t('mcp.projects.overridden')}</span>}
                {parts.length > 0 && (
                  <span className="text-ink-3">
                    {t('mcp.override.summary', { parts: parts.join(t('mcp.tools.partSeparator')) })}
                  </span>
                )}
              </ProjectServerRow>
            );
          })}
        </div>
      ) : (
        <p className="text-[13px] text-ink-3">{t('mcp.projects.noShared')}</p>
      )}
    </section>
  );
}
