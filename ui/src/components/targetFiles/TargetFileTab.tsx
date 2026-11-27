import { useRef, useState } from 'react';
import { Link, useBeforeUnload, useBlocker, useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, ChevronDown, ChevronUp, FileText, Link2 } from 'lucide-react';
import { api } from '../../api/client';
import type { TargetFile } from '../../api/client';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import CodeEditor from '../CodeEditor';
import ConfirmDialog from '../ConfirmDialog';
import { PageSkeleton } from '../Skeleton';
import { useToast } from '../Toast';
import { targetLabel } from '../mcp/mcpView';
import { useFillHeight } from '../instructions/useFillHeight';
import { useSaveShortcut } from '../instructions/useSaveShortcut';
import { BoxHeader, InstructionsPreview } from '../instructions/ViewTabs';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import { fileName, shortenHome } from '../../lib/paths';
import { invalidate } from '../../lib/queryEvents';

// The page's bottom padding, and the least height the tab keeps on a short window (as the instructions tab).
const PAGE_BOTTOM = 40;
const MIN_TAB = 480;

type Content = TargetFile & { content: string };

/** The folder holding a file, keeping the separator it uses. */
const dirOf = (abs: string) => abs.replace(/[\\/][^\\/]*$/, '');
const isMarkdown = (path: string) => /\.(md|markdown|mdx)$/i.test(path);

/** A file tab on the target page: the file this target's tool reads, editable in place. */
export default function TargetFileTab({
  target,
  path,
  project,
}: {
  target: string;
  path: string;
  project: boolean;
}) {
  const { data, error, isPending } = useQuery({
    queryKey: queryKeys.targetFiles.content(target, path),
    queryFn: () => api.getTargetFile(target, path),
  });
  const [fillRef, height] = useFillHeight<HTMLDivElement>(PAGE_BOTTOM, MIN_TAB);
  const [expanded, setExpanded] = useState(false);
  return (
    <div ref={fillRef} style={{ height: expanded ? undefined : height }} className="flex flex-col">
      {isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad">
          <span className="flex-1">{error.message}</span>
        </div>
      ) : (
        // Remount on a new file version so the draft starts from it.
        <Editor
          key={`${data.abs}:${data.content}`}
          target={target}
          data={data}
          project={project}
          expanded={expanded}
          setExpanded={setExpanded}
        />
      )}
    </div>
  );
}

function Editor({
  target,
  data,
  project,
  expanded,
  setExpanded,
}: {
  target: string;
  data: Content;
  project: boolean;
  expanded: boolean;
  setExpanded: (on: boolean) => void;
}) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [draft, setDraft] = useState(data.content);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [busy, setBusy] = useState(false);
  const file = fileName(data.path);
  const markdown = isMarkdown(data.path);
  const [view, setView] = useState<'edit' | 'preview'>(
    markdown && data.exists && data.content.trim() ? 'preview' : 'edit',
  );
  const tool = targetLabel(target);

  // Leaving with an unsaved edit asks first: other tabs, the sidebar, a reload.
  const dirty = draft !== data.content;
  // Set once the tab is removed: the user already confirmed, so the edit is let go.
  const leaving = useRef(false);
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirty &&
      !leaving.current &&
      (currentLocation.pathname !== nextLocation.pathname ||
        currentLocation.search !== nextLocation.search),
  );
  useBeforeUnload((e) => {
    if (dirty) {
      e.preventDefault();
      e.returnValue = '';
    }
  });

  const save = async () => {
    setSaving(true);
    try {
      await api.putTargetFile(target, data.path, draft);
      void invalidate(queryClient, 'targetFilesChanged', target);
      toast(t('instructions.saved', { path: shortenHome(data.abs) }), 'success');
    } catch (err) {
      toast((err as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };
  useSaveShortcut(() => {
    if (!saving && draft !== data.content) void save();
  });

  const remove = async () => {
    setBusy(true);
    try {
      await api.removeTargetFile(target, data.path);
      setRemoving(false);
      toast(t('targetFiles.removed', { file: data.path }), 'success');
      leaving.current = true;
      void navigate('?tab=instructions', { replace: true });
      // Only the list: the removed file's content query is still mounted and would fetch again.
      await invalidate(queryClient, 'targetFileRemoved', target);
    } catch (err) {
      toast((err as Error).message, 'error');
      setBusy(false);
    }
  };

  const explanation =
    file === 'APPEND_SYSTEM.md'
      ? !project && (target === 'pi' || target === 'omp')
        ? t('targetFiles.appendSystemGlobal', { tool, path: `.${target}/APPEND_SYSTEM.md` })
        : t('targetFiles.appendSystem', { tool })
      : !data.exists
        ? t('targetFiles.notCreated')
        : data.link_shared
          ? t('targetFiles.linked', {
              path: shortenHome(data.link_to ?? ''),
              name: data.link_shared,
            })
          : '';
  const share = `/extras?add=file&target=${encodeURIComponent(shortenHome(dirOf(data.abs)))}&file=${encodeURIComponent(file)}`;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <div className="flex flex-col gap-1.5 rounded-lg border border-line bg-surface px-4 py-3">
        <div className="flex min-h-8 flex-wrap items-center gap-2.5">
          <FileText size={15} className="shrink-0 text-ink-3" />
          <span className="inline-flex items-center gap-1.5 font-semibold">
            <AgentIcon target={target} size={15} />
            {target}
          </span>
          {!data.exists && <span className="ss-tag">{t('targetFiles.chip.notCreated')}</span>}
          {!data.builtin && (
            <span className="ss-tag !border-dashed">{t('targetFiles.chip.userAdded')}</span>
          )}
          {data.link_shared && (
            <span className="ss-tag ok">
              {t('targetFiles.chip.shared', { name: data.link_shared })}
            </span>
          )}
          <span className="flex-1" />
          {data.link_shared ? (
            <Link to="/extras" className="ss-btn ghost sm">
              {t('targetFiles.viewInExtras')}
              <ArrowRight size={13} />
            </Link>
          ) : (
            <>
              {!data.builtin && (
                <button type="button" className="ss-btn ghost sm" onClick={() => setRemoving(true)}>
                  {t('targetFiles.removeTab')}
                </button>
              )}
              <Link to={share} className="ss-btn sm">
                <Link2 size={13} />
                {t('targetFiles.shareWithExtras')}
              </Link>
            </>
          )}
        </div>
        {explanation && <p className="pl-[25px] text-[12.5px] text-ink-3">{explanation}</p>}
      </div>

      <div className="ss-code flex min-h-[360px] flex-1 flex-col !bg-surface !overflow-hidden !p-0 !whitespace-normal">
        <BoxHeader
          content={draft}
          view={view}
          onChange={setView}
          views={
            markdown ? undefined : [{ value: 'edit', label: t('instructions.target.view.edit') }]
          }
        >
          <Button
            variant="ghost"
            size="sm"
            aria-expanded={expanded}
            onClick={() => setExpanded(!expanded)}
          >
            {t(expanded ? 'instructions.preview.collapse' : 'instructions.preview.expand')}
            {expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          </Button>
          {/* Only once there is something to save; Cmd+S works either way. */}
          {dirty && (
            <Button variant="primary" size="sm" onClick={save} loading={saving}>
              {t('common.save')}
            </Button>
          )}
        </BoxHeader>
        {view === 'edit' ? (
          <CodeEditor
            value={draft}
            onChange={setDraft}
            ariaLabel={file}
            disabled={saving}
            wrap
            fill={!expanded}
            minHeight="360px"
            maxHeight="none"
            className="min-h-0 flex-1 !rounded-none !border-0 !bg-surface"
            placeholder={t('targetFiles.placeholder', { file })}
          />
        ) : (
          <InstructionsPreview content={draft} names={[]} className={expanded ? '' : undefined} />
        )}
      </div>

      <ConfirmDialog
        open={removing}
        title={t('targetFiles.remove.title', { file: data.path })}
        message={t('targetFiles.remove.message', { path: shortenHome(data.abs), tool })}
        confirmText={t('targetFiles.remove.confirm')}
        loading={busy}
        onConfirm={() => void remove()}
        onCancel={() => setRemoving(false)}
      />
      <ConfirmDialog
        open={blocker.state === 'blocked'}
        title={t('config.discard.title')}
        message={t('config.discard.message')}
        confirmText={t('config.discard.confirmText')}
        variant="danger"
        onConfirm={() => blocker.state === 'blocked' && blocker.proceed()}
        onCancel={() => blocker.state === 'blocked' && blocker.reset()}
      />
    </div>
  );
}
