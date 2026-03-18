import { useState, useMemo, useCallback, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, ArrowRight, Check, FolderPlus } from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import { Input } from '../components/Input';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { SkillPattern, SkillCategory } from '../api/client';

/* -- Step definitions -------------------------------- */

type StepId = 'name' | 'pattern' | 'category' | 'scaffold' | 'confirm';

function computeSteps(selectedPattern: SkillPattern | null): StepId[] {
  const steps: StepId[] = ['name', 'pattern'];
  if (selectedPattern && selectedPattern.name !== 'none') {
    steps.push('category');
    if (selectedPattern.scaffoldDirs.length > 0) {
      steps.push('scaffold');
    }
  }
  steps.push('confirm');
  return steps;
}

/* -- Name validation --------------------------------- */

const NAME_REGEX = /^[a-z_][a-z0-9_-]*$/;

function validateName(name: string, existingNames: Set<string>): string | null {
  if (!name) return 'Name is required';
  if (!NAME_REGEX.test(name)) return 'Must start with a-z or _, and contain only a-z, 0-9, _ or -';
  if (existingNames.has(name)) return 'A skill with this name already exists';
  return null;
}

/* -- Main wizard component --------------------------- */

export default function NewSkillPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  // Form state
  const [name, setName] = useState('');
  const [selectedPattern, setSelectedPattern] = useState<SkillPattern | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<SkillCategory | null>(null);
  const [scaffoldDirs, setScaffoldDirs] = useState<Set<string>>(new Set());
  const [creating, setCreating] = useState(false);

  // Step navigation
  const [stepIndex, setStepIndex] = useState(0);

  // Fetch templates
  const { data: templatesData, isPending: templatesPending } = useQuery({
    queryKey: queryKeys.templates,
    queryFn: () => api.getTemplates(),
    staleTime: staleTimes.config,
  });

  // Fetch existing skills for duplicate check
  const { data: skillsData } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });

  const existingNames = useMemo(() => {
    const names = new Set<string>();
    if (skillsData?.skills) {
      for (const s of skillsData.skills) {
        names.add(s.name);
      }
    }
    return names;
  }, [skillsData]);

  const patterns = templatesData?.patterns ?? [];
  const categories = templatesData?.categories ?? [];

  // Compute dynamic steps
  const steps = useMemo(() => computeSteps(selectedPattern), [selectedPattern]);
  const currentStep = steps[stepIndex] ?? 'name';

  // When pattern changes, reset downstream state
  const handlePatternSelect = useCallback((pattern: SkillPattern) => {
    setSelectedPattern(pattern);
    setSelectedCategory(null);
    setScaffoldDirs(new Set(pattern.scaffoldDirs));
  }, []);

  // Toggle a scaffold directory
  const toggleScaffoldDir = useCallback((dir: string) => {
    setScaffoldDirs((prev) => {
      const next = new Set(prev);
      if (next.has(dir)) next.delete(dir);
      else next.add(dir);
      return next;
    });
  }, []);

  // Step validation
  const nameError = useMemo(() => {
    if (!name) return null; // Don't show error for empty (user hasn't typed yet)
    return validateName(name, existingNames);
  }, [name, existingNames]);

  const canAdvance = useMemo(() => {
    switch (currentStep) {
      case 'name':
        return name.length > 0 && nameError === null;
      case 'pattern':
        return selectedPattern !== null;
      case 'category':
      case 'scaffold':
      case 'confirm':
        return true;
      default:
        return false;
    }
  }, [currentStep, name, nameError, selectedPattern]);

  // Navigation
  const goNext = useCallback(() => {
    if (stepIndex < steps.length - 1) {
      setStepIndex(stepIndex + 1);
      window.history.pushState({ step: stepIndex + 1 }, '');
    }
  }, [stepIndex, steps.length]);

  const goBack = useCallback(() => {
    if (stepIndex > 0) {
      setStepIndex(stepIndex - 1);
    } else {
      navigate('/skills');
    }
  }, [stepIndex, navigate]);

  // Listen for browser back button
  useEffect(() => {
    const handler = (e: PopStateEvent) => {
      const step = e.state?.step;
      if (typeof step === 'number') {
        setStepIndex(step);
      } else {
        setStepIndex(0);
      }
    };
    window.addEventListener('popstate', handler);
    return () => window.removeEventListener('popstate', handler);
  }, []);

  // Push initial history entry on mount
  useEffect(() => {
    window.history.replaceState({ step: 0 }, '');
  }, []);

  // Create skill
  const handleCreate = async () => {
    setCreating(true);
    try {
      const res = await api.createSkill({
        name,
        pattern: selectedPattern?.name ?? 'none',
        category: selectedCategory?.key,
        scaffoldDirs: selectedPattern && selectedPattern.scaffoldDirs.length > 0
          ? [...scaffoldDirs]
          : undefined,
      });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      toast(`Skill "${res.skill.name}" created successfully!`, 'success');
      navigate(`/skills/${encodeURIComponent(res.skill.flatName)}`);
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setCreating(false);
    }
  };

  if (templatesPending) return <PageSkeleton />;

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<></>}
        title="Create New Skill"
        backTo="/skills"
      />

      {/* Progress bar */}
      <ProgressBar current={stepIndex} steps={steps} />

      {/* Step content */}
      <div>
        {currentStep === 'name' && (
          <NameStep
            value={name}
            onChange={setName}
            error={nameError}
          />
        )}
        {currentStep === 'pattern' && (
          <PatternStep
            patterns={patterns}
            selected={selectedPattern}
            onSelect={handlePatternSelect}
          />
        )}
        {currentStep === 'category' && (
          <CategoryStep
            categories={categories}
            selected={selectedCategory}
            onSelect={setSelectedCategory}
          />
        )}
        {currentStep === 'scaffold' && selectedPattern && (
          <ScaffoldStep
            dirs={selectedPattern.scaffoldDirs}
            selected={scaffoldDirs}
            onToggle={toggleScaffoldDir}
          />
        )}
        {currentStep === 'confirm' && (
          <ConfirmStep
            name={name}
            pattern={selectedPattern}
            category={selectedCategory}
            scaffoldDirs={scaffoldDirs}
          />
        )}
      </div>

      {/* Navigation buttons */}
      <div className="flex items-center justify-between">
        <Button variant="secondary" onClick={goBack}>
          <ArrowLeft size={16} strokeWidth={2.5} />
          Back
        </Button>
        {currentStep === 'confirm' ? (
          <Button
            variant="primary"
            onClick={handleCreate}
            loading={creating}
            disabled={!canAdvance}
          >
            {!creating && <Check size={16} strokeWidth={2.5} />}
            Create Skill
          </Button>
        ) : (
          <Button
            variant="primary"
            onClick={goNext}
            disabled={!canAdvance}
          >
            Next
            <ArrowRight size={16} strokeWidth={2.5} />
          </Button>
        )}
      </div>
    </div>
  );
}

/* -- Progress bar ------------------------------------ */

function ProgressBar({ current, steps }: { current: number; steps: StepId[] }) {
  const total = steps.length;
  const labels: Record<StepId, string> = {
    name: 'Name',
    pattern: 'Pattern',
    category: 'Category',
    scaffold: 'Scaffold',
    confirm: 'Confirm',
  };

  return (
    <div>
      {/* Step labels */}
      <div className="flex items-center justify-between mb-2">
        {steps.map((step, i) => (
          <span
            key={step}
            className={`text-sm font-medium ${
              i === current ? 'text-pencil' : i < current ? 'text-pencil-light' : 'text-muted-dark'
            }`}
          >
            {labels[step]}
          </span>
        ))}
      </div>
      {/* Bar */}
      <div className="w-full h-2 bg-muted rounded-full overflow-hidden">
        <div
          className="h-full bg-pencil rounded-full transition-all duration-300"
          style={{ width: `${((current + 1) / total) * 100}%` }}
        />
      </div>
      <p className="text-sm text-pencil-light mt-1">
        Step {current + 1} of {total}
      </p>
    </div>
  );
}

/* -- Step: Name -------------------------------------- */

function NameStep({
  value,
  onChange,
  error,
}: {
  value: string;
  onChange: (v: string) => void;
  error: string | null;
}) {
  return (
    <Card>
      <h3 className="text-lg font-bold text-pencil mb-1">Skill Name</h3>
      <p className="text-pencil-light text-sm mb-4">
        Choose a unique name for your skill. Use lowercase letters, numbers, hyphens, and underscores.
      </p>
      <Input
        type="text"
        placeholder="my-awesome-skill"
        value={value}
        onChange={(e) => onChange(e.target.value.toLowerCase())}
        autoFocus
      />
      {error && (
        <p className="text-danger text-sm mt-2">{error}</p>
      )}
      {value && !error && (
        <p className="text-success text-sm mt-2">Name is available</p>
      )}
    </Card>
  );
}

/* -- Step: Pattern ----------------------------------- */

function PatternStep({
  patterns,
  selected,
  onSelect,
}: {
  patterns: SkillPattern[];
  selected: SkillPattern | null;
  onSelect: (p: SkillPattern) => void;
}) {
  return (
    <div>
      <h3 className="text-lg font-bold text-pencil mb-1">Choose a Pattern</h3>
      <p className="text-pencil-light text-sm mb-4">
        Patterns provide different file structures optimized for various use cases.
      </p>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        {patterns.map((p) => (
          <button
            key={p.name}
            type="button"
            onClick={() => onSelect(p)}
            className={`
              ss-card text-left p-4 border-2 cursor-pointer transition-all duration-150
              rounded-[var(--radius-md)]
              ${selected?.name === p.name
                ? 'border-pencil shadow-md'
                : 'border-muted bg-surface hover:border-muted-dark hover:shadow-sm'
              }
            `}
          >
            <h4 className="font-bold text-pencil text-base mb-1 capitalize">{p.name}</h4>
            <p className="text-pencil-light text-sm leading-snug">{p.description}</p>
            {p.scaffoldDirs.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-2">
                {p.scaffoldDirs.map((d) => (
                  <span key={d} className="text-xs bg-muted text-pencil-light px-1.5 py-0.5 rounded-[var(--radius-sm)]">
                    {d}/
                  </span>
                ))}
              </div>
            )}
          </button>
        ))}
      </div>
    </div>
  );
}

/* -- Step: Category ---------------------------------- */

function CategoryStep({
  categories,
  selected,
  onSelect,
}: {
  categories: SkillCategory[];
  selected: SkillCategory | null;
  onSelect: (c: SkillCategory | null) => void;
}) {
  return (
    <div>
      <h3 className="text-lg font-bold text-pencil mb-1">Choose a Category</h3>
      <p className="text-pencil-light text-sm mb-4">
        Categories help organize your skill with a descriptive label. You can skip this step.
      </p>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        {/* Skip option */}
        <button
          type="button"
          onClick={() => onSelect(null)}
          className={`
            ss-card text-left p-4 border-2 cursor-pointer transition-all duration-150
            rounded-[var(--radius-md)]
            ${selected === null
              ? 'border-pencil shadow-md'
              : 'border-muted bg-surface hover:border-muted-dark hover:shadow-sm'
            }
          `}
        >
          <h4 className="font-bold text-pencil text-base mb-1">Skip</h4>
          <p className="text-pencil-light text-sm leading-snug">No category</p>
        </button>
        {categories.map((c) => (
          <button
            key={c.key}
            type="button"
            onClick={() => onSelect(c)}
            className={`
              ss-card text-left p-4 border-2 cursor-pointer transition-all duration-150
              rounded-[var(--radius-md)]
              ${selected?.key === c.key
                ? 'border-pencil shadow-md'
                : 'border-muted bg-surface hover:border-muted-dark hover:shadow-sm'
              }
            `}
          >
            <h4 className="font-bold text-pencil text-base mb-1">{c.label}</h4>
            <p className="text-pencil-light text-sm leading-snug">{c.key}</p>
          </button>
        ))}
      </div>
    </div>
  );
}

/* -- Step: Scaffold ---------------------------------- */

function ScaffoldStep({
  dirs,
  selected,
  onToggle,
}: {
  dirs: string[];
  selected: Set<string>;
  onToggle: (dir: string) => void;
}) {
  return (
    <div>
      <h3 className="text-lg font-bold text-pencil mb-1">Scaffold Directories</h3>
      <p className="text-pencil-light text-sm mb-4">
        Choose which directories to create. All are selected by default.
      </p>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        {dirs.map((dir) => {
          const isOn = selected.has(dir);
          return (
            <button
              key={dir}
              type="button"
              onClick={() => onToggle(dir)}
              className={`
                ss-card flex items-center gap-3 p-4 border-2 cursor-pointer transition-all duration-150
                rounded-[var(--radius-md)]
                ${isOn
                  ? 'border-pencil shadow-md'
                  : 'border-muted bg-surface hover:border-muted-dark hover:shadow-sm opacity-60'
                }
              `}
            >
              <FolderPlus size={20} strokeWidth={2} className={isOn ? 'text-pencil' : 'text-muted-dark'} />
              <span className={`font-mono text-sm ${isOn ? 'text-pencil font-medium' : 'text-pencil-light'}`}>
                {dir}/
              </span>
              {isOn && (
                <Check size={16} strokeWidth={3} className="text-pencil ml-auto" />
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}

/* -- Step: Confirm ----------------------------------- */

function ConfirmStep({
  name,
  pattern,
  category,
  scaffoldDirs,
}: {
  name: string;
  pattern: SkillPattern | null;
  category: SkillCategory | null;
  scaffoldDirs: Set<string>;
}) {
  return (
    <Card>
      <h3 className="text-lg font-bold text-pencil mb-4">Review &amp; Create</h3>
      <dl className="space-y-3">
        <div className="flex items-start gap-3">
          <dt className="text-pencil-light text-sm w-28 shrink-0">Name</dt>
          <dd className="font-mono font-bold text-pencil">{name}</dd>
        </div>
        <div className="flex items-start gap-3">
          <dt className="text-pencil-light text-sm w-28 shrink-0">Pattern</dt>
          <dd className="text-pencil capitalize">{pattern?.name ?? 'none'}</dd>
        </div>
        {category && (
          <div className="flex items-start gap-3">
            <dt className="text-pencil-light text-sm w-28 shrink-0">Category</dt>
            <dd className="text-pencil">{category.label}</dd>
          </div>
        )}
        {pattern && pattern.scaffoldDirs.length > 0 && (
          <div className="flex items-start gap-3">
            <dt className="text-pencil-light text-sm w-28 shrink-0">Directories</dt>
            <dd className="flex flex-wrap gap-1.5">
              {[...scaffoldDirs].map((dir) => (
                <span
                  key={dir}
                  className="text-sm bg-muted text-pencil px-2 py-0.5 rounded-[var(--radius-sm)]"
                >
                  <FolderPlus size={12} strokeWidth={2.5} className="inline mr-1" />
                  {dir}/
                </span>
              ))}
              {scaffoldDirs.size === 0 && (
                <span className="text-pencil-light text-sm">None</span>
              )}
            </dd>
          </div>
        )}
      </dl>
    </Card>
  );
}
