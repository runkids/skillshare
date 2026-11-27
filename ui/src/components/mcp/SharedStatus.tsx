import type { MCPOverride, MCPServer } from '../../api/mcp';
import { useT } from '../../i18n';
import { scalarText } from '../../lib/scalarText';
import { overrideParts } from './mcpView';

/** How each project loads a server that loads only in the projects that use it. */
export interface SharedLookup {
  /** The server as that project loads it, with the targets it goes to there. */
  loadedIn: (name: string, root: string) => MCPServer & { targets: string[] };
  overrideOf: (name: string, root: string) => MCPOverride | undefined;
  /** The project has its own server of this name, which loads in place of the shared one. */
  replaced: (name: string, root: string) => boolean;
}

/** What one project does with a shared server: its own server, its override, or what it inherits. */
export default function SharedStatus({
  name,
  root,
  using,
  shared,
}: {
  name: string;
  root: string;
  using: boolean;
  shared: SharedLookup;
}) {
  const t = useT();
  if (shared.replaced(name, root))
    return <span className="ss-tag shrink-0">{t('mcp.projectsPick.ownServer', { name })}</span>;
  if (!using) return null;
  const parts = overrideParts(t, shared.overrideOf(name, root));
  if (parts.length > 0)
    return <span className="ss-tag inf shrink-0">{parts.join(t('mcp.tools.partSeparator'))}</span>;
  const loaded = shared.loadedIn(name, root);
  const exposure = scalarText(loaded.piOptions?.exposure ?? 'codemode');
  return (
    <span className="shrink-0 text-xs text-ink-3">
      {loaded.targets.includes('pi')
        ? t('mcp.override.piDefault', {
            value: t(`mcp.piExposureShort.${exposure}`, undefined, exposure),
          })
        : t('mcp.projectsPick.inUse')}
    </span>
  );
}
