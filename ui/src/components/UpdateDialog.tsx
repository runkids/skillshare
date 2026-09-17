import { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { X, Copy, Check, CircleArrowUp } from 'lucide-react';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { api } from '../api/client';
import type { VersionCheck } from '../api/client';
import DialogShell from './DialogShell';
import Button from './Button';
import { useT } from '../i18n';

const DISMISSED_KEY = 'ss-update-dialog-dismissed';

/** Dev-only mock data triggered by ?update-test URL param */
const mockData: VersionCheck = {
  cliVersion: '0.17.0',
  cliLatest: '0.18.0',
  cliUpdateAvailable: true,
  skillVersion: '0.17.0',
  skillLatest: '0.18.0',
  skillUpdateAvailable: true,
};

export default function UpdateDialog() {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [status, setStatus] = useState<string | null>(null);

  const isMockMode = new URLSearchParams(window.location.search).has('update-test');

  const { data: realData } = useQuery({
    queryKey: queryKeys.versionCheck,
    queryFn: () => api.getVersionCheck(),
    staleTime: staleTimes.version,
    enabled: !isMockMode,
  });

  const data = isMockMode ? mockData : realData;
  const hasUpdate = data?.cliUpdateAvailable || data?.skillUpdateAvailable;

  useEffect(() => {
    if (!hasUpdate) return;
    if (!isMockMode) {
      try {
        if (sessionStorage.getItem(DISMISSED_KEY)) return;
      } catch { /* storage unavailable */ }
    }
    setOpen(true);
  }, [hasUpdate, isMockMode]);

  if (!data || !hasUpdate) return null;

  const dismiss = () => {
    setOpen(false);
    try { sessionStorage.setItem(DISMISSED_KEY, '1'); } catch {
      // Ignore storage access failures.
    }
  };

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText('skillshare upgrade');
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Ignore clipboard failures.
    }
  };

  const waitForRestartThenReload = async () => {
    await new Promise((resolve) => setTimeout(resolve, 800));
    for (let i = 0; i < 40; i++) {
      try {
        await api.health();
        window.location.reload();
        return;
      } catch {
        await new Promise((resolve) => setTimeout(resolve, 500));
      }
    }
    setStatus(t('updateDialog.restartManual', {}, 'Updated. Restart with skillshare ui start if this page does not reconnect.'));
    setUpdating(false);
  };

  const handleUpdateNow = async () => {
    setUpdating(true);
    setStatus(t('updateDialog.updating', {}, 'Updating Skillshare…'));
    try {
      const result = await api.upgradeApp();
      if (result.devMode) {
        setStatus(t('updateDialog.restartDev', {}, 'DEV mode restart simulated.'));
        await new Promise((resolve) => setTimeout(resolve, 900));
        setUpdating(false);
        setOpen(false);
        return;
      }
      setStatus(t('updateDialog.restarting', {}, 'Restarting local UI server…'));
      await api.restartApp({ clearCache: true });
      void waitForRestartThenReload();
    } catch (err) {
      setStatus((err as Error).message);
      setUpdating(false);
    }
  };

  return (
    <DialogShell open={open} onClose={dismiss} maxWidth="md" padding="none" preventClose={updating} ariaLabel={t('updateDialog.newVersion')}>
      <div className="dh">
        <h2 className="ss-h2">{t('updateDialog.newVersion')}</h2>
        <button type="button" className="ss-ib" onClick={dismiss} aria-label={t('common.close')} disabled={updating}>
          <X size={16} />
        </button>
      </div>
      <div className="db">
        <dl className="ss-kv">
          {data.cliUpdateAvailable && (
            <>
              <dt>CLI</dt>
              <dd className="font-mono">{data.cliVersion} → <b>{data.cliLatest}</b></dd>
            </>
          )}
          {data.skillUpdateAvailable && (
            <>
              <dt>Skill</dt>
              <dd className="font-mono">{data.skillVersion} → <b>{data.skillLatest}</b></dd>
            </>
          )}
        </dl>
        <div className="ss-fld">
          <label>{t('updateDialog.terminal')}</label>
          <div className="flex items-center gap-2">
            <code className="ss-code flex-1 !py-2">skillshare upgrade</code>
            <Button variant="secondary" onClick={handleCopy} disabled={updating}>
              {copied ? <Check size={15} className="text-ok" /> : <Copy size={15} />}
              {t('updateDialog.copyCommand')}
            </Button>
          </div>
        </div>
        <p className="text-[13px] text-ink-2">{t('updateDialog.restartNote')}</p>
        {status && <p className={`text-[13px] ${updating ? 'text-ink-2' : 'text-bad'}`}>{status}</p>}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={dismiss} disabled={updating}>
          {t('updateDialog.later')}
        </Button>
        <Button variant="primary" onClick={handleUpdateNow} loading={updating}>
          <CircleArrowUp size={15} />
          {t('updateDialog.updateNow', {}, 'Update now')}
        </Button>
      </div>
    </DialogShell>
  );
}
