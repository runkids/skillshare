import type { ReactNode } from 'react';
import { Info } from 'lucide-react';
import IconButton from '../IconButton';
import Tooltip from '../Tooltip';

interface Props {
  icon: ReactNode;
  title: string;
  /** A number, or a count such as `3 / 12`; left out, none is shown. */
  count?: number | string;
  /** A word or two on whether the section's items start on. */
  chip?: string;
  /** How the section works, shown on hover or focus rather than taking a line of its own. */
  hint: string;
  /** Actions at the end of the row. */
  children?: ReactNode;
}

/** The head of one section of a project's tab. */
export default function ProjectSectionHead({ icon, title, count, chip, hint, children }: Props) {
  return (
    <div className="ss-sec !items-center !gap-2">
      <span className="flex text-ink-2">{icon}</span>
      <h2 className="shrink-0">{title}</h2>
      {count !== undefined && <span className="ss-cnt">{count}</span>}
      {chip && <span className="ss-tag">{chip}</span>}
      <InfoTip hint={hint} />
      {children && <div className="ml-auto flex flex-wrap items-center gap-1">{children}</div>}
    </div>
  );
}

/** An info button whose text shows on hover or focus. */
export function InfoTip({ hint }: { hint: string }) {
  return (
    <Tooltip content={hint} side="top">
      {/* The tip mounts only while shown, so the label carries the text for screen readers;
          no native title, which would show a second tip. */}
      <IconButton
        icon={<Info size={14} />}
        label={hint}
        title={undefined}
        className="!h-7 !w-7 text-ink-3"
      />
    </Tooltip>
  );
}
