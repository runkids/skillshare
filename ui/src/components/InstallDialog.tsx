import { useId, useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ChevronDown,
  ChevronRight,
  CircleX,
  Download,
  ExternalLink,
  GitBranch,
  Github,
  Info,
  KeyRound,
  Library,
  Link as LinkIcon,
  Puzzle,
  Search,
  Settings2,
  ShieldAlert,
  ShieldCheck,
  Star,
  X,
} from 'lucide-react';
import { api, ApiError, type DiscoverResult, type DiscoveredSkill, type HubSavedEntry, type SearchResult } from '../api/client';
import { clearAuditCache } from '../lib/auditCache';
import { isAuditBlock, parseFindings, thresholdOf, type Finding } from '../lib/auditMessage';
import { parseSkillMarkdown } from '../lib/frontmatter';
import CodeView from './CodeView';
import MarkdownView, { ViewToggle } from './MarkdownView';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { formatSkillDisplayName } from '../lib/resourceNames';
import { useI18n, useT } from '../i18n';
import Button from './Button';
import { Checkbox } from './Checkbox';
import DialogShell from './DialogShell';
import EmptyState from './EmptyState';
import FindingList from './FindingList';
import { Select } from './Select';
import Spinner from './Spinner';
import { useToast } from './Toast';

type Kind = 'skill' | 'agent';
type Tab = 'search' | 'url';
type View = 'preview' | 'kind' | 'blocked' | 'warnings';
type InstallOpts = Parameters<typeof api.install>[0];
type BatchOpts = Parameters<typeof api.installBatch>[0];
type Blocked = { source: string; names: string[]; threshold: string; findings: Finding[]; retry: InstallOpts | BatchOpts };
type KindChoice = { source: string; skills: number; agents: number; onContinue: (kind: Kind) => void };

const GITHUB = 'github';
const COMMUNITY_HUB: HubSavedEntry = {
  label: 'Skillshare Hub',
  url: 'https://raw.githubusercontent.com/runkids/skillshare-hub/main/skillshare-hub.json',
  builtIn: true,
};
const STEP = 10;
const FOUND_PREVIEW = 5;
/** Backend error code for a mixed-track-repo ambiguity (see internal/server/handler_install.go). */
const TRACK_KIND_AMBIGUOUS = 'install.track_kind_ambiguous';
const WIDTH: Record<Tab | View, string> = {
  search: '!max-w-[700px]',
  url: '!max-w-[700px]',
  preview: '!max-w-[640px]',
  blocked: '!max-w-[600px]',
  warnings: '!max-w-[600px]',
  kind: '!max-w-[460px]',
};

function isGitSource(s: string) {
  const v = s.trim();
  if (!v || /^[/~.]/.test(v) || /^[a-zA-Z]:\\/.test(v)) return false;
  return v.includes('/') || v.startsWith('git@') || v.includes('://');
}

const sameURL = (a: string, b: string) => a.trim().replace(/\/+$/, '') === b.trim().replace(/\/+$/, '');

function mergeHubs(hubs: HubSavedEntry[]): HubSavedEntry[] {
  return [COMMUNITY_HUB, ...hubs.filter((h) => !sameURL(h.url, COMMUNITY_HUB.url)).map((h) => ({ label: h.label, url: h.url }))];
}

function readCount(params: Record<string, unknown> | undefined, key: string): number {
  const raw = params?.[key];
  const n = typeof raw === 'number' ? raw : parseInt(String(raw ?? ''), 10);
  return Number.isFinite(n) && n > 0 ? n : 0;
}

export default function InstallDialog({ kind, initialTab, initialSource, onClose }: { kind: Kind; initialTab: Tab; initialSource?: string; onClose: () => void }) {
  const t = useT();
  const { locale } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const isAgent = kind === 'agent';
  const ids = useId();
  // Search finds SKILL.md files only, so the agents page installs from a URL.
  const [tab, setTab] = useState<Tab>(isAgent ? 'url' : initialTab);
  const [view, setView] = useState<View | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const [query, setQuery] = useState('');
  const [pickedIn, setPickedIn] = useState<string | null>(null);
  const [results, setResults] = useState<SearchResult[] | null>(null);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [tag, setTag] = useState<string | null>(null);
  const [shown, setShown] = useState(STEP);
  const [previewing, setPreviewing] = useState<SearchResult | null>(null);
  const [rawPreview, setRawPreview] = useState(false);

  const [source, setSource] = useState(initialSource ?? '');
  const [track, setTrack] = useState(false);
  const [advanced, setAdvanced] = useState(false);
  const [into, setInto] = useState('');
  const [branch, setBranch] = useState('');
  const [name, setName] = useState('');
  const [force, setForce] = useState(false);
  const [skipAudit, setSkipAudit] = useState(false);
  const [found, setFound] = useState<{ source: string; items: DiscoveredSkill[] } | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState('');
  const [expanded, setExpanded] = useState(false);

  const [kindChoice, setKindChoice] = useState<KindChoice | null>(null);
  const [kindPick, setKindPick] = useState<Kind>(kind);
  const [blocked, setBlocked] = useState<Blocked | null>(null);
  const [warnings, setWarnings] = useState<Finding[]>([]);

  const { data: skillsData } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const installedKeys = useMemo(() => new Set((skillsData?.resources ?? []).map((r) => `${r.kind}:${r.name}`)), [skillsData]);
  const isInstalled = (k: Kind, n: string) => installedKeys.has(`${k}:${n}`);

  const { data: hubConfig } = useQuery({
    queryKey: queryKeys.hubConfig,
    queryFn: () => api.getHubConfig(),
    enabled: !isAgent,
  });
  const hubs = useMemo(() => mergeHubs(hubConfig?.hubs ?? []), [hubConfig]);
  const defaultIn = hubs.find((h) => hubConfig?.default && h.label.toLowerCase() === hubConfig.default.toLowerCase())?.url ?? GITHUB;
  const searchIn = pickedIn && (pickedIn === GITHUB || hubs.some((h) => h.url === pickedIn)) ? pickedIn : defaultIn;
  const hubName = hubs.find((h) => h.url === searchIn)?.label;

  const { data: preview, error: previewError, isFetching: previewLoading, refetch: refetchPreview } = useQuery({
    queryKey: queryKeys.preview(previewing?.source ?? ''),
    queryFn: () => api.preview(previewing!.source),
    enabled: previewing !== null,
    retry: false,
    staleTime: staleTimes.config,
  });

  // The i18n layer has no plural rules, so pick the key by count.
  const countLabel = (k: Kind, n: number) => t(`resources.count.${k}${n === 1 ? '' : 's'}`, { count: n });
  const findingsLabel = (n: number) => t(n === 1 ? 'install.finding' : 'install.findings', { count: n });
  const compact = new Intl.NumberFormat(locale, { notation: 'compact', maximumFractionDigits: 1 });
  const locked = busy !== null && busy !== 'search';
  const src = source.trim();
  const canTrack = !src || isGitSource(src);
  const tracking = track && canTrack;

  const withBusy = async (key: string, fn: () => Promise<void>) => {
    setBusy(key);
    try {
      await fn();
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setBusy(null);
    }
  };

  const refresh = () => {
    clearAuditCache(queryClient);
    queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    queryClient.invalidateQueries({ queryKey: queryKeys.overview });
  };

  const finish = (warningLines: string[]) => {
    refresh();
    const list = parseFindings(warningLines.flatMap((w) => w.split('\n')), true);
    if (list.length > 0) {
      setWarnings(list);
      setView('warnings');
    } else if (tab === 'url') {
      onClose();
    } else {
      setView(null);
    }
  };

  const askKind = (choice: KindChoice) => {
    setKindChoice(choice);
    setKindPick(kind);
    setView('kind');
  };

  const runSingle = async (opts: InstallOpts) => {
    try {
      const res = await api.install(opts);
      toast(t('install.toast.installed', { label: res.skillName ?? res.repoName ?? opts.source }), 'success');
      finish(res.warnings ?? []);
    } catch (e) {
      if (e instanceof ApiError && e.code === TRACK_KIND_AMBIGUOUS) {
        askKind({
          source: opts.source,
          skills: readCount(e.params, 'skills'),
          agents: readCount(e.params, 'agents'),
          onContinue: (k) => void withBusy('primary', () => runSingle({ ...opts, kind: k })),
        });
        return;
      }
      const msg = (e as Error).message;
      if (!isAuditBlock(msg)) throw e;
      setBlocked({
        source: opts.source,
        names: [opts.name || opts.source.replace(/\/+$/, '').split('/').pop() || opts.source],
        threshold: thresholdOf(msg),
        findings: parseFindings(msg.split('\n'), false),
        retry: opts,
      });
      setView('blocked');
    }
  };

  const runBatch = async (opts: BatchOpts) => {
    const res = await api.installBatch(opts);
    const blockedItems: DiscoveredSkill[] = [];
    const findings: Finding[] = [];
    const errors: string[] = [];
    const existing: string[] = [];
    const warningLines: string[] = [];
    let threshold = '';
    let ok = 0;
    for (const item of res.results) {
      if (item.error && isAuditBlock(item.error)) {
        threshold = threshold || thresholdOf(item.error);
        findings.push(...parseFindings(item.error.split('\n'), false).map((f) => ({ ...f, name: item.name })));
        const skill = opts.skills.find((s) => s.name === item.name);
        if (skill) blockedItems.push(skill);
      } else if (item.error?.startsWith('already exists')) {
        // The server's hint is a CLI command; the dialog's own Force overwrite option does the same
        existing.push(formatSkillDisplayName(item.name));
      } else if (item.error) {
        errors.push(`${formatSkillDisplayName(item.name)}: ${item.error}`);
      } else {
        ok++;
      }
      for (const w of item.warnings ?? []) warningLines.push(`${item.name}: ${w}`);
    }
    if (existing.length > 0) toast(t('install.toast.exists', { count: existing.length, names: existing.join(', ') }), 'warning');
    if (errors.length > 0) toast(t('common.nFailed', { count: errors.length, details: errors.join('; ') }), 'error');
    if (ok > 0) toast(res.summary, blockedItems.length > 0 ? 'warning' : 'success');
    if (blockedItems.length > 0) {
      if (ok > 0) refresh();
      setBlocked({ source: opts.source, names: blockedItems.map((s) => s.name), threshold, findings, retry: { ...opts, skills: blockedItems } });
      setView('blocked');
    } else if (ok > 0) {
      finish(warningLines);
    }
  };

  const forceInstall = (b: Blocked) =>
    withBusy('force', () => ('skills' in b.retry ? runBatch({ ...b.retry, force: true }) : runSingle({ ...b.retry, force: true })));

  const applyDiscovery = async (from: string, d: DiscoverResult, opts: Omit<InstallOpts, 'source'>) => {
    const skills = d.skills.map((s) => ({ ...s, kind: 'skill' as const }));
    const agents = (d.agents ?? []).map((a) => ({ name: a.name, path: a.path, kind: 'agent' as const }));
    const show = (items: DiscoveredSkill[]) => {
      setSource(from);
      setFound({ source: from, items });
      setSelected(new Set(items.filter((i) => !isInstalled(i.kind ?? 'skill', i.name)).map((i) => i.path)));
      setFilter('');
      setExpanded(false);
      setTab('url');
      setView(null);
    };
    if (skills.length > 0 && agents.length > 0) {
      askKind({ source: from, skills: skills.length, agents: agents.length, onContinue: (k) => show(k === 'agent' ? agents : skills) });
    } else if (skills.length + agents.length > 0) {
      show(skills.length > 0 ? skills : agents);
    } else {
      await runSingle({ source: from, ...opts });
    }
  };

  const formOpts = () => ({
    name: name.trim() || undefined,
    into: into.trim() || undefined,
    branch: (canTrack && branch.trim()) || undefined,
    force,
    skipAudit,
  });

  const primary = () => {
    if (!src || locked) return;
    const opts = formOpts();
    if (tracking) {
      void withBusy('primary', () => runSingle({ source: src, track: true, ...opts }));
    } else if (found) {
      const items = found.items.filter((i) => selected.has(i.path));
      if (items.length === 0) return;
      void withBusy('primary', () =>
        runBatch({
          source: found.source,
          skills: items,
          ...opts,
          name: items.length === 1 ? opts.name : undefined,
          kind: items[0].kind === 'agent' ? 'agent' : undefined,
        }),
      );
    } else {
      void withBusy('primary', async () => applyDiscovery(src, await api.discover(src, opts.branch), opts));
    }
  };

  const installResult = (r: SearchResult) =>
    withBusy(`row:${r.source}`, async () => {
      const d = await api.discover(r.source);
      const matched = r.skill ? d.skills.filter((s) => s.name === r.skill) : [];
      if (matched.length > 0) {
        await runBatch({ source: r.source, skills: matched });
      } else if (d.skills.length === 1 && !d.agents?.length) {
        await runBatch({ source: r.source, skills: d.skills });
      } else {
        setTrack(false);
        await applyDiscovery(r.source, d, {});
      }
    });

  const search = (q: string, where = searchIn) =>
    withBusy('search', async () => {
      setSearchError(null);
      setTag(null);
      setShown(STEP);
      try {
        const res = where === GITHUB ? await api.search(q) : await api.searchHub(q, where);
        setResults(res.results);
      } catch (e) {
        setResults(null);
        setSearchError((e as Error).message);
      }
    });

  const saveHubs = async (list: HubSavedEntry[], defaultLabel: string) => {
    const next = { hubs: list.filter((h) => !h.builtIn).map(({ label, url }) => ({ label, url })), default: defaultLabel };
    await api.putHubConfig(next);
    queryClient.setQueryData(queryKeys.hubConfig, next);
  };
  const defaultLabelFor = (list: HubSavedEntry[], url: string) => {
    const hub = list.find((h) => h.url === url);
    return hub && !hub.builtIn ? hub.label : '';
  };

  const pickIn = (where: string) => {
    setPickedIn(where);
    setResults(null);
    setSearchError(null);
    setTag(null);
    // The pick still works for this session when saving the default fails.
    saveHubs(hubs, defaultLabelFor(hubs, where)).catch(() => undefined);
  };

  const tabs = !isAgent && (
    <div className="ss-tabs" role="tablist">
      {(['search', 'url'] as const).map((k) => (
        <button key={k} type="button" role="tab" aria-selected={tab === k} className={tab === k ? 'on' : ''} onClick={() => setTab(k)}>
          {t(k === 'search' ? 'install.tab.search' : 'install.tab.url')}
        </button>
      ))}
    </div>
  );

  let title = t(isAgent ? 'install.title.agents' : 'install.title.skills');
  let sub: ReactNode = null;
  let body: ReactNode;
  let foot: ReactNode;

  if (view === 'preview' && previewing) {
    const p = previewing;
    const md = preview?.content ? parseSkillMarkdown(preview.content).body.trim() : '';
    const tags = preview?.tags?.length ? preview.tags : p.tags;
    const stars = preview?.stars || p.stars;
    const failed = previewError !== null && !p.description;
    title = t('install.preview.title');
    sub = t(md || previewLoading ? 'install.preview.subtitle' : 'install.preview.subtitleShort');
    body = (
      <>
        <div className="flex items-center gap-3.5">
          <span className="ss-cat skill"><Puzzle size={17} /></span>
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <span className="flex items-center gap-2.5">
              <span className="truncate font-mono text-[15px] font-semibold">{preview?.name || p.name}</span>
              {stars > 0 && <span className="flex items-center gap-1 text-xs text-ink-3"><Star size={13} />{compact.format(stars)}</span>}
            </span>
            <span className="flex min-w-0 items-center gap-1.5 text-[13px] text-ink-2">
              {searchIn === GITHUB ? <Github size={13} className="shrink-0" /> : <Library size={13} className="shrink-0" />}
              <span className="truncate font-mono">{searchIn !== GITHUB && hubName ? `${hubName} · ${p.source}` : p.source}</span>
              {searchIn === GITHUB && p.owner && p.repo && (
                <a href={`https://github.com/${p.owner}/${p.repo}`} target="_blank" rel="noreferrer" className="shrink-0 text-ink-3" aria-label={`${p.owner}/${p.repo}`}>
                  <ExternalLink size={12} />
                </a>
              )}
            </span>
          </div>
          {md && <ViewToggle raw={rawPreview} onChange={setRawPreview} />}
        </div>
        {previewLoading ? (
          <div className="ss-box !shadow-none flex justify-center py-10"><Spinner /></div>
        ) : md ? (
          rawPreview ? (
            <CodeView content={preview?.content ?? ''} lang="md" className="max-h-[300px]" />
          ) : (
            <div className="ss-box !shadow-none max-h-[300px] overflow-y-auto">
              <MarkdownView>{md}</MarkdownView>
            </div>
          )
        ) : (
          <div className={`ss-note ${failed ? 'bad' : 'inf'}`}>
            {failed ? <CircleX size={16} /> : <Info size={16} />}
            <div className="flex-1">{failed ? previewError.message : t('install.preview.fallback')}</div>
            {previewError && <Button variant="secondary" size="sm" onClick={() => refetchPreview()}>{t('install.preview.retry')}</Button>}
          </div>
        )}
        <dl className="ss-kv">
          {!md && p.description && (<><dt>{t('install.preview.description')}</dt><dd>{p.description}</dd></>)}
          {preview?.license && (<><dt>{t('install.preview.license')}</dt><dd>{preview.license}</dd></>)}
          {tags && tags.length > 0 && (
            <><dt>{t('install.preview.tags')}</dt><dd className="flex flex-wrap gap-1">{tags.map((x) => <span key={x} className="ss-tag">{x}</span>)}</dd></>
          )}
          <dt>{t('install.preview.audit')}</dt>
          <dd className="font-normal text-ink-2">{t('install.preview.auditHint')}</dd>
        </dl>
      </>
    );
    foot = (
      <>
        <Button variant="ghost" onClick={() => setView(null)} disabled={locked}>{t('install.preview.back')}</Button>
        <span className="flex-1" />
        {isInstalled(kind, p.name) ? (
          <span className="ss-st ok text-ok">Installed</span>
        ) : (
          <Button variant="primary" loading={busy === `row:${p.source}`} disabled={locked || previewLoading || failed} onClick={() => installResult(p)}>
            {busy !== `row:${p.source}` && <Download size={15} />}
            {t('install.row.install')}
          </Button>
        )}
      </>
    );
  } else if (view === 'kind' && kindChoice) {
    title = t('install.kind.title');
    sub = t('install.kind.message', { source: kindChoice.source });
    body = (
      <>
        <div role="radiogroup" aria-label={title} className="flex flex-col gap-2">
          {(['skill', 'agent'] as const).map((k) => (
            <button key={k} type="button" role="radio" aria-checked={kindPick === k} className={`ss-pick text-left ${kindPick === k ? 'on' : ''}`} onClick={() => setKindPick(k)}>
              <span className={`ss-chk rad ${kindPick === k ? 'on' : ''}`} />
              <span className="flex flex-1 flex-col gap-0.5">
                <span className="font-semibold">{t(k === 'agent' ? 'install.kind.agents' : 'install.kind.skills')}</span>
                <span className="text-[13px] text-ink-2">{countLabel(k, k === 'agent' ? kindChoice.agents : kindChoice.skills)}</span>
              </span>
            </button>
          ))}
        </div>
        <p className="text-[13px] text-ink-2">{t('install.kind.hint')}</p>
      </>
    );
    foot = (
      <>
        <Button variant="ghost" onClick={() => setView(null)} disabled={locked}>{t('common.cancel')}</Button>
        <Button variant="primary" loading={busy === 'primary'} onClick={() => kindChoice.onContinue(kindPick)}>{t('install.kind.continue')}</Button>
      </>
    );
  } else if (view === 'blocked' && blocked) {
    title = t('install.blocked.title');
    sub = <span className="font-mono">{blocked.source}</span>;
    body = (
      <>
        <div className="ss-note bad">
          <ShieldAlert size={16} />
          <div className="flex-1">
            <b>{blocked.names.length === 1 ? t('install.blocked.one', { name: blocked.names[0] }) : t('install.blocked.many', { names: blocked.names.join(', ') })}</b>
            {blocked.threshold && ` ${t('install.blocked.threshold', { threshold: blocked.threshold })}`}
          </div>
        </div>
        <FindingList findings={blocked.findings} showName={blocked.names.length > 1} header={findingsLabel(blocked.findings.length)} />
        <p className="text-[13px] text-ink-2">{t('install.blocked.forceHint')}</p>
      </>
    );
    foot = (
      <>
        <Button variant="danger" loading={busy === 'force'} onClick={() => forceInstall(blocked)}>{t('install.blocked.force')}</Button>
        <span className="flex-1" />
        <Button variant="primary" onClick={() => setView(null)} disabled={locked}>{t('common.cancel')}</Button>
      </>
    );
  } else if (view === 'warnings') {
    title = t('install.warnings.title');
    body = (
      <>
        <p className="text-[13px] text-ink-2">{t('install.warnings.message')}</p>
        <FindingList findings={warnings} showName header={findingsLabel(warnings.length)} />
      </>
    );
    foot = <Button variant="primary" onClick={() => (tab === 'url' ? onClose() : setView(null))}>{t('install.done')}</Button>;
  } else if (tab === 'search') {
    const visible = tag ? (results ?? []).filter((r) => r.tags?.includes(tag)) : (results ?? []);
    const hasTags = visible.some((r) => (r.tags?.length ?? 0) > 0);
    let content: ReactNode;
    if (searchError) {
      content = searchError.includes('requires authentication') ? (
        <div className="ss-note inf"><KeyRound size={16} /><div className="flex-1">{t('install.search.tokenNeeded')}</div></div>
      ) : (
        <div className="ss-note bad">
          <CircleX size={16} />
          <div className="flex-1">
            {searchIn !== GITHUB && <b>{t('install.search.hubFailed', { hub: hubName ?? searchIn })} </b>}
            {searchError}
          </div>
        </div>
      );
    } else if (results === null) {
      content = (
        <EmptyState
          icon={Search}
          title={t('install.search.startTitle')}
          description={t(searchIn === GITHUB ? 'install.search.startGithub' : 'install.search.startHub')}
          action={searchIn === GITHUB ? (
            <>
              <Button variant="secondary" disabled={busy !== null} onClick={() => search('')}>{t('install.search.browsePopular')}</Button>
              <Button variant="ghost" disabled={busy !== null} onClick={() => { pickIn(COMMUNITY_HUB.url); void search('', COMMUNITY_HUB.url); }}>{t('install.search.browseAll')}</Button>
            </>
          ) : (
            <Button variant="secondary" disabled={busy !== null} onClick={() => search('')}>{t('install.search.browseAll')}</Button>
          )}
        />
      );
    } else if (visible.length === 0) {
      content = (
        <EmptyState
          icon={Search}
          title={t('install.search.noResults')}
          description={t(searchIn === GITHUB ? 'install.search.noResultsGithub' : 'install.search.noResultsHub')}
        />
      );
    } else {
      content = (
        <>
          {/* The inner div scrolls so the list's rounded frame clips the scrollbar */}
          <div className="ss-list !shadow-none !shrink min-h-0 flex flex-col">
            <div className="min-h-0 overflow-y-auto">
              {visible.slice(0, shown).map((r) => (
                <div key={`${r.source}#${r.skill ?? r.name}`} className="ss-r">
                  <span className="ss-cat sm skill"><Puzzle size={14} /></span>
                  <button type="button" className="flex min-w-0 flex-1 flex-col gap-px text-left" onClick={() => { setPreviewing(r); setView('preview'); }}>
                    <span className="flex min-w-0 items-baseline gap-2">
                      <span className="nm m truncate">{r.name}</span>
                      {r.owner && r.repo && <span className="truncate font-mono text-xs text-ink-3">{r.owner}/{r.repo}</span>}
                    </span>
                    {r.description && <span className="truncate text-[13px] text-ink-2">{r.description}</span>}
                  </button>
                  {r.tags && r.tags.length > 0 && (
                    <span className="flex shrink-0 gap-1">
                      {r.tags.slice(0, 2).map((x) => (
                        <button key={x} type="button" className={`ss-tag ${tag === x ? 'inf' : ''}`} aria-pressed={tag === x} onClick={() => { setTag(tag === x ? null : x); setShown(STEP); }}>{x}</button>
                      ))}
                    </span>
                  )}
                  {r.stars > 0 && <span className="flex w-14 shrink-0 items-center gap-1 text-xs text-ink-3"><Star size={13} />{compact.format(r.stars)}</span>}
                  <span className="flex w-[76px] shrink-0 justify-end">
                    {isInstalled(kind, r.name) ? (
                      <span className="ss-st ok text-ok">Installed</span>
                    ) : (
                      <Button variant="secondary" size="sm" loading={busy === `row:${r.source}`} disabled={locked} onClick={() => installResult(r)}>
                        {t('install.row.install')}
                      </Button>
                    )}
                  </span>
                </div>
              ))}
            </div>
          </div>
          <div className="flex items-center justify-between gap-3">
            <span className="flex items-center gap-1 text-[13px] text-ink-3">
              {t('install.search.range', { from: 1, to: Math.min(shown, visible.length), total: visible.length })}
              {tag ? (
                <button type="button" className="ss-tag inf" aria-label={t('install.search.clearTag')} onClick={() => setTag(null)}>{tag}<X size={11} /></button>
              ) : (
                hasTags && <span>· {t('install.search.tagHint')}</span>
              )}
            </span>
            {shown < visible.length && <Button variant="ghost" size="sm" onClick={() => setShown(shown + STEP)}>{t('resources.showMore')}</Button>}
          </div>
        </>
      );
    }
    body = (
      <>
        {tabs}
        <div className="flex items-center gap-2">
          <label className="ss-inp flex-1">
            <Search size={15} className="shrink-0 text-ink-3" />
            <input
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && busy === null && search(query)}
              placeholder={t('install.search.placeholder')}
              aria-label={t('install.search.placeholder')}
            />
            {busy === 'search' && <Spinner size="sm" />}
          </label>
          <Select
            className="w-[190px] shrink-0"
            prefix={t('install.search.in')}
            value={searchIn}
            onChange={pickIn}
            options={[{ value: GITHUB, label: 'GitHub' }, ...hubs.map((h) => ({ value: h.url, label: h.label }))]}
          />
          <Link to="/hubs" className="ss-ib" title={t('install.search.manageHubs')} aria-label={t('install.search.manageHubs')} onClick={onClose}>
            <Settings2 size={16} />
          </Link>
        </div>
        {content}
      </>
    );
    foot = (
      <>
        <span className="flex flex-1 items-center gap-2 text-[13px] text-ink-2"><ShieldCheck size={15} className="shrink-0" />{t('install.audited')}</span>
        <Button variant="primary" onClick={onClose} disabled={locked}>{t('install.done')}</Button>
      </>
    );
  } else {
    const agents = found?.items[0]?.kind === 'agent';
    const f = filter.trim().toLowerCase();
    const filtered = (found?.items ?? []).filter((i) => !f || i.name.toLowerCase().includes(f) || i.description?.toLowerCase().includes(f));
    const listed = expanded || f ? filtered : filtered.slice(0, FOUND_PREVIEW);
    const allSelected = found !== null && selected.size === found.items.length;
    const toggle = (path: string) => {
      const next = new Set(selected);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      setSelected(next);
    };
    const count = selected.size;
    const primaryLabel = tracking
      ? t('install.url.installRepo')
      : found
        ? t('install.url.install', { items: countLabel(agents ? 'agent' : 'skill', count) })
        : t(isAgent ? 'install.url.findAgents' : 'install.url.findSkills');
    body = (
      <>
        {tabs}
        <div className="ss-fld">
          <label htmlFor={`${ids}src`}>{t('install.url.sourceLabel')}</label>
          <span className="ss-inp">
            <LinkIcon size={15} className="shrink-0 text-ink-3" />
            <input
              id={`${ids}src`}
              autoFocus
              value={source}
              onChange={(e) => { setSource(e.target.value); setFound(null); }}
              onKeyDown={(e) => e.key === 'Enter' && !found && primary()}
              placeholder="owner/repo"
            />
          </span>
          <span className="hp">{t('install.url.sourceHint')}</span>
        </div>

        {found && !tracking && (
          <div>
            <div className="mb-2 flex items-center justify-between gap-3">
              <span className="flex items-center gap-3">
                <span className="text-[13px] font-semibold">
                  {t('install.url.found', { items: countLabel(agents ? 'agent' : 'skill', found.items.length) })}
                </span>
                <button type="button" className="ss-more" onClick={() => setSelected(allSelected ? new Set() : new Set(found.items.map((i) => i.path)))}>
                  {t(allSelected ? 'install.url.deselectAll' : 'install.url.selectAll')}
                </button>
              </span>
              <span className="flex items-center gap-3">
                <span className="text-[13px] text-ink-2">{t('install.url.selected', { count })}</span>
                {found.items.length > FOUND_PREVIEW && (
                  <label className="ss-inp !h-[30px] w-[180px]">
                    <Search size={14} className="shrink-0 text-ink-3" />
                    <input value={filter} onChange={(e) => setFilter(e.target.value)} placeholder={t(agents ? 'install.url.filterAgents' : 'install.url.filterSkills')} aria-label={t(agents ? 'install.url.filterAgents' : 'install.url.filterSkills')} />
                  </label>
                )}
              </span>
            </div>
            {agents !== isAgent && (
              <div className="ss-note inf mb-2.5">
                <Info size={16} />
                <div className="flex-1">{t(agents ? 'install.url.onlyAgents' : 'install.url.onlySkills')}</div>
              </div>
            )}
            {/* Rows scroll inside the list frame once expanded, so the dialog keeps its height */}
            <div className="ss-list !shadow-none flex flex-col">
              <div className="max-h-[264px] overflow-y-auto">
                {listed.map((item) => (
                  <div
                    key={item.path}
                    className={`ss-r !min-h-11 cursor-pointer ${selected.has(item.path) ? 'sel' : ''}`}
                    onClick={(e) => { if (!(e.target as HTMLElement).closest('label')) toggle(item.path); }}
                  >
                    <Checkbox label={item.name} hideLabel checked={selected.has(item.path)} onChange={() => toggle(item.path)} />
                    <span className="nm m w-[170px] shrink-0 truncate">{item.name}</span>
                    <span className="min-w-0 flex-1 truncate text-[13px] text-ink-2">{item.description}</span>
                    {isInstalled(item.kind ?? 'skill', item.name) && <span className="ss-st ok shrink-0 text-ok">Installed</span>}
                  </div>
                ))}
              </div>
              {listed.length < filtered.length && (
                <button type="button" className="ss-r !min-h-[38px] w-full text-left" onClick={() => setExpanded(true)}>
                  <span className="flex-1 text-[13px] text-ink-3">{t('install.url.more', { count: filtered.length - listed.length })}</span>
                  <ChevronDown size={15} className="text-ink-3" />
                </button>
              )}
            </div>
          </div>
        )}

        {canTrack && (
          <div className="flex items-start gap-3.5">
            <button type="button" role="switch" aria-checked={track} aria-labelledby={`${ids}track`} className={`ss-sw mt-0.5 ${track ? 'on' : ''}`} onClick={() => setTrack(!track)}>
              <i />
            </button>
            <div className="flex flex-1 flex-col gap-0.5">
              <span id={`${ids}track`} className="font-semibold">{t('install.url.track')}</span>
              <span className="text-[13px] text-ink-2">{t('install.url.trackHint')}</span>
            </div>
          </div>
        )}

        <button type="button" className="ss-disc self-start" aria-expanded={advanced} onClick={() => setAdvanced(!advanced)}>
          {advanced ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
          {t('install.url.advanced')}
        </button>
        {advanced && (
          <div className="ml-[22px] flex flex-col gap-3.5">
            <div className="grid grid-cols-3 gap-3.5">
              <Field label={t('install.url.into')} hint="--into">
                <input value={into} onChange={(e) => setInto(e.target.value)} placeholder="frontend" />
              </Field>
              {canTrack && (
                <Field label={t('install.url.branch')} hint="--branch" icon={<GitBranch size={15} className="shrink-0 text-ink-3" />}>
                  <input value={branch} onChange={(e) => { setBranch(e.target.value); setFound(null); }} placeholder="main" />
                </Field>
              )}
              <Field label={t('install.url.name')} hint={`--name · ${t('install.url.nameHint')}`} disabled={tracking || (found !== null && count !== 1)}>
                <input value={name} onChange={(e) => setName(e.target.value)} placeholder={t('install.url.namePlaceholder')} disabled={tracking || (found !== null && count !== 1)} />
              </Field>
            </div>
            <Option label={t('install.url.force')} hint={t('install.url.forceHint')} checked={force} onChange={setForce} />
            <Option label={t('install.url.skipAudit')} hint={t('install.url.skipAuditHint')} checked={skipAudit} onChange={setSkipAudit} />
          </div>
        )}
      </>
    );
    foot = (
      <>
        <Button variant="ghost" onClick={onClose} disabled={locked}>{t('common.cancel')}</Button>
        <Button variant="primary" loading={busy === 'primary'} disabled={!src || locked || (found !== null && !tracking && count === 0)} onClick={primary}>
          {busy !== 'primary' && (found || tracking ? <Download size={15} /> : <Search size={15} />)}
          {primaryLabel}
        </Button>
      </>
    );
  }

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={locked} ariaLabel={title} className={WIDTH[view ?? tab]}>
      <div className="dh">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          {sub && <p className="text-[13px] text-ink-2">{sub}</p>}
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={locked}>
          <X size={18} />
        </button>
      </div>
      <div className="db min-h-0 flex-1 overflow-y-auto">{body}</div>
      <div className="df">{foot}</div>
    </DialogShell>
  );
}

function Field({ label, hint, icon, disabled, children }: { label: string; hint: string; icon?: ReactNode; disabled?: boolean; children: ReactNode }) {
  return (
    <label className={`ss-fld min-w-0 ${disabled ? 'opacity-50' : ''}`}>
      <span className="text-[13px] font-semibold">{label}</span>
      <span className="ss-inp">{icon}{children}</span>
      <span className="hp font-mono">{hint}</span>
    </label>
  );
}

function Option({ label, hint, checked, onChange }: { label: string; hint: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <span className="flex items-center gap-2">
      <Checkbox label={label} size="sm" className="font-semibold" checked={checked} onChange={onChange} />
      <span className="text-[13px] text-ink-2">{hint}</span>
    </span>
  );
}
