import { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { X, Copy, Check } from 'lucide-react';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { radius } from '../design';
import { api } from '../api/client';
import type { VersionCheck } from '../api/client';
import DialogShell from './DialogShell';
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
    try { sessionStorage.setItem(DISMISSED_KEY, '1'); } catch {}
  };

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText('skillshare upgrade');
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {}
  };

  return (
    <DialogShell open={open} onClose={dismiss} maxWidth="sm">
        {/* Close */}
        <button
          onClick={dismiss}
          className="absolute top-3 right-3 p-1 text-pencil-light hover:text-pencil transition-colors cursor-pointer"
          aria-label={t('common.close')}
        >
          <X size={16} />
        </button>

        {/* Title — plain text, no icon block */}
        <p className="text-sm font-medium text-pencil mb-3 pr-6">
          {t('updateDialog.newVersion')}
        </p>

        {/* Version lines */}
        <div className="space-y-1.5 mb-4">
          {data.cliUpdateAvailable && (
            <div className="flex items-baseline gap-2 text-sm">
              <span className="text-pencil-light w-10">CLI</span>
              <span className="font-mono text-pencil-light">{data.cliVersion}</span>
              <span className="text-pencil-light">&rarr;</span>
              <span className="font-mono font-medium text-pencil">{data.cliLatest}</span>
            </div>
          )}
          {data.skillUpdateAvailable && (
            <div className="flex items-baseline gap-2 text-sm">
              <span className="text-pencil-light w-10">Skill</span>
              <span className="font-mono text-pencil-light">{data.skillVersion}</span>
              <span className="text-pencil-light">&rarr;</span>
              <span className="font-mono font-medium text-pencil">{data.skillLatest}</span>
            </div>
          )}
        </div>

        {/* Upgrade command — inline copyable */}
        <div
          className="flex items-center justify-between py-2 px-3 bg-muted/30 border border-dashed border-pencil-light/30"
          style={{ borderRadius: radius.sm }}
        >
          <code className="font-mono text-sm text-pencil">skillshare upgrade</code>
          <button
            onClick={handleCopy}
            className="p-1 text-pencil-light hover:text-pencil transition-colors cursor-pointer"
            aria-label={t('updateDialog.copyCommand')}
          >
            {copied
              ? <Check size={14} className="text-success" />
              : <Copy size={14} />}
          </button>
        </div>
    </DialogShell>
  );
}
