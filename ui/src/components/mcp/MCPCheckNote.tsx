import { useEffect, useState } from 'react';
import { Check, Info, RefreshCw, TriangleAlert } from 'lucide-react';
import type { MCPCheckReport } from '../../api/mcpCheck';
import Button from '../Button';
import { formatRelativeTime, useI18n, plural } from '../../i18n';
import { describeError } from './mcpView';
import { problemsByServer, serversFor } from './useMCPCheck';

interface Props {
  report?: MCPCheckReport;
  checkedAt?: number;
  error?: string;
  running: boolean;
  /** The mcp.projects root the page shows; absent on the global MCP page. */
  project?: string;
  onRun: () => void;
}

/** The last check's summary. Hidden until the first check runs. */
export default function MCPCheckNote({ report, checkedAt, error, running, onRun, project }: Props) {
  const { t, locale } = useI18n();
  const [now, setNow] = useState(() => Date.now());
  // Keeps "checked just now" true as the page stays open.
  useEffect(() => {
    if (!checkedAt) return;
    const id = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(id);
  }, [checkedAt]);

  if (error)
    return (
      <div className="ss-note bad">
        <TriangleAlert size={16} />
        <span className="flex-1">{describeError(t, error)}</span>
      </div>
    );
  if (!report || !checkedAt) return null;
  const count = Object.keys(problemsByServer(report, project)).length;
  // Counts only what the page lists; each project's servers belong to that project's page.
  const servers = serversFor(report, project);
  const total = servers.length;
  const levels = servers.flatMap((server) => server.findings.map((f) => f.level));
  const errors = levels.filter((level) => level === 'error').length;
  const warnings = levels.filter((level) => level === 'warning').length;
  const checked =
    now - checkedAt < 60_000
      ? t('mcp.check.justNow')
      : t('mcp.check.checkedAt', { time: formatRelativeTime(checkedAt, locale) });
  return (
    <div className={`ss-note ${count > 0 ? 'bad' : 'inf'} !items-center`}>
      {count > 0 ? (
        <TriangleAlert size={16} className="!mt-0" />
      ) : (
        <Check size={16} className="!mt-0" />
      )}
      <div className="min-w-0 flex-1">
        <div>
          <b>
            {count > 0
              ? t(plural('mcp.check.problems', count), { count })
              : t(plural('mcp.check.allGood', total), { count: total })}
          </b>
          <span className="opacity-70">
            {' '}
            · {count > 0 && <>{t('mcp.check.counts', { errors, warnings })} · </>}
            {checked}
          </span>
        </div>
        <div className="mt-[3px] flex items-center gap-1.5 text-xs opacity-70">
          <Info size={13} className="shrink-0" />
          {t('mcp.check.envNote')}
        </div>
      </div>
      <Button size="sm" variant="secondary" disabled={running} onClick={onRun}>
        <RefreshCw size={14} />
        {t('mcp.check.rerun')}
      </Button>
    </div>
  );
}
