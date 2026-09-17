import { Fragment, useCallback, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ChevronRight, FileEdit, List, RotateCcw, Save, Search, ShieldCheck } from 'lucide-react';
import { api, type CompiledRule } from '../api/client';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { Select } from '../components/Select';
import { useToast } from '../components/Toast';
import { useAppContext } from '../context/AppContext';
import { useT } from '../i18n';
import { getCachedAuditResult } from '../lib/auditCache';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import AuditRulesYaml from './AuditRulesYaml';

const PROFILES = ['default', 'strict', 'permissive'];
const SEVERITIES = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'];
/** "credential-access" is the rule's pattern id; the group header spells it out. */
const patternLabel = (pattern: string) => pattern.replace(/-/g, ' ').replace(/^./, (c) => c.toUpperCase());

export default function AuditRulesPage() {
  const t = useT();
  const { toast } = useToast();
  const { isProjectMode } = useAppContext();
  const queryClient = useQueryClient();

  const compiled = useQuery({ queryKey: queryKeys.audit.compiled, queryFn: () => api.getCompiledRules(), staleTime: staleTimes.auditRules });
  const overview = useQuery({ queryKey: queryKeys.overview, queryFn: () => api.getOverview(), staleTime: staleTimes.overview });

  const [yamlView, setYamlView] = useState(false);
  const [search, setSearch] = useState('');
  const [show, setShow] = useState('ALL');
  const [open, setOpen] = useState<Set<string>>(new Set());
  const [openRule, setOpenRule] = useState<string | null>(null);
  const [confirmReset, setConfirmReset] = useState(false);
  const [yamlSave, setYamlSave] = useState<{ dirty: boolean; saving: boolean; save: () => void } | null>(null);

  const onYamlSaveState = useCallback((dirty: boolean, saving: boolean, save: () => void) => setYamlSave({ dirty, saving, save }), []);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.audit.compiled });
    queryClient.invalidateQueries({ queryKey: queryKeys.audit.rules });
  };
  const toggle = useMutation({
    mutationFn: (req: { id?: string; pattern?: string; enabled: boolean; severity?: string }) => api.toggleRule(req),
    onSuccess: invalidate,
    onError: (e: Error) => toast(e.message, 'error'),
  });
  const reset = useMutation({
    mutationFn: () => api.resetRules(),
    onSuccess: () => { invalidate(); toast(t('auditRules.toast.resetSuccess'), 'success'); },
    onError: (e: Error) => toast(e.message, 'error'),
  });
  const setProfile = useMutation({
    mutationFn: (profile: string) => api.setAuditProfile(profile),
    onSuccess: (res) => { invalidate(); toast(t('auditRules.toast.profileSaved', { profile: res.profile }), 'success'); },
    onError: (e: Error) => toast(e.message, 'error'),
  });

  const rules = compiled.data?.rules ?? [];
  const stats = useMemo(() => {
    let enabled = 0, custom = 0;
    for (const r of rules) {
      if (r.enabled) enabled++;
      if (r.source !== 'builtin') custom++;
    }
    return { total: rules.length, enabled, disabled: rules.length - enabled, custom };
  }, [rules]);

  // Findings from the last scan, so a rule can show what it catches right now.
  const scan = getCachedAuditResult(queryClient, 'skills', overview.data?.skillCount);
  const hits = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const result of scan?.results ?? []) {
      for (const f of result.findings) {
        if (!f.ruleId) continue;
        const names = map.get(f.ruleId) ?? [];
        if (!names.includes(result.skillName)) names.push(result.skillName);
        map.set(f.ruleId, names);
      }
    }
    return map;
  }, [scan]);

  const groups = useMemo(() => {
    const term = search.trim().toLowerCase();
    const map = new Map<string, CompiledRule[]>();
    for (const r of rules) {
      if (show === 'DISABLED' ? r.enabled : show !== 'ALL' && r.severity !== show) continue;
      if (term && !`${r.id} ${r.message} ${r.regex} ${r.pattern}`.toLowerCase().includes(term)) continue;
      map.set(r.pattern, [...(map.get(r.pattern) ?? []), r]);
    }
    return [...map.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [rules, search, show]);

  const allOpen = groups.length > 0 && groups.every(([p]) => open.has(p));
  const toggleGroup = (pattern: string) => setOpen((s) => { const next = new Set(s); if (!next.delete(pattern)) next.add(pattern); return next; });

  const header = (
    <PageHeader className="!mb-0" title={t('audit.header.title')} subtitle={t('audit.header.subtitle')} />
  );

  const tabs = (
    <nav className="ss-tabs" aria-label={t('audit.header.title')}>
      <Link to="/audit">{t('audit.tab.findings')}</Link>
      <Link to="/audit/rules" className="on" aria-current="page">{t('audit.tab.rules')}</Link>
    </nav>
  );

  if (yamlView) {
    return (
      <div className="ss-wrap animate-fade-in">
        {header}
        {tabs}
        <div className="flex flex-wrap items-center gap-3">
          <span className="text-[13px] text-ink-2">{isProjectMode ? t('auditRules.header.subtitleProject') : t('auditRules.header.subtitleGlobal')}</span>
          <span className="flex-1" />
          <Button variant="ghost" size="sm" onClick={() => setYamlView(false)}><List size={14} />{t('auditRules.header.ruleBrowser')}</Button>
          <Button variant="primary" size="sm" disabled={!yamlSave?.dirty || yamlSave?.saving} onClick={() => yamlSave?.save()}>
            <Save size={14} />
            {t(yamlSave?.saving ? 'auditRules.header.saving' : 'auditRules.header.save')}
          </Button>
        </div>
        <AuditRulesYaml
          isProjectMode={isProjectMode}
          onSaveStateChange={onYamlSaveState}
        />
      </div>
    );
  }

  return (
    <div className="ss-wrap animate-fade-in">
      {header}
      {tabs}

      <div className="flex flex-wrap items-center gap-3">
        <span className="text-[13px] text-ink-2">{t('auditRules.profile.label')}</span>
        <div className="ss-seg" role="radiogroup" aria-label={t('auditRules.profile.label')}>
          {PROFILES.map((p) => (
            <button
              key={p}
              type="button"
              role="radio"
              aria-checked={compiled.data?.profile === p}
              className={compiled.data?.profile === p ? 'on' : ''}
              disabled={setProfile.isPending}
              onClick={() => setProfile.mutate(p)}
            >
              {p}
            </button>
          ))}
        </div>
        <label className="ss-inp w-[210px]">
          <Search size={15} className="text-ink-3" />
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t('auditRules.search.placeholder')} aria-label={t('auditRules.search.placeholder')} />
        </label>
        <Select
          value={show}
          onChange={setShow}
          prefix={t('auditRules.show.label')}
          className="w-[160px]"
          options={[{ value: 'ALL', label: t('auditRules.show.all') }, ...SEVERITIES.map((s) => ({ value: s, label: s })), { value: 'DISABLED', label: t('auditRules.show.disabled') }]}
        />
        <span className="flex-1" />
        <Button variant="ghost" size="sm" onClick={() => setOpen(allOpen ? new Set() : new Set(groups.map(([p]) => p)))}>
          {t(allOpen ? 'auditRules.search.collapseAll' : 'auditRules.search.expandAll')}
        </Button>
        <Button variant="ghost" size="sm" loading={reset.isPending} onClick={() => setConfirmReset(true)}>
          {!reset.isPending && <RotateCcw size={14} />}
          {t('auditRules.header.resetAll')}
        </Button>
        <Button variant="secondary" size="sm" onClick={() => setYamlView(true)}><FileEdit size={14} />{t('auditRules.header.editYaml')}</Button>
      </div>

      {compiled.error ? (
        <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1 break-words">{compiled.error.message}</span></div>
      ) : (
        <p className="text-[13px] text-ink-3">
          {stats.total} {t('auditRules.stats.rulesLabel')} · {t('auditRules.stats.enabled', { count: stats.enabled })}
          {stats.disabled > 0 && <> · {t('auditRules.stats.disabled', { count: stats.disabled })}</>}
          {stats.custom > 0 && <> · {t('auditRules.stats.custom', { count: stats.custom })}</>}
        </p>
      )}

      {groups.length === 0 ? (
        !compiled.isPending && !compiled.error && (
          <EmptyState icon={ShieldCheck} title={t('auditRules.empty.noMatch.title')} description={t('auditRules.empty.noMatch.description')} />
        )
      ) : (
        <div className="ss-list">
          {groups.map(([pattern, groupRules]) => {
            const expanded = open.has(pattern);
            return (
              <Fragment key={pattern}>
                <div className="ss-gh !min-h-[46px]">
                  <button type="button" className="flex items-center gap-2 font-semibold" aria-expanded={expanded} onClick={() => toggleGroup(pattern)}>
                    <ChevronRight size={14} className={`transition-transform ${expanded ? 'rotate-90' : ''}`} />
                    {patternLabel(pattern)}
                  </button>
                  <span className="text-[13px] text-ink-3">{t(groupRules.length === 1 ? 'auditRules.pattern.rule' : 'auditRules.pattern.rules', { count: groupRules.length })}</span>
                  <span className="flex-1" />
                  {expanded && (
                    <>
                      <Button variant="ghost" size="sm" onClick={() => toggle.mutate({ pattern, enabled: true })}>{t('auditRules.pattern.enableAll')}</Button>
                      <Button variant="ghost" size="sm" onClick={() => toggle.mutate({ pattern, enabled: false })}>{t('auditRules.pattern.disableAll')}</Button>
                      <Select
                        size="sm"
                        value=""
                        onChange={(severity) => toggle.mutate({ pattern, enabled: true, severity })}
                        prefix={t('auditRules.pattern.groupSeverity')}
                        className="w-[170px]"
                        options={SEVERITIES.map((s) => ({ value: s, label: s }))}
                      />
                    </>
                  )}
                </div>
                {expanded && groupRules.map((rule) => (
                  <RuleRow
                    key={rule.id}
                    rule={rule}
                    hits={hits.get(rule.id)}
                    expanded={openRule === rule.id}
                    onExpand={() => setOpenRule(openRule === rule.id ? null : rule.id)}
                    onToggle={(enabled) => toggle.mutate({ id: rule.id, enabled })}
                    onSeverity={(severity) => toggle.mutate({ id: rule.id, enabled: true, severity })}
                  />
                ))}
              </Fragment>
            );
          })}
        </div>
      )}

      <ConfirmDialog
        open={confirmReset}
        variant="danger"
        title={t('auditRules.confirm.title')}
        message={t('auditRules.confirm.message')}
        confirmText={t('auditRules.confirm.confirmText')}
        cancelText={t('auditRules.confirm.cancel')}
        onCancel={() => setConfirmReset(false)}
        onConfirm={() => { setConfirmReset(false); reset.mutate(); }}
      />
    </div>
  );
}

function RuleRow({ rule, hits, expanded, onExpand, onToggle, onSeverity }: {
  rule: CompiledRule;
  hits?: string[];
  expanded: boolean;
  onExpand: () => void;
  onToggle: (enabled: boolean) => void;
  onSeverity: (severity: string) => void;
}) {
  const t = useT();
  return (
    <>
      <div className="ss-r !min-h-[46px]">
        <button
          type="button"
          role="switch"
          aria-checked={rule.enabled}
          aria-label={t(rule.enabled ? 'auditRules.toggle.disable' : 'auditRules.toggle.enable')}
          className={`ss-sw ${rule.enabled ? 'on' : ''}`}
          onClick={() => onToggle(!rule.enabled)}
        >
          <i />
        </button>
        <button type="button" className="flex min-w-0 flex-1 flex-col items-start gap-px text-left" aria-expanded={expanded} onClick={onExpand}>
          <span className={`truncate max-w-full text-[13.5px] font-semibold ${rule.enabled ? '' : 'text-ink-3'}`}>{rule.message}</span>
          <span className="font-mono text-[12px] text-ink-3">{rule.id}</span>
        </button>
        {hits && <span className="ss-tag warn shrink-0">{t('auditRules.detail.matchCount', { count: hits.length })}</span>}
        <Select size="sm" value={rule.severity} onChange={onSeverity} className="w-[130px] shrink-0" options={SEVERITIES.map((s) => ({ value: s, label: s }))} />
      </div>
      {expanded && (
        <div className="ss-r !items-start bg-sunken !py-3.5 !pl-[58px]">
          <dl className="ss-kv flex-1 !grid-cols-[110px_minmax(0,1fr)]">
            <dt>{t('auditRules.detail.regex')}</dt>
            <dd className="break-all font-mono text-ink-2">{rule.regex}</dd>
            {rule.exclude && (
              <>
                <dt>{t('auditRules.detail.exclude')}</dt>
                <dd className="break-all font-mono text-ink-2">{rule.exclude}</dd>
              </>
            )}
            <dt>{t('auditRules.detail.source')}</dt>
            <dd>{rule.source}</dd>
            {hits && (
              <>
                <dt>{t('auditRules.detail.matches')}</dt>
                <dd className="truncate font-mono" title={hits.join(', ')}>{hits.join(', ')}</dd>
              </>
            )}
          </dl>
          {hits && <Link to="/audit" className="ss-btn sm shrink-0">{t('auditRules.detail.viewFinding')}</Link>}
        </div>
      )}
    </>
  );
}
