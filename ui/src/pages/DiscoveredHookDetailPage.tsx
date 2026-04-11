import { useMemo } from 'react';
import { Link, Navigate, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, FileText } from 'lucide-react';
import Badge from '../components/Badge';
import Card from '../components/Card';
import CopyButton from '../components/CopyButton';
import HandButton from '../components/HandButton';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api, type HooksListResponse } from '../api/client';
import {
  formatHookDiscoveryGroupTitle,
  getHookActionPayload,
  groupDiscoveredHooks,
  type HookDiscoveryGroup,
} from '../lib/hookDiscovery';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { wobbly } from '../design';

function getGroupRef(group: HookDiscoveryGroup): string {
  return group.id;
}

export default function DiscoveredHookDetailPage() {
  const { groupRef = '' } = useParams<{ groupRef: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const groupFromState = (location.state as { group?: HookDiscoveryGroup } | null)?.group;

  const discoveredQuery = useQuery({
    queryKey: queryKeys.hooks.discovered,
    queryFn: () => api.listHooks(),
    initialData: () => queryClient.getQueryData(queryKeys.hooks.discovered) as HooksListResponse | undefined,
    staleTime: staleTimes.hooks,
  });

  const group = useMemo(() => {
    if (groupFromState && getGroupRef(groupFromState) === groupRef) {
      return groupFromState;
    }
    const groups = groupDiscoveredHooks(discoveredQuery.data?.hooks ?? []);
    return groups.find((item) => getGroupRef(item) === groupRef) ?? null;
  }, [discoveredQuery.data, groupFromState, groupRef]);

  const collectMutation = useMutation({
    mutationFn: async () => {
      if (!group?.collectible) {
        throw new Error('This discovered hook group cannot be collected.');
      }
      return api.managedHooks.collect({
        groupIds: [group.id],
        strategy: 'overwrite',
      });
    },
    onSuccess: (result) => {
      toast('Hook collected.', 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.hooks.managed, exact: true });
      queryClient.invalidateQueries({ queryKey: queryKeys.hooks.discovered });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      const managedId = result.created[0] ?? result.overwritten[0];
      if (managedId) {
        navigate(`/hooks/manage/${managedId}`);
      }
    },
    onError: (error: Error) => {
      toast(error.message, 'error');
    },
  });

  const sourcePaths = useMemo(() => {
    return Array.from(new Set(group?.hooks.map((hook) => hook.path) ?? []));
  }, [group]);

  if (discoveredQuery.isPending && !group) {
    return <PageSkeleton />;
  }

  if (discoveredQuery.error && !group) {
    return (
      <Card variant="accent" className="py-8 text-center">
        <p className="text-lg text-danger" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load discovered hook
        </p>
        <p className="mt-1 text-sm text-pencil-light">{discoveredQuery.error.message}</p>
      </Card>
    );
  }

  if (!group) {
    return <Navigate to="/hooks?mode=discovered" replace />;
  }

  return (
    <div className="animate-sketch-in space-y-6">
      <div className="sticky top-0 z-20 -mx-4 -mt-3 bg-paper px-4 py-3 md:-mx-8 md:px-8">
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate('/hooks?mode=discovered')}
            className="flex h-9 w-9 items-center justify-center border-2 border-pencil bg-surface text-pencil-light transition-colors hover:text-pencil"
            style={{ borderRadius: wobbly.sm }}
            aria-label="Back to Hooks"
          >
            <ArrowLeft size={18} strokeWidth={2.5} />
          </button>
          <div className="min-w-0">
            <Link to="/hooks?mode=discovered" className="text-sm text-pencil-light hover:text-pencil">
              Back to Hooks
            </Link>
            <div className="mt-1 flex flex-wrap items-center gap-2">
              <h2 className="text-2xl font-bold text-pencil md:text-3xl" style={{ fontFamily: 'var(--font-heading)' }}>
                {formatHookDiscoveryGroupTitle(group)}
              </h2>
              <Badge variant="accent">{group.sourceTool}</Badge>
              <Badge variant={group.scope === 'project' ? 'info' : 'default'}>{group.scope}</Badge>
              {group.collectible && <Badge variant="success">Collectible</Badge>}
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <Card decoration="tape">
            <div className="mb-4 flex flex-wrap items-center gap-4 border-b-2 border-dashed border-pencil-light/30 py-3 text-sm text-pencil-light">
              <span>{group.hooks.length.toLocaleString()} action{group.hooks.length !== 1 ? 's' : ''}</span>
              <span>{sourcePaths.length.toLocaleString()} file{sourcePaths.length !== 1 ? 's' : ''}</span>
            </div>

            <div className="space-y-4">
              {group.hooks.map((hook, index) => (
                <div
                  key={`${group.id}:${index}`}
                  className="space-y-3 border-b-2 border-dashed border-pencil-light/20 pb-4 last:border-b-0 last:pb-0"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="default">{hook.actionType}</Badge>
                    {hook.timeout && <Badge variant="warning">{hook.timeout}</Badge>}
                    {hook.timeoutSec !== undefined && <Badge variant="info">{hook.timeoutSec}s</Badge>}
                  </div>
                  <p className="text-xs uppercase tracking-[0.2em] text-pencil-light">Action payload</p>
                  <pre className="overflow-x-auto whitespace-pre-wrap break-words rounded-md bg-surface px-3 py-2 text-sm text-pencil">
                    {getHookActionPayload(hook)}
                  </pre>
                  {hook.statusMessage && <p className="text-sm text-pencil-light">{hook.statusMessage}</p>}
                  <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                    {hook.path}
                  </p>
                </div>
              ))}
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
                <dt className="text-sm uppercase tracking-wider text-pencil-light">Group Id</dt>
                <dd className="mt-1 flex items-center gap-1.5">
                  <span className="min-w-0 truncate text-base text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                    {group.id}
                  </span>
                  <CopyButton
                    value={group.id}
                    title="Copy hook group id"
                    copiedLabelClassName="text-xs font-normal"
                  />
                </dd>
              </div>
              <div>
                <dt className="text-sm uppercase tracking-wider text-pencil-light">Event</dt>
                <dd className="text-base text-pencil">{group.event}</dd>
              </div>
              <div>
                <dt className="text-sm uppercase tracking-wider text-pencil-light">Matcher</dt>
                <dd className="text-base text-pencil">{group.matcher}</dd>
              </div>
              <div>
                <dt className="text-sm uppercase tracking-wider text-pencil-light">Hook Actions</dt>
                <dd className="text-base text-pencil">{group.hooks.length}</dd>
              </div>
              {group.collectReason && (
                <div>
                  <dt className="text-sm uppercase tracking-wider text-pencil-light">Collection Notes</dt>
                  <dd className="text-base text-pencil">{group.collectReason}</dd>
                </div>
              )}
            </dl>

            {group.collectible ? (
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
              Source Files
            </h3>
            <div className="space-y-3">
              {sourcePaths.map((path) => (
                <div key={path} className="rounded-md bg-surface px-3 py-2">
                  <p className="break-all text-sm text-pencil" style={{ fontFamily: "'Courier New', monospace" }}>
                    {path}
                  </p>
                </div>
              ))}
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}
