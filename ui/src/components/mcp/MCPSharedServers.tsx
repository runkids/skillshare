import { ToggleRight } from 'lucide-react';
import type { MCPServer } from '../../api/mcp';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import ProjectSectionHead from './ProjectSectionHead';
import ProjectServerRow from './ProjectServerRow';
import { targetLabel, writes, type MCPChange } from './mcpView';

interface Props {
  root: string;
  /** Every server of the global list, by name. */
  servers: Record<string, MCPServer>;
  /** The projects that use each shared server. */
  users: Record<string, string[]>;
  /** The shared servers this project uses. */
  use: string[];
  /** This project's own servers; one of the same name cannot also be used. */
  own: Record<string, MCPServer>;
  /** The Agents this project uses, which a shared server without its own targets follows. */
  targets: string[];
  changes: MCPChange[];
  busy: boolean;
  onToggle: (name: string, on: boolean) => void;
}

/** The servers defined once for the projects that use them, each with a switch for this project. */
export default function MCPSharedServers({
  root,
  servers,
  users,
  use,
  own,
  targets,
  changes,
  busy,
  onToggle,
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
            const server = servers[n];
            const on = use.includes(n);
            const taken = Boolean(own[n]);
            const others = users[n].filter((r) => r !== root);
            const icons = on ? (server.targets ?? targets) : [];
            const iconsLabel = t('mcp.projects.onIn', {
              targets: icons.map(targetLabel).join(', '),
            });
            return (
              <ProjectServerRow
                key={n}
                name={n}
                server={server}
                tags={
                  changes.some((c) => c.name === n && writes(c)) && (
                    <span className="ss-tag warn">{t('plugins.pending')}</span>
                  )
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
              >
                {taken && <span className="text-ink-3">{t('mcp.projects.overridden')}</span>}
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
