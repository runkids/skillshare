import { useMemo } from 'react';
import { Link, Navigate, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, FileText } from 'lucide-react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import Badge from '../components/Badge';
import Card from '../components/Card';
import CopyButton from '../components/CopyButton';
import HandButton from '../components/HandButton';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api, type RuleItem, type RulesListResponse } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { wobbly } from '../design';

function getRuleRef(rule: RuleItem): string {
  return rule.id ?? rule.path;
}

export default function DiscoveredRuleDetailPage() {
  const { ruleRef = '' } = useParams<{ ruleRef: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const ruleFromState = (location.state as { rule?: RuleItem } | null)?.rule;

  const discoveredQuery = useQuery({
    queryKey: queryKeys.rules.discovered,
    queryFn: () => api.listRules(),
    initialData: () => queryClient.getQueryData(queryKeys.rules.discovered) as RulesListResponse | undefined,
    staleTime: staleTimes.rules,
  });

  const rule = useMemo(() => {
    if (ruleFromState && getRuleRef(ruleFromState) === ruleRef) {
      return ruleFromState;
    }
    return discoveredQuery.data?.rules.find((item) => getRuleRef(item) === ruleRef) ?? null;
  }, [discoveredQuery.data, ruleFromState, ruleRef]);

  const collectMutation = useMutation({
    mutationFn: async () => {
      if (!rule?.id) {
        throw new Error('This discovered rule cannot be collected.');
      }
      return api.managedRules.collect({
        ids: [rule.id],
        strategy: 'overwrite',
      });
    },
    onSuccess: (result) => {
      toast('Rule collected.', 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.rules.managed, exact: true });
      queryClient.invalidateQueries({ queryKey: queryKeys.rules.discovered });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      const managedId = result.created[0] ?? result.overwritten[0];
      if (managedId) {
        navigate(`/rules/manage/${managedId}`);
      }
    },
    onError: (error: Error) => {
      toast(error.message, 'error');
    },
  });

  const markdownComponents: Components = {
    a: ({ children }) => (
      <span className="underline decoration-muted-dark underline-offset-2">
        {children}
      </span>
    ),
    img: ({ alt }) => (
      <span className="italic text-pencil-light">
        {alt ? `[image: ${alt}]` : '[image]'}
      </span>
    ),
  };

  if (discoveredQuery.isPending && !rule) {
    return <PageSkeleton />;
  }

  if (discoveredQuery.error && !rule) {
    return (
      <Card variant="accent" className="py-8 text-center">
        <p className="text-lg text-danger" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load discovered rule
        </p>
        <p className="mt-1 text-sm text-pencil-light">{discoveredQuery.error.message}</p>
      </Card>
    );
  }

  if (!rule) {
    return <Navigate to="/rules?mode=discovered" replace />;
  }

  return (
    <div className="animate-sketch-in space-y-6">
      <div className="sticky top-0 z-20 -mx-4 -mt-3 bg-paper px-4 py-3 md:-mx-8 md:px-8">
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate('/rules?mode=discovered')}
            className="flex h-9 w-9 items-center justify-center border-2 border-pencil bg-surface text-pencil-light transition-colors hover:text-pencil"
            style={{ borderRadius: wobbly.sm }}
            aria-label="Back to Rules"
          >
            <ArrowLeft size={18} strokeWidth={2.5} />
          </button>
          <div className="min-w-0">
            <Link to="/rules?mode=discovered" className="text-sm text-pencil-light hover:text-pencil">
              Back to Rules
            </Link>
            <div className="mt-1 flex flex-wrap items-center gap-2">
              <h2 className="text-2xl font-bold text-pencil md:text-3xl" style={{ fontFamily: 'var(--font-heading)' }}>
                {rule.name}
              </h2>
              <Badge variant="accent">{rule.sourceTool}</Badge>
              <Badge variant={rule.scope === 'project' ? 'info' : 'default'}>{rule.scope}</Badge>
              {rule.collectible && <Badge variant="success">Collectible</Badge>}
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <Card decoration="tape">
            <div className="mb-4 flex flex-wrap items-center gap-4 border-b-2 border-dashed border-pencil-light/30 py-3 text-sm text-pencil-light">
              <span>{(rule.stats?.wordCount ?? 0).toLocaleString()} words</span>
              <span>{(rule.stats?.lineCount ?? 0).toLocaleString()} lines</span>
              <span>{(rule.stats?.tokenCount ?? 0).toLocaleString()} tokens</span>
            </div>
            <div className="prose-hand">
              <Markdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                {rule.content}
              </Markdown>
            </div>
          </Card>
        </div>

        <div className="space-y-5 lg:sticky lg:top-16 lg:self-start">
          <Card variant="postit">
            <h3 className="mb-3 font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
              Metadata
            </h3>
            <dl className="space-y-3">
              <div>
                <dt className="text-sm uppercase tracking-wider text-pencil-light">Path</dt>
                <dd className="mt-1 flex items-center gap-1.5">
                  <span className="min-w-0 truncate text-base text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                    {rule.path}
                  </span>
                  <CopyButton
                    value={rule.path}
                    title="Copy rule path"
                    copiedLabelClassName="text-xs font-normal"
                  />
                </dd>
              </div>
              <div>
                <dt className="text-sm uppercase tracking-wider text-pencil-light">Source Tool</dt>
                <dd className="text-base text-pencil">{rule.sourceTool}</dd>
              </div>
              <div>
                <dt className="text-sm uppercase tracking-wider text-pencil-light">Scope</dt>
                <dd className="text-base text-pencil">{rule.scope}</dd>
              </div>
              <div>
                <dt className="text-sm uppercase tracking-wider text-pencil-light">File Size</dt>
                <dd className="text-base text-pencil">{rule.size.toLocaleString()} bytes</dd>
              </div>
              {rule.collectReason && (
                <div>
                  <dt className="text-sm uppercase tracking-wider text-pencil-light">Collection Notes</dt>
                  <dd className="text-base text-pencil">{rule.collectReason}</dd>
                </div>
              )}
            </dl>

            {rule.collectible && rule.id ? (
              <div className="mt-4 border-t-2 border-dashed border-pencil-light/30 pt-4">
                <HandButton
                  variant="secondary"
                  size="sm"
                  disabled={collectMutation.isPending}
                  onClick={() => collectMutation.mutate()}
                  className="w-full"
                >
                  {collectMutation.isPending ? 'Collecting...' : 'Collect & Edit'}
                </HandButton>
              </div>
            ) : null}
          </Card>

          <Card>
            <h3
              className="mb-3 flex items-center gap-2 font-bold text-pencil"
              style={{ fontFamily: 'var(--font-heading)' }}
            >
              <FileText size={16} strokeWidth={2.5} />
              Source Snapshot
            </h3>
            <pre className="overflow-x-auto whitespace-pre-wrap break-words rounded-md bg-surface px-3 py-2 text-sm text-pencil">
              {rule.path}
            </pre>
          </Card>
        </div>
      </div>
    </div>
  );
}
