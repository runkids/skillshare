import { Info } from 'lucide-react';
import { SEV, type Finding } from '../lib/auditMessage';

export default function FindingList({ findings, header, showName }: { findings: Finding[]; header: string; showName: boolean }) {
  return (
    <div className="ss-list !shadow-none max-h-[320px] !overflow-y-auto">
      <div className="ss-lh"><span className="flex-1">{header}</span></div>
      {findings.map((f, i) => {
        const where = [showName && f.name, f.where].filter(Boolean).join(' · ');
        return (
          <div key={i} className="ss-r !min-h-12">
            <span className="w-[74px] shrink-0">
              {f.severity ? <span className={`ss-sev ${SEV[f.severity]}`}>{f.severity}</span> : <Info size={15} className="text-ink-3" />}
            </span>
            <div className="flex min-w-0 flex-1 flex-col gap-px">
              <span className="break-words font-semibold">{f.message}</span>
              {where && <span className="truncate font-mono text-xs text-ink-3">{where}</span>}
              {f.snippet && <span className="truncate font-mono text-xs text-ink-3">{f.snippet}</span>}
            </div>
          </div>
        );
      })}
    </div>
  );
}
