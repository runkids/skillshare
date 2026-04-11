import { useState, type ReactNode } from 'react';
import { Link, Navigate, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Trash2 } from 'lucide-react';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api, type ManagedHookDetailResponse, type ManagedHookSaveRequest } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import HookEditor from '../components/HookEditor';
import { createEmptyHookEditorValue, type HookEditorValue } from '../components/hookEditorState';
import CompiledPreviewCard from '../components/CompiledPreviewCard';

function emptyValue(): HookEditorValue {
  return createEmptyHookEditorValue();
}

function toRequest(value: HookEditorValue): ManagedHookSaveRequest {
  return {
    tool: value.tool,
    event: value.event,
    matcher: value.matcher || undefined,
    handlers: value.handlers.map((handler) => ({
      type: handler.type,
      command: handler.command || undefined,
      url: handler.url || undefined,
      prompt: handler.prompt || undefined,
      timeout: handler.timeout || undefined,
      timeoutSec: handler.timeoutSec ? Number(handler.timeoutSec) : undefined,
      statusMessage: handler.statusMessage || undefined,
    })),
  };
}

function fromDetail(detail: ManagedHookDetailResponse): HookEditorValue {
  return {
    tool: detail.hook.tool,
    event: detail.hook.event,
    matcher: detail.hook.matcher ?? '',
    handlers: detail.hook.handlers.length > 0
      ? detail.hook.handlers.map((handler) => ({
          type: handler.type,
          command: handler.command ?? '',
          url: handler.url ?? '',
          prompt: handler.prompt ?? '',
          timeout: handler.timeout ?? '',
          timeoutSec: handler.timeoutSec === undefined ? '' : String(handler.timeoutSec),
          statusMessage: handler.statusMessage ?? '',
        }))
      : emptyValue().handlers,
  };
}

function HookDetailEditorSection({
  initialValue,
  onSave,
  saving,
  status,
  deleteAction,
}: {
  initialValue: HookEditorValue;
  onSave: (value: HookEditorValue) => void;
  saving: boolean;
  status: string | null;
  deleteAction: ReactNode;
}) {
  const [value, setValue] = useState(initialValue);

  return (
    <HookEditor
      value={value}
      onChange={setValue}
      onSave={() => onSave(value)}
      saving={saving}
      status={status}
      submitLabel="Save Hook"
      deleteAction={deleteAction}
    />
  );
}

export default function HookDetailPage() {
  const location = useLocation();

  return <HookDetailPageContent key={location.pathname} pathname={location.pathname} />;
}

function HookDetailPageContent({ pathname }: { pathname: string }) {
  const params = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const routeManagedId = params['*'] ?? '';
  const isCreateRoute = pathname.endsWith('/new');
  const shouldRedirectToList = !isCreateRoute && !routeManagedId;
  const [savedDetail, setSavedDetail] = useState<ManagedHookDetailResponse | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const detailQuery = useQuery({
    queryKey: queryKeys.hooks.detail(routeManagedId),
    queryFn: () => api.managedHooks.get(routeManagedId),
    initialData: () =>
      queryClient.getQueryData(queryKeys.hooks.detail(routeManagedId)) as ManagedHookDetailResponse | undefined,
    staleTime: staleTimes.hooks,
    enabled: !isCreateRoute && !!routeManagedId,
  });

  const detail = savedDetail ?? detailQuery.data ?? null;
  const managedId = detail?.hook.id ?? routeManagedId;
  const isNew = !managedId;
  const initialValue = detail ? fromDetail(detail) : emptyValue();
  const editorKey = detail
    ? `${detail.hook.id}:${detail.hook.handlers.map((handler) => `${handler.type}:${handler.command ?? handler.url ?? handler.prompt ?? ''}`).join('|')}`
    : 'new';
  const previews = detail?.previews ?? [];

  const saveMutation = useMutation({
    mutationFn: (body: ManagedHookSaveRequest) =>
      isNew ? api.managedHooks.create(body) : api.managedHooks.update(managedId, body),
    onSuccess: (next) => {
      const nextManagedId = next.hook.id;
      setSavedDetail(next);
      setStatus('Saved hook.');
      toast('Hook saved.', 'success');
      queryClient.setQueryData(queryKeys.hooks.detail(nextManagedId), next);
      queryClient.invalidateQueries({ queryKey: queryKeys.hooks.managed, exact: true });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      if (isCreateRoute || nextManagedId !== managedId) {
        navigate(`/hooks/manage/${nextManagedId}`, { replace: true });
      }
    },
    onError: (error: Error) => {
      toast(error.message, 'error');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.managedHooks.remove(managedId),
    onSuccess: () => {
      toast('Hook deleted.', 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.hooks.managed, exact: true });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      navigate('/hooks');
    },
    onError: (error: Error) => {
      toast(error.message, 'error');
    },
  });

  if (shouldRedirectToList) {
    return <Navigate to="/hooks" replace />;
  }

  if (!isCreateRoute && detailQuery.isPending && !detail) {
    return <PageSkeleton />;
  }

  if (!isCreateRoute && detailQuery.error && !detail) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load hook
        </p>
        <p className="text-pencil-light text-sm mt-1">{detailQuery.error.message}</p>
      </Card>
    );
  }

  const deleteButton = !isNew ? (
    <HandButton
      type="button"
      variant="danger"
      size="sm"
      onClick={() => setConfirmDelete(true)}
    >
      <Trash2 size={16} strokeWidth={2.5} />
      Delete Hook
    </HandButton>
  ) : null;

  const previewSection = previews.length > 0 ? (
    <div className="space-y-4">
      <h3 className="text-2xl text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
        Compiled preview
      </h3>
      <div className="space-y-4">
        {previews.map((preview) => (
          <CompiledPreviewCard key={preview.target} preview={preview} />
        ))}
      </div>
    </div>
  ) : null;

  return (
    <div className="animate-sketch-in space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Link to="/hooks" className="mb-2 inline-flex items-center gap-2 text-sm text-pencil-light hover:text-pencil">
            <ArrowLeft size={14} strokeWidth={2.5} />
            Back to Hooks
          </Link>
          <h2 className="text-3xl font-bold text-pencil md:text-4xl" style={{ fontFamily: 'var(--font-heading)' }}>
            {isNew ? 'New Hook' : detail?.hook.id ?? 'Manage Hook'}
          </h2>
          <p className="text-pencil-light">
            {isNew ? 'Create a hook' : detail?.hook.tool ?? managedId}
          </p>
        </div>
      </div>

      <HookDetailEditorSection
        key={editorKey}
        initialValue={initialValue}
        onSave={(value) => saveMutation.mutate(toRequest(value))}
        saving={saveMutation.isPending}
        status={status}
        deleteAction={deleteButton}
      />

      {previewSection}

      <ConfirmDialog
        open={confirmDelete}
        onCancel={() => setConfirmDelete(false)}
        onConfirm={() => deleteMutation.mutate()}
        title="Delete hook?"
        message={`This will remove ${managedId}.`}
        confirmText="Delete Hook"
        variant="danger"
        loading={deleteMutation.isPending}
      />
    </div>
  );
}
