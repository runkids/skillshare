import { Check, ChevronDown } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import AgentStack from '../AgentStack';
import { targetLabel } from './mcpView';

const STACK = 6;

/** The selected Agents as a stack of logos. `text` is the count, or a word such as "inherited". */
export function TargetPill({
  selected,
  text,
  expanded,
  label,
  onClick,
}: {
  selected: string[];
  text: string;
  expanded: boolean;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`ss-btn ${selected.length > 0 ? '!pl-1.5' : ''}`}
      aria-expanded={expanded}
      aria-label={label}
      onClick={onClick}
    >
      {selected.length > 0 && (
        <AgentStack targets={selected} max={STACK} showRest aria-hidden="true" />
      )}
      {text}
      <ChevronDown
        size={14}
        className={`transition-transform duration-200 ease-[cubic-bezier(.3,.7,.3,1)] motion-reduce:transition-none ${expanded ? 'rotate-180' : ''}`}
      />
    </button>
  );
}

/**
 * One tickable logo per Agent. Claude Desktop only takes stdio servers, so `http` locks it.
 * `tags` adds a note after an Agent's name, such as Pi's exposure mode.
 */
export function TargetToggles({
  offered,
  selected,
  onToggle,
  http = false,
  disabled = false,
  tags,
}: {
  offered: readonly string[];
  selected: string[];
  onToggle: (target: string, on: boolean) => void;
  http?: boolean;
  disabled?: boolean;
  tags?: Partial<Record<string, { text: string; title?: string }>>;
}) {
  return (
    <>
      {offered.map((target) => {
        const on = selected.includes(target);
        return (
          <button
            key={target}
            type="button"
            role="checkbox"
            aria-checked={on}
            className={`ss-tgl ${on ? 'on' : ''}`}
            onClick={() => onToggle(target, !on)}
            disabled={disabled || (target === 'claude-desktop' && http && !on)}
          >
            <span className="ic">
              <AgentIcon target={target} size={20} />
              <i>
                <Check size={9} strokeWidth={3.5} />
              </i>
            </span>
            {targetLabel(target)}
            {target === 'claude-desktop' && <span className="ss-tag">stdio</span>}
            {tags?.[target] && (
              <span className="ss-tag" title={tags[target].title}>
                {tags[target].text}
              </span>
            )}
          </button>
        );
      })}
    </>
  );
}
