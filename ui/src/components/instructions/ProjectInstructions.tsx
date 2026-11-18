import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { FileText, Info } from 'lucide-react';
import { api } from '../../api/client';
import type { ProjectInstructionsReach } from '../../api/client';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { PageSkeleton } from '../Skeleton';
import { useToast } from '../Toast';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import InstructionsEditorDialog from './InstructionsEditorDialog';
import ProjectSharedFiles from './ProjectSharedFiles';
import { instructionsErrorMessage, formatSize } from './instructionsView';
import { invalidate } from '../../lib/queryEvents';

/** ⑤ Extras › Instructions (project): one ./AGENTS.md, whether each target reads it, and the project's shared files. */
export default function ProjectInstructions({
  creating,
  setCreating,
}: {
  creating: boolean;
  setCreating: (open: boolean) => void;
}) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const { data, error, isPending } = useQuery({
    queryKey: queryKeys.instructions.project,
    queryFn: () => api.getProjectInstructions(),
  });
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);

  if (isPending) return <PageSkeleton />;
  if (error)
    return (
      <div className="ss-note bad">
        <span className="flex-1">{instructionsErrorMessage(error, t)}</span>
      </div>
    );

  const shim = async (r: ProjectInstructionsReach) => {
    setBusy(r.target);
    try {
      await api.addProjectInstructionsShim(r.target);
      toast(t('instructions.project.shimDone', { name: r.target, file: r.file }), 'success');
      void invalidate(queryClient, 'instructionsChanged');
    } catch (err) {
      toast(instructionsErrorMessage(err, t), 'error');
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="flex flex-col gap-7">
      <div className="ss-box flex items-center gap-4">
        <span className="ss-cat extra">
          <FileText size={16} />
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="flex items-center gap-2">
            <span className="font-mono font-semibold">AGENTS.md</span>
            <span className="ss-tag">project</span>
          </span>
          <span className="font-mono text-[12.5px] text-ink-3">
            {data.exists
              ? `./AGENTS.md · ${formatSize(data.size)} · ${t('instructions.project.tracked')}`
              : t('instructions.project.notCreated')}
          </span>
        </span>
        <Button variant="secondary" onClick={() => setEditing(true)}>
          {t(data.exists ? 'instructions.edit' : 'instructions.project.create')}
        </Button>
      </div>

      <section className="flex flex-col gap-2.5">
        <div className="ss-sec">
          <h2>{t('instructions.project.readsTitle')}</h2>
          <span className="text-[13px] text-ink-3">{t('instructions.project.readsHint')}</span>
        </div>
        <div className="ss-list">
          <div className="ss-lh">
            <span className="w-[30px]" />
            <span className="w-[120px]">{t('instructions.shared.colTarget')}</span>
            <span className="flex-1">{t('instructions.project.colHow')}</span>
            <span className="w-[110px]">{t('instructions.project.colStatus')}</span>
            <span className="w-[170px]" />
          </div>
          {data.targets.length === 0 && (
            <div className="ss-r text-[13px] text-ink-3">{t('instructions.project.noTargets')}</div>
          )}
          {data.targets.map((r) => (
            <div key={r.target} className="ss-r !min-h-[52px]">
              <span className="ss-at">
                <AgentIcon target={r.target} size={17} />
              </span>
              <span className="w-[120px] shrink-0 truncate font-mono text-[13px] font-semibold">
                {r.target}
              </span>
              <span className="min-w-0 flex-1 text-[13px] text-ink-2">
                {t(`instructions.project.how.${r.how}`, { name: r.target, file: `./${r.file}` })}
              </span>
              <span className="w-[110px] shrink-0">
                <span className={`ss-st ${r.reads ? 'ok' : 'warn'}`}>
                  {t(r.reads ? 'instructions.project.reads' : 'instructions.project.blocked')}
                </span>
              </span>
              <span className="flex w-[170px] shrink-0 justify-end">
                {r.shim && (
                  <Button
                    variant="secondary"
                    size="sm"
                    loading={busy === r.target}
                    disabled={busy !== null}
                    onClick={() => void shim(r)}
                  >
                    {r.shim === 'import'
                      ? t('instructions.project.shimImport')
                      : t('instructions.project.shimLink', { file: r.file })}
                  </Button>
                )}
              </span>
            </div>
          ))}
        </div>
      </section>

      <ProjectSharedFiles creating={creating} setCreating={setCreating} />

      <div className="ss-note inf">
        <Info size={16} />
        <span className="flex-1">{t('instructions.project.oneFile')}</span>
      </div>

      {editing && (
        <InstructionsEditorDialog
          title="AGENTS.md"
          path={data.path}
          content={data.content}
          note={t('instructions.project.editNote')}
          onSave={async (content) => {
            await api.putProjectInstructions(content);
            void invalidate(queryClient, 'instructionsChanged');
          }}
          onClose={() => setEditing(false)}
        />
      )}
    </div>
  );
}
