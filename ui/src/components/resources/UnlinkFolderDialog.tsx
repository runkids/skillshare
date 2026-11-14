import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { Skill, SourceLink } from '../../api/client';
import { useT } from '../../i18n';
import ConfirmDialog from '../ConfirmDialog';
import { useToast } from '../Toast';
import { invalidate } from '../../lib/queryEvents';

export default function UnlinkFolderDialog({ link, skills, onClose }: { link: SourceLink; skills: Skill[]; onClose: () => void }) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState('');

  const unlink = async () => {
    if (saving) return;
    setSaving(true);
    setFailure('');
    try {
      const result = await api.removeSourceLink(link.name);
      await invalidate(queryClient, 'sourceUnlinked');
      if (result.warning) toast(t('sourceLinks.metadataWarning'), 'warning');
      else toast(t('sourceLinks.unlinked', { name: link.name }), 'success');
      onClose();
    } catch (err) {
      setFailure((err as Error).message);
      setSaving(false);
    }
  };

  return (
    <ConfirmDialog open title={t('sourceLinks.unlinkTitle', { name: link.name })} confirmText={t('sourceLinks.unlink')}
      variant="danger" loading={saving} onConfirm={() => void unlink()} onCancel={onClose}
      message={
        <div className="flex flex-col gap-3">
          <p>{t('sourceLinks.unlinkMessage')}</p>
          <div className="ss-list !shadow-none">
            <div className="ss-r items-start gap-3">
              <span className="w-16 shrink-0 text-xs text-ink-3">{t('sourceLinks.target')}</span>
              <span className="min-w-0 break-all font-mono text-xs">{link.target}</span>
            </div>
            <div className="ss-r items-start gap-3">
              <span className="w-16 shrink-0 text-xs text-ink-3">{t('layout.nav.skills')}</span>
              <div className="min-w-0">
                <p className="break-words font-mono text-xs">{skills.length ? skills.map((s) => s.name).join(', ') : t('resources.count.skills', { count: 0 })}</p>
                <p className="mt-1 text-xs text-ink-3">{t('sourceLinks.unlinkSkills')}</p>
              </div>
            </div>
          </div>
          <p className="text-xs text-ink-3">{t('sourceLinks.unlinkTrash')}</p>
          {failure && <div className="ss-note bad" role="alert">{failure}</div>}
        </div>
      }
    />
  );
}
