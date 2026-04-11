import { useState, type ReactNode } from 'react';
import { Link, Navigate, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Trash2 } from 'lucide-react';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api, type ManagedRuleDetailResponse, type ManagedRuleSaveRequest } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import RuleEditor, { type RuleEditorValue } from '../components/RuleEditor';
import CompiledPreviewCard from '../components/CompiledPreviewCard';

function emptyValue(): RuleEditorValue {
  return { tool: '', relativePath: '', content: '' };
}

function RuleDetailEditorSection({
  initialValue,
  onSave,
  saving,
  status,
  deleteAction,
}: {
  initialValue: RuleEditorValue;
  onSave: (value: RuleEditorValue) => void;
  saving: boolean;
  status: string | null;
  deleteAction: ReactNode;
}) {
  const [value, setValue] = useState(initialValue);

  return (
    <RuleEditor
      value={value}
      onChange={setValue}
      onSave={() => onSave(value)}
      saving={saving}
      status={status}
      submitLabel="Save Rule"
      deleteAction={deleteAction}
    />
  );
}

export default function RuleDetailPage() {
  const location = useLocation();

  return <RuleDetailPageContent key={location.pathname} pathname={location.pathname} />;
}

function RuleDetailPageContent({ pathname }: { pathname: string }) {
  const params = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const routeManagedId = params['*'] ?? '';
  const isCreateRoute = pathname.endsWith('/new');
  const shouldRedirectToList = !isCreateRoute && !routeManagedId;
  const [savedDetail, setSavedDetail] = useState<ManagedRuleDetailResponse | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const detailQuery = useQuery({
    queryKey: queryKeys.rules.detail(routeManagedId),
    queryFn: () => api.managedRules.get(routeManagedId),
    initialData: () =>
      queryClient.getQueryData(queryKeys.rules.detail(routeManagedId)) as ManagedRuleDetailResponse | undefined,
    staleTime: staleTimes.rules,
    enabled: !isCreateRoute && !!routeManagedId,
  });

  const detail = savedDetail ?? detailQuery.data ?? null;
  const managedId = detail?.rule.id ?? routeManagedId;
  const isNew = !managedId;
  const initialValue = detail
    ? {
        tool: detail.rule.tool,
        relativePath: detail.rule.relativePath,
        content: detail.rule.content,
      }
    : emptyValue();
  const editorKey = detail ? `${detail.rule.id}:${detail.rule.content}` : 'new';
  const previews = detail?.previews ?? [];

  const saveMutation = useMutation({
    mutationFn: (body: ManagedRuleSaveRequest) =>
      isNew ? api.managedRules.create(body) : api.managedRules.update(managedId, body),
    onSuccess: (next) => {
      const nextManagedId = next.rule.id;
      setSavedDetail(next);
      setStatus('Saved rule.');
      toast('Rule saved.', 'success');
      queryClient.setQueryData(queryKeys.rules.detail(nextManagedId), next);
      queryClient.invalidateQueries({ queryKey: queryKeys.rules.managed, exact: true });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      if (isCreateRoute || nextManagedId !== managedId) {
        navigate(`/rules/manage/${nextManagedId}`, { replace: true });
      }
    },
    onError: (error: Error) => {
      toast(error.message, 'error');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.managedRules.remove(managedId),
    onSuccess: () => {
      toast('Rule deleted.', 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.rules.managed, exact: true });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      navigate('/rules');
    },
    onError: (error: Error) => {
      toast(error.message, 'error');
    },
  });

  if (shouldRedirectToList) {
    return <Navigate to="/rules" replace />;
  }

  if (!isCreateRoute && detailQuery.isPending && !detail) {
    return <PageSkeleton />;
  }

  if (!isCreateRoute && detailQuery.error && !detail) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load rule
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
      Delete Rule
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
          <Link to="/rules" className="mb-2 inline-flex items-center gap-2 text-sm text-pencil-light hover:text-pencil">
            <ArrowLeft size={14} strokeWidth={2.5} />
            Back to Rules
          </Link>
          <h2 className="text-3xl font-bold text-pencil md:text-4xl" style={{ fontFamily: 'var(--font-heading)' }}>
            {isNew ? 'New Rule' : detail?.rule.name ?? 'Manage Rule'}
          </h2>
          <p className="text-pencil-light">
            {isNew ? 'Create a rule' : detail?.rule.relativePath ?? managedId}
          </p>
        </div>
      </div>

      <RuleDetailEditorSection
        key={editorKey}
        initialValue={initialValue}
        onSave={(value) => saveMutation.mutate(value)}
        saving={saveMutation.isPending}
        status={status}
        deleteAction={deleteButton}
      />

      {previewSection}

      <ConfirmDialog
        open={confirmDelete}
        onCancel={() => setConfirmDelete(false)}
        onConfirm={() => deleteMutation.mutate()}
        title="Delete rule?"
        message={`This will remove ${initialValue.relativePath || managedId}.`}
        confirmText="Delete Rule"
        variant="danger"
        loading={deleteMutation.isPending}
      />
    </div>
  );
}
