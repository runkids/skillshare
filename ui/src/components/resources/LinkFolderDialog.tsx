import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { X } from 'lucide-react';
import { api } from '../../api/client';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import DialogShell from '../DialogShell';
import { Input } from '../Input';
import { useToast } from '../Toast';
import { invalidate } from '../../lib/queryEvents';

export default function LinkFolderDialog({ onClose }: { onClose: () => void }) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const {
    data,
    isPending,
    error: configError,
  } = useQuery({ queryKey: queryKeys.config, queryFn: api.getConfig });
  // /config returns the current mode's Go config, with exported field names.
  const following = Boolean(
    (data?.config as { FollowSourceLinks?: boolean } | undefined)?.FollowSourceLinks,
  );
  const [path, setPath] = useState('');
  const [name, setName] = useState('');
  const [enable, setEnable] = useState(false);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState('');
  const title = t('sourceLinks.title');

  const submit = async () => {
    if (!path.trim() || saving || isPending || configError) return;
    setSaving(true);
    setFailure('');
    try {
      const res = await api.createSourceLink({
        path: path.trim(),
        ...(name.trim() ? { name: name.trim() } : {}),
        enable: !following && enable,
      });
      await invalidate(queryClient, 'sourceLinked');
      toast(t('sourceLinks.created'), 'success');
      if (res.warning) toast(res.warning, 'warning');
      if (!following && !enable) toast(t('sourceLinks.notFollowing'), 'info');
      onClose();
    } catch (err) {
      setFailure((err as Error).message);
      setSaving(false);
    }
  };

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={saving} ariaLabel={title}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <div className="dh">
          <div>
            <h2 className="ss-h2">{title}</h2>
            <p className="text-[13px] text-ink-2">{t('sourceLinks.description')}</p>
          </div>
          <button
            type="button"
            className="ss-ib"
            aria-label={t('common.close')}
            onClick={onClose}
            disabled={saving}
          >
            <X size={16} />
          </button>
        </div>
        <div className="db flex flex-col gap-4">
          <Input
            label={t('sourceLinks.path')}
            autoFocus
            value={path}
            onChange={(e) => setPath(e.target.value)}
            disabled={saving}
            autoComplete="off"
            spellCheck={false}
          />
          <Input
            label={t('sourceLinks.name')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={saving}
            autoComplete="off"
            spellCheck={false}
          />
          <p className="text-xs text-ink-3">{t('sourceLinks.nameHint')}</p>
          {data && !following && (
            <Checkbox
              label={t('sourceLinks.enable')}
              checked={enable}
              onChange={setEnable}
              disabled={saving}
            />
          )}
          {(failure || configError) && (
            <div className="ss-note bad" role="alert">
              {failure || configError?.message}
            </div>
          )}
        </div>
        <div className="df">
          <span className="flex-1" />
          <Button type="button" variant="ghost" size="sm" onClick={onClose} disabled={saving}>
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="sm"
            loading={saving}
            disabled={!path.trim() || isPending || Boolean(configError)}
          >
            {title}
          </Button>
        </div>
      </form>
    </DialogShell>
  );
}
