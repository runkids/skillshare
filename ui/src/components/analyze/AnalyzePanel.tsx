import { useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { AlertCircle, AlertTriangle, ArrowDown, Puzzle, Search, X } from 'lucide-react';
import { api, type AnalyzeSkill, type AnalyzeTarget } from '../../api/client';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import DialogShell from '../DialogShell';
import EmptyState from '../EmptyState';
import { Select } from '../Select';
import Spinner from '../Spinner';
import { useT } from '../../i18n';
import { formatSkillDisplayName } from '../../lib/resourceNames';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import { segSlide } from '../../lib/segSlide';

const STEP = 100;
const MANAGED_ONLY_KEY = 'skillshare:analyze-managed-only';
/** Thousands collapse to "3.4k" so the column stays one glance wide; smaller counts stay exact. */
const kTokens = (n: number) => (n < 1000 ? n.toLocaleString() : n >= 10000 ? `${Math.round(n / 1000)}k` : `${(n / 1000).toFixed(1)}k`);
/** "description-length" reads as "description length" — the full sentence waits in the dialog. */
const ruleLabel = (rule: string) => rule.replace(/-/g, ' ');
/** A local skill's path is wherever the tool put it (e.g. synced/<org>_<uuid>/docx), so show just its folder. */
const skillLabel = (s: AnalyzeSkill) => (s.local ? s.path.split('/').pop() || s.path : formatSkillDisplayName(s.name));
/** Project targets are named `project@tool`; own targets have no `@`. */
const projectOf = (name: string) => (name.includes('@') ? name.slice(0, name.lastIndexOf('@')) : '');
/** Exact name first. A project link may name a tool that shares its folder with another, so any target of that project will do. */
function pickTarget(targets: AnalyzeTarget[], want: string) {
  const project = projectOf(want);
  return targets.find((x) => x.name === want) ?? (project ? targets.find((x) => projectOf(x.name) === project) : undefined) ?? targets[0];
}

/** Context cost of the skills a target loads: descriptions always, bodies on demand. */
export default function AnalyzePanel() {
  const t = useT();
  const { data, isPending, error } = useQuery({ queryKey: queryKeys.analyze, queryFn: () => api.analyze(), staleTime: staleTimes.analyze });

  // The target lives in the URL so target and project pages can deep-link here.
  const [params, setParams] = useSearchParams();
  const target = params.get('target') ?? '';
  const setTarget = (v: string) => setParams((p) => { p.set('target', v); return p; }, { replace: true });
  const [search, setSearch] = useState('');
  const [onlyIssues, setOnlyIssues] = useState(false);
  const [managedOnly, setManagedOnly] = useState(() => {
    try { return localStorage.getItem(MANAGED_ONLY_KEY) === '1'; } catch { return false; }
  });
  const [limit, setLimit] = useState(STEP);
  const [detail, setDetail] = useState<AnalyzeSkill | null>(null);

  if (isPending) return <div className="ss-note inf !items-center"><Spinner size="sm" />{t('analyze.loading')}</div>;
  if (error) return <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1 break-words">{(error as Error).message}</span></div>;

  const targets = data?.targets ?? [];
  if (targets.length === 0) return <EmptyState icon={Puzzle} title={t('analyze.empty.title')} description={t('analyze.empty.description')} />;

  const current = pickTarget(targets, target);
  // Own targets first, project targets (`project@tool`) after, each keeping the API's heaviest-first order.
  const options = [...targets]
    .sort((a, b) => Number(Boolean(projectOf(a.name))) - Number(Boolean(projectOf(b.name))))
    .map((x) => ({ value: x.name, label: x.name, icon: <AgentIcon target={x.name} size={15} /> }));
  const hasLocal = current.skills.some((s) => s.local);
  const skills = current.skills.filter((s) => !(managedOnly && s.local)).sort((a, b) => b.body_tokens - a.body_tokens);
  // Summed here rather than read from the API totals so they follow the managed-only switch.
  const alwaysLoaded = skills.reduce((n, s) => n + s.description_tokens, 0);
  const onDemand = skills.reduce((n, s) => n + s.body_tokens, 0);
  const toggleManagedOnly = (on: boolean) => {
    setManagedOnly(on);
    setLimit(STEP);
    try { localStorage.setItem(MANAGED_ONLY_KEY, on ? '1' : '0'); } catch { /* storage unavailable */ }
  };
  const issueCount = skills.filter((s) => s.lint_issues?.length).length;
  const heaviest = skills[0]?.body_tokens ?? 0;
  const term = search.trim().toLowerCase();
  const filtered = skills.filter((s) => (!onlyIssues || s.lint_issues?.length) && (!term || skillLabel(s).toLowerCase().includes(term)));

  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-center justify-between gap-6">
        <p className="max-w-[640px] text-[13px] leading-relaxed text-ink-2">{t('analyze.intro')}</p>
        <Select
          value={current.name}
          onChange={(v) => { setTarget(v); setLimit(STEP); }}
          prefix={t('analyze.target')}
          className="w-[260px] shrink-0"
          options={options}
        />
      </div>

      <div className="ss-counts !grid-cols-3">
        <div>
          <span className="flex flex-col gap-[3px]">
            <b>{kTokens(alwaysLoaded)}</b>
            <span className="text-[13px] text-ink-2">{t('analyze.stat.alwaysLoaded')}</span>
          </span>
        </div>
        <div>
          <span className="flex flex-col gap-[3px]">
            <b>{kTokens(onDemand)}</b>
            <span className="text-[13px] text-ink-2">{t('analyze.stat.onDemand')}</span>
          </span>
        </div>
        <button type="button" aria-pressed={onlyIssues} disabled={issueCount === 0} onClick={() => { setOnlyIssues(!onlyIssues); setLimit(STEP); }}>
          <span className="flex flex-col items-start gap-[3px]">
            <b className={issueCount === 0 ? 'text-ink-3' : onlyIssues ? 'text-accent' : 'text-warn'}>{issueCount}</b>
            <span className="text-[13px] text-ink-2">{t('analyze.stat.issues')}</span>
          </span>
        </button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <label className="ss-inp w-[220px] shrink-0">
          <Search size={15} className="text-ink-3" />
          <input value={search} onChange={(e) => { setSearch(e.target.value); setLimit(STEP); }} placeholder={t('analyze.search')} aria-label={t('analyze.search')} />
        </label>
        <div ref={segSlide} className="ss-seg" role="tablist">
          <button type="button" role="tab" aria-selected={!onlyIssues} className={onlyIssues ? '' : 'on'} onClick={() => { setOnlyIssues(false); setLimit(STEP); }}>
            {t('analyze.filter.all')}
          </button>
          <button type="button" role="tab" aria-selected={onlyIssues} className={onlyIssues ? 'on' : ''} disabled={issueCount === 0} onClick={() => { setOnlyIssues(true); setLimit(STEP); }}>
            {t('analyze.filter.issues')}
            {issueCount > 0 && <span className="ss-cnt">{issueCount}</span>}
          </button>
        </div>
        {hasLocal && <Checkbox size="sm" className="ml-2" label={t('analyze.filter.managedOnly')} checked={managedOnly} onChange={toggleManagedOnly} />}
      </div>

      {filtered.length === 0 ? (
        <EmptyState icon={Search} title={t('analyze.noMatch.title')} description={t('analyze.noMatch.description')} />
      ) : (
        <div className="ss-list">
          <div className="ss-lh">
            <span className="w-[34px]" />
            <span className="flex-1">{t('analyze.col.skill')}</span>
            <span className="w-[110px]">{t('analyze.col.alwaysLoaded')}</span>
            <span className="flex w-[240px] items-center gap-1.5">{t('analyze.col.onDemand')}<ArrowDown size={13} /></span>
          </div>
          {filtered.slice(0, limit).map((s) => (
            <button key={s.name} type="button" className="ss-r w-full text-left" onClick={() => setDetail(s)}>
              <span className="ss-cat sm skill"><Puzzle size={14} /></span>
              <span className="nm m truncate" title={s.local ? s.path : undefined}>{skillLabel(s)}</span>
              {s.lint_issues?.length ? (
                <span className="ss-tag warn shrink-0" title={s.lint_issues.map((i) => i.message).join(' · ')}>{ruleLabel(s.lint_issues[0].rule)}</span>
              ) : null}
              {s.local && <span className="ss-tag shrink-0">{t('analyze.tag.local')}</span>}
              {s.disabled && <span className="ss-tag warn shrink-0">{t('analyze.tag.disabled')}</span>}
              <span className="flex-1" />
              <span className="w-[110px] font-mono text-[13px]">{s.description_tokens.toLocaleString()}</span>
              <span className="flex w-[240px] items-center gap-2.5">
                <span className="ss-prog flex-1"><i style={{ width: `${heaviest ? (s.body_tokens / heaviest) * 100 : 0}%` }} /></span>
                <span className="w-[38px] text-right font-mono text-[13px]">{kTokens(s.body_tokens)}</span>
              </span>
            </button>
          ))}
          {filtered.length > limit && (
            <div className="ss-r justify-center">
              <Button variant="ghost" size="sm" onClick={() => setLimit((l) => l + STEP)}>{t('resources.showMore')}</Button>
            </div>
          )}
        </div>
      )}

      <p className="text-[13px] leading-relaxed text-ink-3">{t('analyze.footnote')}</p>

      {detail && <SkillDialog skill={detail} onClose={() => setDetail(null)} />}
    </div>
  );
}

function SkillDialog({ skill, onClose }: { skill: AnalyzeSkill; onClose: () => void }) {
  const t = useT();
  return (
    <DialogShell open onClose={onClose} padding="none" ariaLabel={skill.name} className="!max-w-[560px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
            <h2 className="ss-h2 font-mono">{skillLabel(skill)}</h2>
          {skill.local && <p className="font-mono text-[12.5px] text-ink-3 break-all">{skill.path}</p>}
          <p className="text-[13px] text-ink-2">{t('analyze.detail.subtitle')}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose}><X size={16} /></button>
      </div>
      <div className="db">
        <dl className="ss-kv !grid-cols-[150px_minmax(0,1fr)]">
          <dt>{t('analyze.col.alwaysLoaded')}</dt>
          <dd><span className="font-mono font-semibold">{skill.description_tokens.toLocaleString()}</span> {t('analyze.detail.alwaysLoadedDesc')}</dd>
          <dt>{t('analyze.col.onDemand')}</dt>
          <dd><span className="font-mono font-semibold">{kTokens(skill.body_tokens)}</span> {t('analyze.detail.onDemandDesc')}</dd>
          {skill.targets?.length ? (
            <>
              <dt>{t('analyze.detail.restrictedTo')}</dt>
              <dd className="flex flex-wrap items-center gap-1.5">
                {skill.targets.map((name) => <span key={name} className="ss-at"><AgentIcon target={name} size={15} /></span>)}
                <span className="text-[13px] text-ink-2">{skill.targets.join(', ')}</span>
              </dd>
            </>
          ) : null}
        </dl>

        {skill.local && <div className="ss-note inf"><span className="flex-1">{t('analyze.detail.local')}</span></div>}
        {skill.disabled && <div className="ss-note warn"><AlertTriangle size={16} /><span className="flex-1">{t('analyze.detail.disabled')}</span></div>}

        {skill.description && (
          <div className="ss-fld">
            <label>{t('analyze.detail.descriptionPreview')}</label>
            <div className="ss-box max-h-[120px] overflow-y-auto !px-3.5 !py-3 !shadow-none text-[13px] leading-relaxed">{skill.description}</div>
          </div>
        )}

        {skill.lint_issues?.length ? (
          <div className="ss-fld">
            <label>{t('analyze.detail.qualityIssues')}</label>
            {skill.lint_issues.map((issue) => (
              <div key={issue.rule} className={`ss-note ${issue.severity === 'error' ? 'bad' : 'warn'}`}>
                <AlertTriangle size={16} />
                <span className="flex-1">{issue.message}</span>
              </div>
            ))}
          </div>
        ) : null}

        <p className="text-[12.5px] text-ink-3">{t('analyze.detail.note')}</p>
      </div>
      <div className="df">
        <span className="flex-1" />
        <Button variant="ghost" onClick={onClose}>{t('common.close')}</Button>
        {!skill.local && <Link to={`/skills/${encodeURIComponent(skill.name)}`} className="ss-btn pri">{t('analyze.detail.viewSkill')}</Link>}
      </div>
    </DialogShell>
  );
}
