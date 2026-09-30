import { useState } from 'react';
import { Check, Folder, X } from 'lucide-react';
import { api, type ProjectList } from '../../api/client';
import { hooksApi } from '../../api/hooks';
import { mcpApi, mcpTargets } from '../../api/mcp';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import ProjectTools from './ProjectTools';
import { toolGroups } from './projectView';

interface Props {
  tools: ProjectList['tools'];
  common: string[];
  /** Absolute paths of the projects that exist */
  existing: string[];
  onClose: () => void;
  onAdded: (root: string) => void;
}

const PARTS = ['skills', 'agents', 'mcp', 'hooks'] as const;
type PartName = (typeof PARTS)[number];

/** Add one folder to projects. Filters and MCP servers are edited on the project's page. */
export default function AddProjectDialog({ tools, common, existing, onClose, onAdded }: Props) {
  const t = useT();
  const [path, setPath] = useState('');
  const [targets, setTargets] = useState<string[]>([]);
  const [parts, setParts] = useState<Record<PartName, boolean>>({ skills: true, agents: false, mcp: false, hooks: false });
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  // Trailing separators aside, this is the key config.yaml will hold.
  const root = path.trim().replace(/(?<=.)[\\/]+$/, '');
  const absolute = /^(~($|[\\/])|\/|[A-Za-z]:[\\/])/.test(root);
  const taken = existing.some((x) => x === root || shortenHome(x) === root);
  const pathError = root && !absolute ? t('projects.add.folderHint') : taken ? t('projects.add.taken') : '';
  const agentTools = targets.filter((x) => tools.find((tool) => tool.name === x)?.agentsPath);
  const folders = toolGroups(tools, targets).length;
  const canSave = Boolean(root) && !pathError && targets.length > 0 && PARTS.some((p) => parts[p]) && !saving;

  const partHint = (p: PartName) => {
    if (p === 'agents' && targets.length > 0) {
      return agentTools.length === 0 ? t('projects.add.agentsNone') : agentTools.length < targets.length ? t('projects.add.agentsSome', { targets: agentTools.join(', ') }) : t('projects.add.agentsHint');
    }
    return t(`projects.add.${p}Hint`);
  };

  const save = async () => {
    if (!canSave) return;
    setSaving(true);
    setError('');
    const all = { mode: '', include: [], exclude: [] };
    try {
      const saved = await api.saveProject({ create: true, root, targets, skills: parts.skills ? all : null, agents: parts.agents ? all : null });
      if (parts.mcp) {
        const clients = mcpTargets.filter((x) => targets.includes(x));
        await mcpApi.save({ project: root, settings: { targets: clients.length > 0 ? clients : undefined } });
      }
      // An empty hooks block: the project's own hooks are added on its Hooks tab.
      if (parts.hooks) await hooksApi.save({ project: saved.root });
      onAdded(saved.root);
    } catch (e) {
      setError((e as Error).message);
      setSaving(false);
    }
  };

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={saving} ariaLabel={t('projects.add.title')} className="!max-w-[640px]">
      <div className="dh">
        <h2 className="ss-h2">{t('projects.add.title')}</h2>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={saving}><X size={16} /></button>
      </div>
      <form id="add-project" className="db" onSubmit={(e) => { e.preventDefault(); void save(); }}>
        <div className="ss-fld">
          <label htmlFor="add-project-path">{t('projects.add.folder')}</label>
          <span className={`ss-inp font-mono ${pathError ? 'err' : ''}`}>
            <Folder size={15} className="shrink-0 text-ink-3" />
            <input id="add-project-path" autoFocus value={path} onChange={(e) => setPath(e.target.value)} placeholder="~/work/my-project" disabled={saving} />
          </span>
          <span className={`hp ${pathError ? '!text-bad' : ''}`}>{pathError || t('projects.add.folderHint')}</span>
        </div>
        <div className="ss-fld">
          <span className="text-[13px] font-semibold">{t('projects.targets')}</span>
          <ProjectTools tools={tools} common={common} selected={targets} onChange={setTargets} disabled={saving} />
          <span className="hp">{t('projects.add.targetsHint')}</span>
        </div>
        <div className="ss-fld">
          <span className="text-[13px] font-semibold">{t('projects.add.parts')}</span>
          <div className="flex flex-col gap-2.5">
            {PARTS.map((p) => (
              <button key={p} type="button" role="checkbox" aria-checked={parts[p]} className={`ss-pick text-left ${parts[p] ? 'on' : ''}`} onClick={() => setParts({ ...parts, [p]: !parts[p] })} disabled={saving}>
                <span className={`ss-chk ${parts[p] ? 'on' : ''}`}>{parts[p] && <Check size={12} strokeWidth={3} />}</span>
                <span className="flex flex-col gap-0.5">
                  <span className="font-semibold">{p === 'mcp' ? 'MCP' : p === 'hooks' ? 'Hooks' : p === 'agents' ? 'Agents' : 'Skills'}</span>
                  <span className="text-[13px] text-ink-2">{partHint(p)}</span>
                </span>
              </button>
            ))}
          </div>
        </div>
        {error && <div className="ss-note bad"><span className="flex-1">{error}</span></div>}
      </form>
      <div className="df">
        <span className="flex-1 text-[13px] text-ink-2">{folders > 0 && t(folders === 1 ? 'projects.add.writes.one' : 'projects.add.writes.other', { count: folders })}</span>
        <Button variant="ghost" onClick={onClose} disabled={saving}>{t('common.cancel')}</Button>
        <Button type="submit" form="add-project" variant="primary" loading={saving} disabled={!canSave}>{t('projects.add.submit')}</Button>
      </div>
    </DialogShell>
  );
}
