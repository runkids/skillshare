import { useState } from 'react';
import type { ReactNode } from 'react';
import { ArrowLeft, Bot, Check, ChevronDown, FileText, Folder, FolderPlus, Layers, Pencil, Plug, Plus, Search, SquareTerminal, Users, X } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { api, type AvailableTarget, type Target } from '../../api/client';
import { mcpTargets } from '../../api/mcp';
import { staleTimes } from '../../lib/queryKeys';
import { useAppContext } from '../../context/AppContext';
import { shortenHome } from '../../lib/paths';
import { useI18n } from '../../i18n';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { Checkbox } from '../Input';
import { setupPathProblem } from '../instructions/instructionsView';
import { mcpClient } from '../mcp/mcpView';
import { useMcpQuery } from '../../hooks/useSharedQueries';

const PREVIEW_COUNT = 5;
const SHARED_ICONS = 5;

// An account's executable is a name looked up on PATH or a full path; a relative path would
// depend on where skillshare runs, and nothing goes through a shell to split arguments.
const cliProblem = (cli: string) => (/[\\/]/.test(cli) ? !/^(\/|~[\\/]|[A-Za-z]:[\\/])/.test(cli) : /\s/.test(cli));

function FolderField({ id, label, value, onChange, hint, placeholder, disabled }: {
  id: string; label: string; value: string; onChange: (v: string) => void; hint: string; placeholder?: string; disabled: boolean;
}) {
  return (
    <div className="ss-fld">
      <label htmlFor={id}>{label}</label>
      <span className="ss-inp">
        <Folder size={15} className="shrink-0 text-ink-3" />
        <input id={id} className="font-mono" value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} disabled={disabled} />
      </span>
      <span className="hp">{hint}</span>
    </div>
  );
}

/** Pick a known tool (defaults filled from targets.yaml), describe a custom one, or add another account of a known one. */
export default function AddTargetDialog({ available, initial, existing, targets = [], onClose, onAdded }: {
  available: AvailableTarget[];
  initial?: string;
  existing: string[];
  /** Configured targets, to say what a tool that reads one of their folders would see */
  targets?: Target[];
  onClose: () => void;
  onAdded: (name: string) => void;
}) {
  const { t } = useI18n();
  const { isProjectMode } = useAppContext();
  const pool = available.filter((a) => !a.installed).sort((a, b) => a.name.localeCompare(b.name));
  const [query, setQuery] = useState('');
  const [showAll, setShowAll] = useState(false);
  const [mode, setMode] = useState<'known' | 'custom' | 'account'>('known');
  const custom = mode !== 'known';
  // Another account is another config folder of an Agent; its paths follow the folder.
  const accountAgents = available.filter((a) => a.configDir).sort((a, b) => a.name.localeCompare(b.name));
  const [account, setAccount] = useState({ agent: accountAgents[0]?.name ?? '', dir: '', named: false, cli: '' });
  const [draft, setDraft] = useState(() => {
    const first = pool.find((a) => a.name === initial) ?? pool.find((a) => a.name === 'universal') ?? pool.find((a) => a.detected);
    return { name: first?.name ?? '', path: first?.path ?? '', agentPath: first?.agentPath ?? '' };
  });
  // A tool that already reads another target's skills folder gets no second copy unless asked.
  const [skills, setSkills] = useState(() => !(pool.find((a) => a.name === initial) ?? pool.find((a) => a.name === 'universal') ?? pool.find((a) => a.detected))?.readsFrom?.length);
  // A custom tool's instruction file, so its file tab works right after adding it.
  const [instructions, setInstructions] = useState({ path: '', import: false });
  const instructionsProblem = mode === 'custom' ? setupPathProblem(instructions.path, isProjectMode) : null;
  const instructionsExample = `${isProjectMode ? '' : '~/'}.${draft.name.trim() || 'my-tool'}/AGENTS.md`;
  // Which folder is open for editing; the rest show as one line each.
  const [editing, setEditing] = useState<'skills' | 'agents' | null>(null);
  const mcp = useMcpQuery({ staleTime: staleTimes.extras, enabled: !isProjectMode });
  const mcpPaths = mcp.data?.paths ?? {};
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const q = query.trim().toLowerCase();
  const matches = q ? pool.filter((a) => a.name.toLowerCase().includes(q)) : pool;
  // universal is one folder many tools read; until it is a target it leads the list with who reads it.
  const shared = matches.find((a) => a.name === 'universal');
  const found = matches.filter((a) => a.detected && a !== shared);
  const others = matches.filter((a) => !a.detected && a !== shared);
  const shownOthers = q || showAll ? others : others.slice(0, PREVIEW_COUNT);
  const known = pool.find((a) => a.name === draft.name);

  const taken = custom && existing.includes(draft.name.trim());
  const accountAgent = accountAgents.find((a) => a.name === account.agent);
  const dir = account.dir.trim().replace(/[\\/]+$/, '');
  const moved = (path?: string) => (path && accountAgent?.configDir && dir && path.startsWith(accountAgent.configDir) ? dir + path.slice(accountAgent.configDir.length) : '');
  // Codex reads the shared ~/.agents/skills, outside its config folder; an account's skills
  // are always in its own folder.
  const accountSkills = dir && accountAgent ? moved(accountAgent.path) || `${dir}/skills` : '';
  const cli = account.cli.trim();
  const cliInvalid = mode === 'account' && cliProblem(cli);
  const canAdd = Boolean(draft.name.trim()) && !taken && !instructionsProblem && (mode === 'account' ? Boolean(dir && accountAgent) && !cliInvalid : Boolean(draft.path.trim()) && (custom || Boolean(known)));
  const add = async () => {
    const name = draft.name.trim();
    setBusy(true);
    setError('');
    try {
      if (mode === 'account') await api.addAgentConfigDir(name, account.agent, dir, cli || undefined);
      else await api.addTarget(name, draft.path.trim(), draft.agentPath.trim() || undefined, mode === 'custom' && instructions.path.trim() ? { path: instructions.path.trim(), import: instructions.import } : undefined, skills);
      onAdded(name);
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  };
  const open = (next: typeof mode) => {
    setMode(next);
    setError('');
    setDraft({ name: '', path: '', agentPath: '' });
    setSkills(true);
    setInstructions({ path: '', import: false });
    setAccount({ agent: accountAgents[0]?.name ?? '', dir: '', named: false, cli: '' });
  };
  // The name follows the folder (~/.claude-work gives claude-work) until it is typed.
  const setDir = (value: string) => {
    setAccount({ ...account, dir: value });
    if (!account.named) setDraft({ ...draft, name: value.trim().replace(/[\\/]+$/, '').split(/[\\/]/).pop()!.replace(/^\.+/, '') });
  };

  const readsHint = (a: AvailableTarget) => {
    const from = targets.find((tg) => tg.name === a.readsFrom?.[0]);
    if (!from) return t('targets.add.readsFromHintShort', { name: a.name, from: a.readsFrom?.[0] ?? '' });
    return t(from.linkedCount === 1 ? 'targets.add.readsFromHint.one' : 'targets.add.readsFromHint.other', { name: a.name, from: from.name, path: shortenHome(from.path), count: from.linkedCount });
  };

  const mcpWrites = (a: AvailableTarget) => (mcpTargets as readonly string[]).includes(mcpClient(a.name));
  const managedMark = <span className="flex items-center gap-1 text-[12px] text-ok"><Check size={13} strokeWidth={2.5} />{t('targets.add.managed')}</span>;
  // One line per thing skillshare writes for the picked tool: what, where, and its control.
  const writeRow = (key: string, Icon: LucideIcon, label: string, value: ReactNode, control?: ReactNode) => (
    <div key={key} className="grid grid-cols-[104px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line py-2 first-of-type:border-t-0">
      <span className="flex items-center gap-1.5 text-[13px] font-semibold"><Icon size={13} className="shrink-0 text-ink-3" />{label}</span>
      <span className="flex min-w-0 flex-col">{value}</span>
      {control || <span />}
    </div>
  );
  const row = (a: AvailableTarget, intro?: ReactNode) => {
    const on = !custom && draft.name === a.name;
    const reads = a.readsFrom?.length ? a.readsFrom : null;
    return (
      <div key={a.name} className={`ss-r !block !p-0 ${on ? 'sel' : ''}`}>
        <button
          type="button"
          role="radio"
          aria-checked={on}
          className="flex w-full items-center gap-3 px-4 py-2.5 text-left"
          onClick={() => { setDraft({ name: a.name, path: a.path, agentPath: a.agentPath ?? '' }); setSkills(!reads); setEditing(null); }}
          disabled={busy}
        >
          <span className="ss-at"><AgentIcon target={a.name} size={17} /></span>
          <span className="flex min-w-0 flex-col">
            <span className="flex items-center gap-2">
              <span className="font-semibold">{a.name}</span>
              {reads && <span className="ss-tag">{t('targets.add.readsTag', { target: reads[0] })}</span>}
            </span>
            <span className="truncate font-mono text-[12px] text-ink-3">{shortenHome(a.path)}</span>
          </span>
          {on && <span className="ml-auto grid h-[22px] w-[22px] shrink-0 place-items-center rounded-full bg-ink text-surface"><Check size={13} strokeWidth={3} /></span>}
        </button>
        {intro}
        {on && (
          <div className="flex flex-col pb-3 pl-[60px] pr-4">
            <span className="pb-1.5 text-[12px] text-ink-3">{t('targets.add.writes')}</span>
            {writeRow('skills', Layers, 'Skills', skills ? (
              editing === 'skills'
                ? <FolderField id="target-path" label={t('targets.add.skillsFolder')} value={draft.path} onChange={(path) => setDraft({ ...draft, path })} hint={a.detected ? t('targets.add.detectedHint', { name: a.name }) : t('targets.add.createdHint')} disabled={busy} />
                : <span className="truncate font-mono text-[12.5px]">{shortenHome(draft.path)}</span>
            ) : <span className="truncate text-[12.5px] text-ink-3">{reads ? readsHint(a) : t('targets.add.skillsOffValue')}</span>, (
              <span className="flex items-center gap-1">
                {skills && editing !== 'skills' && <button type="button" className="ss-ib !h-7 !w-7" aria-label={t('targets.add.changeFolder', { name: 'Skills' })} onClick={() => setEditing('skills')} disabled={busy}><Pencil size={13} /></button>}
                <button type="button" role="switch" aria-checked={skills} aria-label={t('targets.add.syncSkills')} className="grid h-7 place-items-center" onClick={() => setSkills(!skills)} disabled={busy}><span className={`ss-sw ${skills ? 'on' : ''}`}><i /></span></button>
              </span>
            ))}
            {a.agentPath && writeRow('agents', Bot, 'Agents', editing === 'agents'
              ? <FolderField id="target-agent-path" label={t('targets.add.agentsFolder')} value={draft.agentPath} onChange={(agentPath) => setDraft({ ...draft, agentPath })} hint={t('targets.add.agentsHint')} disabled={busy} />
              : <span className="truncate font-mono text-[12.5px]">{shortenHome(draft.agentPath)}</span>,
            editing !== 'agents' && <button type="button" className="ss-ib !h-7 !w-7" aria-label={t('targets.add.changeFolder', { name: 'Agents' })} onClick={() => setEditing('agents')} disabled={busy}><Pencil size={13} /></button>)}
            {mcpWrites(a) && writeRow('mcp', Plug, 'MCP', <span className="truncate font-mono text-[12.5px]">{mcpPaths[mcpClient(a.name)] ? shortenHome(mcpPaths[mcpClient(a.name)]) : t('targets.add.mcpConfig')}</span>, managedMark)}
            {a.instructionsFile && writeRow('instructions', FileText, a.instructionsFile, <span className="truncate font-mono text-[12.5px]">{shortenHome(a.instructionsPath ?? a.instructionsFile)}</span>, managedMark)}
          </div>
        )}
      </div>
    );
  };

  const title = t({ known: 'targets.add.title', custom: 'targets.add.customTitle', account: 'targets.add.accountTitle' }[mode]);
  const subtitle = t({ known: 'targets.add.subtitle', custom: 'targets.add.customSubtitle', account: 'targets.add.accountSubtitle' }[mode]);
  const nameField = (
    <div className="ss-fld">
      <label htmlFor="target-name">{t('targets.add.name')}</label>
      <span className={`ss-inp ${taken ? 'err' : ''}`}>
        <input id="target-name" autoFocus={mode === 'custom'} value={draft.name} onChange={(e) => { setDraft({ ...draft, name: e.target.value }); setAccount({ ...account, named: true }); }} placeholder={mode === 'account' ? `${account.agent}-work` : 'my-tool'} disabled={busy} />
      </span>
      <span className={`hp ${taken ? '!text-bad' : ''}`}>{taken ? t('targets.add.nameTaken') : t('targets.add.nameHint')}</span>
    </div>
  );
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[720px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{subtitle}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <form id="add-target" className="db" onSubmit={(e) => { e.preventDefault(); if (canAdd) void add(); }}>
        {mode === 'account' ? (
          <>
            <div className="flex flex-wrap gap-x-5 gap-y-3" role="radiogroup" aria-label={t('targets.add.accountAgent')}>
              {accountAgents.map((a) => (
                <button key={a.name} type="button" role="radio" aria-checked={a.name === account.agent} aria-label={a.name} className={`ss-tgl ${a.name === account.agent ? 'on' : ''}`} onClick={() => setAccount({ ...account, agent: a.name })} disabled={busy}>
                  <span className="ic"><AgentIcon target={a.name} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                  {a.name}
                </button>
              ))}
            </div>
            <FolderField id="target-config-dir" label={t('targets.add.accountFolder')} value={account.dir} onChange={setDir} placeholder={accountAgent ? `${shortenHome(accountAgent.configDir!)}-work` : ''} hint={t('targets.add.accountFolderHint', { name: account.agent })} disabled={busy} />
            <div className="ss-fld">
              <label htmlFor="target-cli">{t('targets.add.accountCli')}</label>
              <span className={`ss-inp ${cliInvalid ? 'err' : ''}`}>
                <SquareTerminal size={15} className="shrink-0 text-ink-3" />
                <input id="target-cli" className="font-mono" value={account.cli} onChange={(e) => setAccount({ ...account, cli: e.target.value })} placeholder={account.agent} spellCheck={false} autoComplete="off" disabled={busy} />
              </span>
              <span className={`hp ${cliInvalid ? '!text-bad' : ''}`}>{cliInvalid ? t('targets.add.accountCliInvalid') : t('targets.add.accountCliHint', { name: account.agent })}</span>
            </div>
            {nameField}
            {dir && (
              <div className="ss-note inf">
                <span className="flex flex-1 flex-col gap-1">
                  <span>{t('targets.add.accountWrites')}</span>
                  {[accountSkills, moved(accountAgent?.agentPath)].filter(Boolean).map((path) => <span key={path} className="break-all font-mono text-[12px]">{path}</span>)}
                  {cli && !cliInvalid && (
                    <>
                      <span className="mt-1">{t('targets.add.accountCliRuns')}</span>
                      <span className="break-all font-mono text-[12px]">{cli}</span>
                    </>
                  )}
                </span>
              </div>
            )}
          </>
        ) : custom ? (
          <>
            {nameField}
            <div className="flex flex-col gap-1">
              <Checkbox label={t('targets.add.syncSkills')} checked={skills} onChange={setSkills} size="sm" disabled={busy} />
              <span className="pl-[26px] text-[12.5px] text-ink-3">{t('targets.add.syncSkillsHint')}</span>
            </div>
            <FolderField id="target-path" label={t('targets.add.skillsFolder')} value={draft.path} onChange={(path) => setDraft({ ...draft, path })} placeholder="~/tools/my-tool/skills" hint={t('targets.add.customSkillsHint')} disabled={busy} />
            <FolderField id="target-agent-path" label={t('targets.add.agentsFolder')} value={draft.agentPath} onChange={(agentPath) => setDraft({ ...draft, agentPath })} placeholder={t('targets.add.optional')} hint={t('targets.add.customAgentsHint')} disabled={busy} />
            <div className="ss-fld">
              <label htmlFor="target-instructions">{t('targets.add.instructionsFile')}</label>
              <span className={`ss-inp ${instructionsProblem ? 'err' : ''}`}>
                <FileText size={15} className="shrink-0 text-ink-3" />
                <input id="target-instructions" className="font-mono" value={instructions.path} onChange={(e) => setInstructions({ ...instructions, path: e.target.value })} placeholder={instructionsExample} spellCheck={false} autoComplete="off" disabled={busy} />
              </span>
              <span className={`hp ${instructionsProblem ? '!text-bad' : ''}`}>
                {instructionsProblem ? t(`instructions.setup.problem.${instructionsProblem}`) : t(isProjectMode ? 'targets.add.instructionsHintProject' : 'targets.add.instructionsHint')}
              </span>
            </div>
            {instructions.path.trim() && <Checkbox label={t('instructions.setup.import')} checked={instructions.import} onChange={(v) => setInstructions({ ...instructions, import: v })} size="sm" disabled={busy} />}
            {skills && <div className="ss-note inf"><span className="flex-1">{t('targets.add.createdHint')}</span></div>}
          </>
        ) : (
          <>
            <span className="ss-inp">
              <Search size={15} className="shrink-0 text-ink-3" />
              <input autoFocus value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') e.preventDefault(); }} placeholder={t(pool.length === 1 ? 'targets.add.search.one' : 'targets.add.search.other', { count: pool.length })} aria-label={t(pool.length === 1 ? 'targets.add.search.one' : 'targets.add.search.other', { count: pool.length })} />
            </span>
            <div className="ss-list max-h-[440px] overflow-y-auto !shadow-none" role="radiogroup" aria-label={t('targets.add.title')}>
              {shared && (
                <>
                  <div className="ss-gh text-ink-2"><span className="flex-1">{t('targets.add.shared')}</span></div>
                  {row(shared, (
                    <div className="flex flex-col gap-1.5 pb-3 pl-[60px] pr-4 text-[13px] text-ink-2">
                      <span>{t('targets.add.sharedHint')}</span>
                      {(shared.readBy?.length ?? 0) > 0 && (
                        <span className="flex flex-wrap items-center gap-2">
                          {t('targets.add.sharedReadBy')}
                          <span className="ss-stack" role="img" aria-label={shared.readBy!.join(', ')} title={shared.readBy!.join(', ')}>
                            {shared.readBy!.slice(0, SHARED_ICONS).map((name) => <span key={name} className="ss-at !h-5 !w-5"><AgentIcon target={name} size={11} /></span>)}
                          </span>
                          {shared.readBy!.length > SHARED_ICONS && <span className="text-[12.5px] text-ink-3">{t('targetDetail.skillsOff.readers.more', { count: shared.readBy!.length - SHARED_ICONS })}</span>}
                        </span>
                      )}
                    </div>
                  ))}
                </>
              )}
              {found.length > 0 && (
                <>
                  <div className="ss-gh text-ink-2"><span className="flex-1">{t('targets.foundOnMachine')}</span><span className="text-ink-3">{found.length}</span></div>
                  {found.map((a) => row(a))}
                </>
              )}
              {others.length > 0 && (
                <>
                  <div className="ss-gh text-ink-2"><span className="flex-1">{t('targets.add.allTools')}</span><span className="text-ink-3">{others.length}</span></div>
                  {shownOthers.map((a) => row(a))}
                  {shownOthers.length < others.length && (
                    <button type="button" className="ss-r !min-h-10 w-full text-[13px] text-ink-2 hover:text-ink" onClick={() => setShowAll(true)}>
                      <span className="flex-1 text-left">{t(others.length - shownOthers.length === 1 ? 'targets.add.moreTools.one' : 'targets.add.moreTools.other', { count: others.length - shownOthers.length })}</span>
                      <ChevronDown size={15} />
                    </button>
                  )}
                </>
              )}
              {matches.length === 0 && (
                <div className="ss-r text-[13px] text-ink-2">{q ? t('targets.add.noMatch', { query: query.trim() }) : t('targets.add.allAdded')}</div>
              )}
            </div>
            <button type="button" className="flex w-fit items-center gap-2 text-[13px] font-semibold hover:text-accent" onClick={() => open('custom')} disabled={busy}>
              <FolderPlus size={15} />
              {t('targets.add.customLink')}
            </button>
            {accountAgents.length > 0 && (
              <button type="button" className="flex w-fit items-center gap-2 text-[13px] font-semibold hover:text-accent" onClick={() => open('account')} disabled={busy}>
                <Users size={15} />
                {t('targets.add.accountLink')}
              </button>
            )}
          </>
        )}
        {error && <div className="ss-note bad"><span className="flex-1">{error}</span></div>}
      </form>
      <div className="df">
        {custom ? (
          <Button variant="ghost" onClick={() => open('known')} disabled={busy}><ArrowLeft size={15} />{t('targets.add.back')}</Button>
        ) : (
          <span className="text-[13px] text-ink-2">{t('targets.add.modeHint')}</span>
        )}
        <span className="flex-1" />
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
        <Button variant="primary" type="submit" form="add-target" loading={busy} disabled={!canAdd}>
          {!busy && <Plus size={15} />}
          {mode === 'custom' || !draft.name.trim() ? t('targets.addTarget') : t('targets.add.addNamed', { name: draft.name.trim() })}
        </Button>
      </div>
    </DialogShell>
  );
}
