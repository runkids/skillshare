import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Button from '../components/Button';
import CodeView from '../components/CodeView';
import PageHeader from '../components/PageHeader';
import { Checkbox, Select } from '../components/Input';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import { useT } from '../i18n';

const NAME_REGEX = /^[a-z_][a-z0-9_-]*$/;
const MAX_DESCRIPTION = 1024;
// Display order for scaffold folders; the blank template may create any of them
const DIR_ORDER = ['scripts', 'references', 'assets'];

export default function NewSkillPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const t = useT();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [patternName, setPatternName] = useState('none');
  const [category, setCategory] = useState('');
  const [dirs, setDirs] = useState<Set<string>>(new Set());
  const [into, setInto] = useState('');
  const [creating, setCreating] = useState(false);

  const { data: templates, isPending } = useQuery({
    queryKey: queryKeys.templates,
    queryFn: () => api.getTemplates(),
    staleTime: staleTimes.config,
  });
  const { data: skillsData } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const { data: overview } = useQuery({
    queryKey: queryKeys.overview,
    queryFn: () => api.getOverview(),
    staleTime: staleTimes.overview,
  });

  const skills = useMemo(() => (skillsData?.resources ?? []).filter((r) => r.kind === 'skill'), [skillsData]);
  // Folders that already hold local skills. Tracked repos are git clones, so new skills stay out of them
  const folders = useMemo(() => {
    const set = new Set<string>();
    for (const r of skills) {
      if (r.isInRepo) continue;
      const parts = r.relPath.split('/').slice(0, -1);
      parts.forEach((_, i) => set.add(parts.slice(0, i + 1).join('/')));
    }
    return [...set].sort();
  }, [skills]);

  const patterns = useMemo(
    () => [...(templates?.patterns ?? [])].sort((a, b) => Number(b.name === 'none') - Number(a.name === 'none')),
    [templates],
  );
  const pattern = patterns.find((p) => p.name === patternName);
  const allDirs = useMemo(() => new Set(patterns.flatMap((p) => p.scaffoldDirs)), [patterns]);
  const dirOptions = DIR_ORDER.filter((d) => (pattern?.scaffoldDirs.length ? pattern.scaffoldDirs : [...allDirs]).includes(d));

  const relPath = into ? `${into}/${name}` : name;
  const nameError = !name
    ? null
    : !NAME_REGEX.test(name)
      ? t('newSkill.name.validation.format')
      : skills.some((r) => r.relPath === relPath)
        ? t('newSkill.name.validation.alreadyExists')
        : null;
  const tooLong = description.length > MAX_DESCRIPTION;
  const canCreate = name !== '' && !nameError && !tooLong && !creating;

  // The preview comes from the server's own template, debounced while typing
  const request = { name: name && !nameError ? name : 'my-skill', pattern: patternName, category: pattern?.name !== 'none' ? category : '', description, into };
  const requestKey = JSON.stringify(request);
  const [previewRequest, setPreviewRequest] = useState(request);
  useEffect(() => {
    const timer = window.setTimeout(() => setPreviewRequest(JSON.parse(requestKey)), 250);
    return () => window.clearTimeout(timer);
  }, [requestKey]);
  const { data: preview } = useQuery({
    queryKey: queryKeys.skillPreview(previewRequest),
    queryFn: () => api.previewSkill(previewRequest),
    placeholderData: keepPreviousData,
    enabled: !tooLong,
  });

  const pickPattern = (next: string) => {
    setPatternName(next);
    setDirs(new Set(patterns.find((p) => p.name === next)?.scaffoldDirs ?? []));
  };

  const toggleDir = (dir: string) => {
    const next = new Set(dirs);
    if (next.has(dir)) next.delete(dir);
    else next.add(dir);
    setDirs(next);
  };

  const create = async () => {
    if (!canCreate) return;
    setCreating(true);
    try {
      const res = await api.createSkill({ ...request, name, scaffoldDirs: dirOptions.filter((d) => dirs.has(d)) });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      toast(t('newSkill.toast.created', { name: res.skill.name }), 'success');
      navigate(`/skills/${encodeURIComponent(res.skill.flatName)}`);
    } catch (e) {
      toast((e as Error).message, 'error');
      setCreating(false);
    }
  };

  if (isPending) return <PageSkeleton />;

  const sourceDir = overview?.source.replace(/\/+$/, '').split('/').pop() || 'skills';

  return (
    <div className="animate-fade-in">
      <PageHeader
        title={t('newSkill.title')}
        subtitle={t('newSkill.subtitle')}
        crumbs={[{ label: t('layout.nav.skills'), to: '/skills' }, { label: t('newSkill.title') }]}
      />

      <div className="grid grid-cols-2 items-start gap-12">
        <form className="flex min-w-0 flex-col gap-[22px]" onSubmit={(e) => { e.preventDefault(); void create(); }}>
          <div className="ss-fld">
            <label htmlFor="new-skill-name">{t('newSkill.name.label')}</label>
            <span className="ss-inp">
              <input
                id="new-skill-name"
                autoFocus
                value={name}
                placeholder={t('newSkill.name.placeholder')}
                onChange={(e) => setName(e.target.value.toLowerCase())}
                aria-invalid={nameError ? true : undefined}
              />
            </span>
            <span className="hp">
              {nameError ? <span className="text-bad">{nameError}</span> : name && <span className="text-ok">{t('newSkill.name.available')}</span>}{' '}
              {t('newSkill.name.hint')}
            </span>
          </div>

          <div className="ss-fld">
            <label htmlFor="new-skill-description">
              {t('newSkill.description.label')}{' '}
              <span className={`font-normal ${tooLong ? 'text-bad' : 'text-ink-3'}`}>· {description.length} / {MAX_DESCRIPTION}</span>
            </label>
            <textarea
              id="new-skill-description"
              className="ss-inp area min-h-[84px] resize-y outline-none"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
            <span className="hp">{t('newSkill.description.hint')}</span>
          </div>

          <div className="ss-fld">
            <span id="new-skill-pattern" className="text-[13px] font-semibold">{t('newSkill.pattern.label')}</span>
            <div role="radiogroup" aria-labelledby="new-skill-pattern" className="grid grid-cols-2 gap-2">
              {patterns.map((p) => (
                <button
                  key={p.name}
                  type="button"
                  role="radio"
                  aria-checked={p.name === patternName}
                  className={`ss-pick text-left ${p.name === patternName ? 'on' : ''}`}
                  onClick={() => pickPattern(p.name)}
                >
                  <span className={`ss-chk rad ${p.name === patternName ? 'on' : ''}`} />
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="font-semibold">{t(`newSkill.pattern.${p.name}.label`, {}, p.name)}</span>
                    <span className="text-[13px] text-ink-2">{t(`newSkill.pattern.${p.name}.description`, {}, p.description)}</span>
                  </span>
                </button>
              ))}
            </div>
          </div>

          {patternName !== 'none' && (
            <div className="ss-fld">
              <span className="text-[13px] font-semibold">{t('newSkill.category.label')}</span>
              <Select
                value={category}
                onChange={setCategory}
                options={[
                  { value: '', label: t('newSkill.category.noCategory') },
                  ...(templates?.categories ?? []).map((c) => ({ value: c.key, label: t(`newSkill.category.${c.key}.label`, {}, c.label) })),
                ]}
              />
            </div>
          )}

          {dirOptions.length > 0 && (
            <div role="group" aria-labelledby="new-skill-dirs" className="flex flex-col gap-1.5">
              <span id="new-skill-dirs" className="text-[13px] font-semibold">{t('newSkill.folders.label')}</span>
              <div className="flex flex-wrap gap-[18px]">
                {dirOptions.map((d) => (
                  <Checkbox key={d} label={`${d}/`} checked={dirs.has(d)} onChange={() => toggleDir(d)} className="font-mono text-[13px]" />
                ))}
              </div>
            </div>
          )}

          <div className="ss-fld">
            <span className="text-[13px] font-semibold">{t('newSkill.folder.label')}</span>
            <Select
              value={into}
              onChange={setInto}
              options={[{ value: '', label: t('newSkill.folder.top', { dir: sourceDir }) }, ...folders.map((f) => ({ value: f, label: `${f}/` }))]}
            />
          </div>

          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => navigate('/skills')}>{t('common.cancel')}</Button>
            <Button type="submit" variant="primary" loading={creating} disabled={!canCreate}>{t('newSkill.createSkill')}</Button>
          </div>
        </form>

        {/* The preview stays in view while the form scrolls; the offset clears the fixed account avatar */}
        <div className="sticky top-20 flex min-w-0 flex-col">
          <div className="ss-sec"><h2 className="ss-h2">{t('newSkill.preview.title')}</h2></div>
          <CodeView content={preview?.content ?? ''} lang="md" className="max-h-[calc(100vh-190px)] min-h-[330px]" />
          {preview && <p className="mt-2.5 truncate font-mono text-[13px] text-ink-3" title={preview.path}>{preview.path}</p>}
        </div>
      </div>
    </div>
  );
}
