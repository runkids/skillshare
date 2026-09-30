import { CircleCheck } from 'lucide-react';

import type { ChangeGroup } from './sync/syncView';

import AgentIcon from './AgentIcon';
import { hookLabel } from './hooks/hooksView';
import { targetLabel } from './mcp/mcpView';
import { baseName } from './projects/projectView';
import { useT } from '../i18n';
import { shortenHome } from '../lib/paths';

/** Friendly empty state for a sync with nothing to write. */
export function SyncUpToDate({ text }: { text: string }) {
  return (
    <div className="flex flex-col items-center gap-3 py-8 text-center">
      <span className="grid size-12 place-items-center rounded-full bg-ok-bg text-ok"><CircleCheck size={24} /></span>
      <p className="text-[14px] text-ink-2">{text}</p>
    </div>
  );
}

/** One compact row per target with changes; targets already in sync fold into a single closing line. */
export default function SyncResultList({ groups, inSync, className = '' }: { groups: ChangeGroup[]; inSync: string[]; className?: string }) {
  const t = useT();
  // A folder name is enough unless two projects share it.
  const projects = [...new Set(groups.flatMap((g) => g.project ?? []))];
  const projectLabel = (p: string) => (projects.some((q) => q !== p && baseName(q) === baseName(p)) ? shortenHome(p) : baseName(p));

  return (
    <div className={`ss-list ${className}`}>
      {groups.map((g) => {
        const count = (match: (r: ChangeGroup['rows'][number]) => boolean) => g.rows.filter(match).length;
        const folder = count((r) => r.text === 'sync.row.folder');
        // An MCP switch adds or removes no server: it turns a global one off in a project, or back on.
        const off = count((r) => r.icon === 'add' && r.switch === true);
        const on = count((r) => r.icon === 'remove' && r.switch === true);
        const linked = count((r) => r.icon === 'add' && !r.switch) - folder;
        const updated = count((r) => r.icon === 'update');
        const adopted = count((r) => r.icon === 'adopt');
        const pruned = count((r) => r.icon === 'remove' && !r.switch);
        const kept = count((r) => r.icon === 'kept');
        const conflict = count((r) => r.icon === 'conflict');
        const mcp = g.part === 'mcp';
        const hooks = g.part === 'hooks';
        const listed = mcp || hooks;
        return (
          <div key={g.key} className="ss-r">
            <span className="ss-at"><AgentIcon target={g.name} size={15} /></span>
            <span className={`min-w-0 truncate text-[13px] font-semibold ${listed ? 'shrink-0' : 'flex-1'}`} title={g.path ?? g.name}>{mcp ? targetLabel(g.name) : hooks ? hookLabel(g.name) : g.name}</span>
            {/* One Agent file holds many servers: name the ones that change, and the project the file is for. */}
            {listed && <span className="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-3">{[g.project && projectLabel(g.project), g.rows.map((r) => r.name).join(', ')].filter(Boolean).join(' · ')}</span>}
            <span className="flex shrink-0 flex-wrap justify-end gap-1.5">
              {folder > 0 && <span className="ss-tag inf">{t('syncResult.dirCreated')}</span>}
              {linked > 0 && <span className="ss-tag ok">{t(listed ? 'syncResult.mcp.add' : 'syncResult.linked', { count: linked })}</span>}
              {updated > 0 && <span className="ss-tag inf">{t(listed ? 'syncResult.mcp.update' : 'syncResult.updated', { count: updated })}</span>}
              {adopted > 0 && <span className="ss-tag inf">{t('syncResult.mcp.adopt', { count: adopted })}</span>}
              {pruned > 0 && <span className="ss-tag warn">{t(listed ? 'syncResult.mcp.remove' : 'syncResult.pruned', { count: pruned })}</span>}
              {off > 0 && <span className="ss-tag warn">{t('syncResult.mcp.off', { count: off })}</span>}
              {on > 0 && <span className="ss-tag ok">{t('syncResult.mcp.on', { count: on })}</span>}
              {kept > 0 && <span className="ss-tag">{t('syncResult.skipped', { count: kept })}</span>}
              {conflict > 0 && <span className="ss-tag bad">{t('syncResult.mcp.conflict', { count: conflict })}</span>}
            </span>
          </div>
        );
      })}
      {inSync.length > 0 && (
        <div className="ss-r text-[13px] text-ink-3" title={inSync.join(', ')}>
          <CircleCheck size={16} className="shrink-0 text-ok" />
          {t(inSync.length === 1 ? 'syncResult.upToDate.one' : 'syncResult.upToDate.other', { count: inSync.length })}
        </div>
      )}
    </div>
  );
}
