import { useContext, useEffect, useState } from 'react';
import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { Braces, Check, Download, FileUp, Info, Plus, X } from 'lucide-react';
import { mcpApi, type MCPCandidate, type MCPImportSource, type MCPMutation, type MCPServer } from '../../api/mcp';
import MCPConfigView from './MCPConfigView';
import PiSettingsFields from './PiSettingsFields';
import { piOptionsError } from './mcpServerDraft';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import CodeEditor from '../CodeEditor';
import DialogShell from '../DialogShell';
import SegmentedControl from '../SegmentedControl';
import { Checkbox, Select } from '../Input';
import { useToast } from '../Toast';
import { useT } from '../../i18n';
import { useAppContext } from '../../context/AppContext';
import { shortenHome } from '../../lib/paths';
import { describeEndpoint, describeError, hasToolPolicy, targetLabel, parsePiOptions } from './mcpView';
import { MCPTargetOrder } from './targetOrder';

/** Where the configuration comes from. Each entry point fixes one; the dialog never switches. */
type Source = 'target' | 'paste';

const SNIPPET_PLACEHOLDER = `{
  "mcpServers": {
    "linear": { "command": "npx", "args": ["-y", "mcp-remote", "https://mcp.linear.app/sse"] }
  }
}`;
const TOML = /^\s*\[mcp_servers[.\]]/m;
const GOOSE_YAML = /^extensions\s*:/m;

/** Pretty-printed JSON, or null when the text isn't plain JSON (comments and trailing commas still import fine). */
function formatJSON(text: string) {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return null;
  }
}

interface Props {
  source: Source;
  servers: Record<string, MCPServer>;
  defaultTargets: string[];
  paths: Record<string, string>;
  importSources?: MCPImportSource[];
  /** The exact file behind an unmanaged entry or conflict. */
  defaultPath?: string;
  detected: string[];
  /** A conflicting entry to take over: it may replace the source server of the same name. */
  conflict?: { target: string; name: string };
  /** A root under mcp.projects: the import is written to that project instead of the global source,
   *  and an import from an Agent reads that project's file of it. */
  project?: string;
  /** The Agent file selected first, such as one found to hold servers skillshare does not manage. */
  defaultFrom?: string;
  /** Overrides the targets offered, for a project where only some Agents have a file. */
  availableTargets?: readonly string[];
  /** Present when this is the paste half of "add a server", so the user can swap back to the form. */
  onMode?: (mode: 'form' | 'paste') => void;
  onClose: () => void;
  onImported: () => void;
}

// Pi has two files, told apart by piExtension; the path shown is what tells them apart for the user.
const sourceKey = (s: MCPImportSource) => `${s.target}:${s.piExtension ?? ''}`;

function importSourceChoices({ paths, importSources, detected, conflict, defaultFrom, defaultPath }: Props, order: readonly string[]) {
  // An account of an Agent has its own file, so it is an import source under its own name.
  const sourcesByTarget = new Map<string, MCPImportSource[]>();
  for (const file of importSources ?? Object.entries(paths).map(([target, path]) => ({ target, path }))) {
    const files = sourcesByTarget.get(file.target) ?? [];
    files.push(file);
    sourcesByTarget.set(file.target, files);
  }
  const sources = order.flatMap((target) => sourcesByTarget.get(target) ?? []);
  const detectedTargets = new Set(detected);
  const initialTarget = conflict?.target ?? defaultFrom ?? sources.find((s) => detectedTargets.has(s.target))?.target ?? sources[0]?.target;
  const initialSources = sources.filter((s) => s.target === initialTarget);
  const initialSource = initialSources.find((s) => s.path === defaultPath) ?? initialSources[0];
  return { sources, initialSource };
}

interface ImportSelection {
  candidates: MCPCandidate[];
  servers: Record<string, MCPServer>;
  conflict: Props['conflict'];
  picked: string[] | null;
  targets: string[];
  defaultTargets: string[];
  previewName: string;
  tab: Source;
}

function importSelection({ candidates, servers, conflict, picked, targets, defaultTargets, previewName, tab }: ImportSelection) {
  const exists = (c: MCPCandidate) => c.name in servers && c.name !== conflict?.name;
  const importable = candidates.filter((c) => c.problems.length === 0 && !exists(c));
  // Everything importable starts ticked; a conflict import starts with just that entry
  const selected = picked ?? importable.filter((c) => !conflict || c.name === conflict.name).map((c) => c.name);
  const chosen = importable.filter((c) => selected.includes(c.name));
  const piChosen = targets.includes('pi') || chosen.some((c) => servers[c.name]?.targets?.includes('pi'));
  // Pi's settings are checked as the form checks them, so an import cannot save what the form refuses.
  const piProblem = piChosen ? chosen.map((c) => ({ name: c.name, options: parsePiOptions(JSON.stringify(c.server.piOptions ?? {}), hasToolPolicy(c.server.tools)) })).find((p) => !p.options.value && Object.keys(p.options).length > 0) : undefined;
  const incompatible = Boolean(piProblem);
  const preview = chosen.find((c) => c.name === previewName) ?? chosen[0];

  const showList = tab === 'target' || candidates.length > 1 || candidates.some((c) => c.problems.length || c.warnings.length || exists(c));

  // An import that matches the inherited targets keeps inheriting them, like the server dialog
  const inherited = targets.length > 0 && targets.length === defaultTargets.length && targets.every((x) => defaultTargets.includes(x));
  // A pasted snippet is a new server and may stay in Skillshare only. Importing from an Agent
  // takes over the entry that Agent has, so it still needs somewhere to write.
  const needsTarget = tab === 'target' && targets.length === 0;
  return { exists, importable, selected, chosen, piChosen, piProblem, incompatible, preview, showList, inherited, needsTarget };
}

/** Pi settings of the one pasted server: the field starts from its own, and only an edit changes it. */
function pastedPiSettings(single: MCPCandidate | undefined, edit: string | undefined, t: ReturnType<typeof useT>) {
  const optionsText = edit ?? (single?.server.piOptions ? JSON.stringify(single.server.piOptions, null, 2) : '');
  const options = single ? parsePiOptions(optionsText, hasToolPolicy(single.server.tools)) : {};
  const optionsError = piOptionsError(options, t);
  let server: MCPServer | undefined;
  if (single && !optionsError && edit !== undefined) {
    server = { ...single.server };
    if (options.value && Object.keys(options.value).length > 0) server.piOptions = options.value; else delete server.piOptions;
  }
  return { optionsText, options, optionsError, server };
}

type ImportQuery = UseQueryResult<{ candidates: MCPCandidate[] }, Error>;

function ImportSourceEditor({ content, pasted, tomlFrom, query, saving, onContent, onTomlFrom }: { content: string; pasted: string; tomlFrom: string; query: ImportQuery; saving: boolean; onContent: (text: string) => void; onTomlFrom: (value: string) => void }) {
  const t = useT();
  const { toast } = useToast();
  const tomlInput = TOML.test(content);
  const yamlInput = GOOSE_YAML.test(content);
  const formatted = tomlInput || !content.trim() ? null : formatJSON(content);
  return (
    <>
      <div className="ss-fld">
        <span className="flex items-center justify-between">
          <span className="text-[13px] font-semibold">{t('mcp.snippet')}</span>
          <span className="flex items-center gap-4">
            {formatted !== null && formatted !== content && (
              <button type="button" className="flex items-center gap-1.5 text-[13px] text-ink-2 hover:text-ink" onClick={() => { onContent(formatted); }} disabled={saving}>
                <Braces size={14} />
                {t('mcp.format')}
              </button>
            )}
            {/* The CLI takes `--file`; in a browser the file picker fills the editor and the flow is the same from there. */}
            <label className="flex cursor-pointer items-center gap-1.5 text-[13px] text-ink-2 hover:text-ink">
              <FileUp size={14} />
              {t('mcp.loadFile')}
              <input
                type="file"
                className="sr-only"
                accept=".json,.jsonc,.toml,.yaml,.yml"
                disabled={saving}
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  e.target.value = ''; // so picking the same file again still fires
                  if (file) void file.text().then(onContent).catch((error: Error) => toast(describeError(t, error.message), 'error'));
                }}
              />
            </label>
          </span>
        </span>
        <CodeEditor
          value={content}
          onChange={onContent}
          lang={tomlInput ? '' : yamlInput ? 'yaml' : 'json'}
          placeholder={SNIPPET_PLACEHOLDER}
          ariaLabel={t('mcp.snippet')}
          disabled={saving}
        />
        <span className="hp">{t('mcp.pasteHint')}</span>
      </div>
      {pasted && <SnippetStatus pasted={pasted} tomlFrom={tomlFrom} query={query} onTomlFrom={onTomlFrom} />}
    </>
  );
}

function SnippetStatus({ pasted, tomlFrom, query, onTomlFrom }: { pasted: string; tomlFrom: string; query: ImportQuery; onTomlFrom: (value: string) => void }) {
  const t = useT();
  const toml = TOML.test(pasted);
  const candidates = query.data?.candidates ?? [];
  return (
    <div className="flex items-center justify-between gap-4 text-[13px]">
      {query.error ? (
        <span className="ss-st bad wrap min-w-0">{describeError(t, query.error.message)}</span>
      ) : query.data ? (
        <span className="ss-st ok">{t(candidates.length === 1 ? 'mcp.detected.one' : 'mcp.detected.other', { format: toml ? 'TOML' : GOOSE_YAML.test(pasted) ? 'YAML' : 'JSON', count: candidates.length })}</span>
      ) : (
        <span className="text-ink-3">{t('mcp.reading')}</span>
      )}
      {toml && (
        <span className="flex items-center gap-2.5">
          <span className="text-ink-2">{t('mcp.tomlFrom')}</span>
          <SegmentedControl value={tomlFrom} onChange={onTomlFrom} options={['codex', 'grok'].map((v) => ({ value: v, label: v }))} />
        </span>
      )}
    </div>
  );
}

function importButtonLabel(adding: boolean, count: number, t: ReturnType<typeof useT>) {
  if (count === 0) return t(adding ? 'mcp.addServer' : 'mcp.importAction');
  const key = adding ? 'mcp.addCount' : 'mcp.importCount';
  return t(`${key}.${count === 1 ? 'one' : 'other'}`, { count });
}

function ImportCandidateList({ tab, candidates, selection, query, saving, onPick }: { tab: Source; candidates: MCPCandidate[]; selection: ReturnType<typeof importSelection>; query: ImportQuery; saving: boolean; onPick: (names: string[]) => void }) {
  const t = useT();
  const { exists, importable, chosen, selected } = selection;
  return (
    <div className="ss-list !shadow-none">
      <div className="ss-lh">
        <Checkbox
          hideLabel
          label={t('mcp.selectAll')}
          checked={importable.length > 0 && chosen.length === importable.length}
          indeterminate={chosen.length > 0 && chosen.length < importable.length}
          onChange={(on) => onPick(on ? importable.map((c) => c.name) : [])}
          disabled={saving || importable.length === 0}
        />
        <span className="flex-1">
          {query.isPending && query.fetchStatus !== 'idle' ? t('mcp.reading') : t(candidates.length === 1 ? 'mcp.found.one' : 'mcp.found.other', { count: candidates.length })}
        </span>
        <span>{t('mcp.transport')}</span>
      </div>
      {tab === 'target' && query.error && <div className="ss-r text-[13px] text-bad">{describeError(t, query.error.message)}</div>}
      {candidates.map((c) => {
        const blocked = c.problems.length > 0 || exists(c);
        const on = selected.includes(c.name) && !blocked;
        return (
          <div key={c.name} className={`ss-r !items-start !py-2.5 ${on ? 'sel' : ''} ${blocked ? 'opacity-55' : ''}`}>
            <Checkbox
              className="mt-0.5"
              hideLabel
              label={c.name}
              checked={on}
              onChange={(v) => onPick(v ? [...selected, c.name] : selected.filter((n) => n !== c.name))}
              disabled={saving || blocked}
            />
            <span className="flex min-w-0 flex-1 flex-col gap-px">
              <span className="font-mono text-[13px] font-semibold">{c.name}</span>
              <span className="truncate font-mono text-xs text-ink-3" title={describeEndpoint(c.server)}>{describeEndpoint(c.server)}</span>
              {[...new Set(c.problems)].map((p) => <span key={p} className="text-xs text-bad">{p}</span>)}
              {[...new Set(c.warnings)].map((w) => <span key={w} className="text-xs text-warn">{w}</span>)}
            </span>
            {exists(c) ? (
              <span className="ss-st off self-center">{t('mcp.alreadyAdded')}</span>
            ) : (
              <span className="ss-tag self-center">{c.server.url ? 'http' : 'stdio'}</span>
            )}
          </div>
        );
      })}
    </div>
  );
}

function ImportTargetNote({ targets, needsTarget }: { targets: string[]; needsTarget: boolean }) {
  const t = useT();
  return (
    <span className="flex-1 text-[13px]">
      {targets.length === 0
        ? (needsTarget ? <span className="ss-st warn">{t('mcp.pickTarget')}</span> : <span className="text-ink-2">{t('mcp.noTargetsNote')}</span>)
        : <span className="text-ink-2">{t(targets.length === 1 ? 'mcp.writes.one' : 'mcp.writes.other', { count: targets.length })}</span>}
    </span>
  );
}

function ImportFooter({ adding, count, targets, needsTarget, incompatible, saving, onClose, onRun }: { adding: boolean; count: number; targets: string[]; needsTarget: boolean; incompatible: boolean; saving: boolean; onClose: () => void; onRun: () => Promise<void> }) {
  const t = useT();
  return (
    <div className="df">
      {/* Name what is missing: a greyed-out button next to "writes 0 config files" reads as a bug. */}
      <ImportTargetNote targets={targets} needsTarget={needsTarget} />
      <Button variant="ghost" onClick={onClose} disabled={saving}>{t('common.cancel')}</Button>
      <Button variant="primary" loading={saving} disabled={count === 0 || needsTarget || incompatible} onClick={onRun}>
        {adding ? <Plus size={15} /> : <Download size={15} />}
        {/* "Add 0 servers" reads as a bug before anything is pasted; the plain verb doesn't. */}
        {importButtonLabel(adding, count, t)}
      </Button>
    </div>
  );
}

function useImportQuery(sourceID: string, selectedSource: MCPImportSource | undefined, project: string | undefined, tab: Source, pasted: string, tomlFrom: string) {
  const from = selectedSource?.target ?? '';
  const toml = TOML.test(pasted);
  const fromTarget = useQuery({ queryKey: ['mcp-import', sourceID, project], queryFn: () => mcpApi.import({ from, ...(selectedSource?.piExtension && { piExtension: selectedSource.piExtension }), ...(project && { root: project }) }), enabled: tab === 'target' && from !== '', gcTime: 0, retry: false });
  const fromPaste = useQuery({
    queryKey: ['mcp-import-paste', pasted, toml && tomlFrom],
    queryFn: () => mcpApi.import({ content: pasted, ...(toml && { from: tomlFrom }) }),
    enabled: tab === 'paste' && pasted !== '',
    gcTime: 0,
    retry: false,
  });
  return tab === 'target' ? fromTarget : fromPaste;
}

function ImportPreview({ selection, targets, project, onPreviewName }: { selection: ReturnType<typeof importSelection>; targets: string[]; project?: string; onPreviewName: (name: string) => void }) {
  const t = useT();
  const { preview, chosen } = selection;
  if (!preview || targets.length === 0) return null;
  return (
    <div className="ss-fld">
      {chosen.length > 1 && <Select ariaLabel={t('mcp.name')} value={preview.name} onChange={onPreviewName} options={chosen.map((c) => ({ value: c.name, label: c.name }))} />}
      <MCPConfigView mutation={{ project, name: preview.name, server: { ...preview.server, targets } }} />
    </div>
  );

}

export default function MCPImportDialog(props: Props) {
  const { source, servers, defaultTargets, paths, conflict, project, availableTargets: offered, onMode, onClose, onImported } = props;
  const t = useT();
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const tab = source;
  const order = useContext(MCPTargetOrder);
  const availableTargets = offered ?? order.filter((x) => paths[x]);
  const { sources, initialSource } = importSourceChoices(props, order);
  const [sourceID, setSourceID] = useState(initialSource ? sourceKey(initialSource) : '');
  const selectedSource = sources.find((s) => sourceKey(s) === sourceID);
  const [content, setContent] = useState('');
  const [pasted, setPasted] = useState('');
  const [tomlFrom, setTomlFrom] = useState('codex');
  const [picked, setPicked] = useState<string[] | null>(null);
  const [targets, setTargets] = useState<string[]>(defaultTargets);
  const [previewName, setPreviewName] = useState('');
  const [piEdit, setPiEdit] = useState<string>();
  const [saving, setSaving] = useState(false);
  const visibleTargets = new Set([...availableTargets, ...targets]);

  // Codex and Grok share one TOML shape; JSON formats are detected by the server
  useEffect(() => {
    const id = window.setTimeout(() => setPasted(content.trim()), 400);
    return () => window.clearTimeout(id);
  }, [content]);

  const query = useImportQuery(sourceID, selectedSource, project, tab, pasted, tomlFrom);
  const candidates = query.data?.candidates ?? [];

  const base = { servers, conflict, picked, targets, defaultTargets, previewName, tab };
  const raw = importSelection({ ...base, candidates });
  // One pasted server gets the form's Pi settings; several keep their own.
  const single = tab === 'paste' && candidates.length === 1 && raw.chosen.length === 1 && raw.piChosen ? raw.chosen[0] : undefined;
  const pi = pastedPiSettings(single, piEdit, t);
  const selection = single && pi.server ? importSelection({ ...base, candidates: [{ ...single, server: pi.server }] }) : raw;
  const { chosen, piProblem, incompatible, showList, inherited, needsTarget } = selection;
  const reset = () => { setPicked(null); setPiEdit(undefined); };

  const run = async () => {
    setSaving(true);
    const failed: string[] = [];
    for (const c of chosen) {
      // A takeover keeps the targets the existing server already has
      const own = servers[c.name]?.targets;
      const write = own ?? (inherited ? undefined : order.filter((x) => targets.includes(x)));
      const mutation: MCPMutation = { ...(project && { project }), name: c.name, server: { ...c.server, ...(write && { targets: write }) }, replace: c.name in servers };
      // Adopting records the tool's identical entry as managed, so the next sync has no conflict there
      if (c.from && (own ?? targets).includes(c.from)) mutation.resolutions = [{ target: c.from, name: c.name, action: 'adopt' }];
      try {
        await mcpApi.save(mutation);
      } catch (e) {
        failed.push(`${c.name}: ${(e as Error).message}`);
      }
    }
    const done = chosen.length - failed.length;
    if (done > 0) toast(t(done === 1 ? 'mcp.toast.imported.one' : 'mcp.toast.imported.other', { count: done }), 'success');
    if (failed.length) toast(failed.join('\n'), 'error');
    onImported();
  };

  const adding = source === 'paste';
  const title = t(adding ? 'mcp.addServer' : 'mcp.importTitle');
  const count = chosen.length;

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={saving} ariaLabel={title} className="!max-w-[880px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t(adding ? 'mcp.pasteSubtitle' : 'mcp.importSubtitle')}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={saving}><X size={16} /></button>
      </div>
      <div className="db">
        {onMode && (
          <SegmentedControl<'form' | 'paste'>
            className="self-start"
            value="paste"
            onChange={onMode}
            options={[{ value: 'form', label: t('mcp.manualTab') }, { value: 'paste', label: t('mcp.pasteTab') }]}
          />
        )}

        {tab === 'target' ? (
          <div className="ss-fld">
            <span className="text-[13px] font-semibold">{t('mcp.target')}</span>
            <Select
              ariaLabel={t('mcp.target')}
              value={sourceID}
              onChange={(v) => { setSourceID(v); reset(); setPreviewName(''); }}
              options={sources.map((s) => ({ value: sourceKey(s), label: `${targetLabel(s.target)}  ${shortenHome(s.path)}`, icon: <AgentIcon target={s.target} size={16} /> }))}
              disabled={saving}
            />
            <span className="hp">{t('mcp.fromTargetHint')}</span>
          </div>
        ) : (
          <ImportSourceEditor content={content} pasted={pasted} tomlFrom={tomlFrom} query={query} saving={saving} onContent={(text) => { setContent(text); reset(); }} onTomlFrom={(value) => { setTomlFrom(value); reset(); }} />
        )}

        {showList && (
          <ImportCandidateList tab={tab} candidates={candidates} selection={selection} query={query} saving={saving} onPick={setPicked} />
        )}

        <div className="ss-fld">
          <span className="text-[13px] font-semibold">{t('mcp.targets')}</span>
          <div className="flex flex-wrap gap-x-5 gap-y-3">
            {order.filter((target) => visibleTargets.has(target)).map((target) => {
              const on = targets.includes(target);
              return (
                <button
                  key={target}
                  type="button"
                  role="checkbox"
                  aria-checked={on}
                  className={`ss-tgl ${on ? 'on' : ''}`}
                  onClick={() => setTargets((prev) => (prev.includes(target) ? prev.filter((x) => x !== target) : [...prev, target]))}
                  disabled={saving}
                >
                  <span className="ic"><AgentIcon target={target} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                  {targetLabel(target)}
                </button>
              );
            })}
          </div>
        </div>

        {single && <PiSettingsFields optionsText={pi.optionsText} options={pi.options} optionsError={pi.optionsError} onOptions={setPiEdit} disabled={saving} project={Boolean(project) || isProjectMode} toolsSet={hasToolPolicy(single.server.tools)} />}
        {piProblem && <div className="ss-note bad"><span><span className="font-mono">{piProblem.name}</span>: {piOptionsError(piProblem.options, t)}</span></div>}
        <ImportPreview selection={selection} targets={targets} project={project} onPreviewName={setPreviewName} />
        {tab === 'target' && candidates.length > 0 && (
          <div className="ss-note inf">
            <Info size={16} />
            <span className="flex-1">{t('mcp.secretsNote')}</span>
          </div>
        )}
      </div>
      <ImportFooter adding={adding} count={count} targets={targets} needsTarget={needsTarget} incompatible={incompatible || Boolean(pi.optionsError)} saving={saving} onClose={onClose} onRun={run} />
    </DialogShell>
  );
}
