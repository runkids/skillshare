import { Info } from 'lucide-react';
import { useT } from '../../i18n';
import { mcpNotice } from './mcpView';

/** The plan's notices: what a sync changes beyond the Agent files, and what an Agent cannot hold. */
export default function MCPNotices({ notices }: { notices?: string[] }) {
  const t = useT();
  if (!notices?.length) return null;
  return (
    <div className="flex flex-col gap-2">
      {notices.map((notice) => <div key={notice} className="ss-note"><Info size={16} className="shrink-0" /><span className="flex-1">{mcpNotice(t, notice)}</span></div>)}
    </div>
  );
}
