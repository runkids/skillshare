import { useState } from 'react';
import { mcpTargets } from '../../api/mcp';
import Button from '../Button';
import ConfirmDialog from '../ConfirmDialog';
import { useT } from '../../i18n';
import { TargetToggles } from './TargetPicker';
import { targetLabel } from './mcpView';

interface TargetsProps {
  /** undefined follows mcp.targets. */
  value: string[] | undefined;
  defaults: string[];
  offered: readonly string[];
  /** Resolves false when the save failed. */
  onChange: (value: string[] | undefined) => Promise<boolean> | void;
  disabled?: boolean;
}

/** A project's targets: the global ones until an Agent is ticked here, then the project's own pick. */
export function ProjectTargets({ value, defaults, offered, onChange, disabled }: TargetsProps) {
  const t = useT();
  // An empty own list is stored as "no key", which reads back as inherit. Without
  // local state the toggles would spring back to the global ones whenever the
  // project's own list is emptied.
  const [picking, setPicking] = useState(false);
  const [asking, setAsking] = useState(false);
  // The list last sent, shown until the saved one comes back, so a quick second tick builds on the first.
  const [sent, setSent] = useState<string[] | null>(null);
  const [shown, setShown] = useState(value);
  if (shown !== value) {
    setShown(value);
    setSent(null);
  }
  const own = value !== undefined || picking;
  // Inheriting, the global targets show ticked, and the first change starts the project's own list from them.
  const selected = sent ?? (own ? (value ?? []) : defaults);
  const pick = (target: string, on: boolean) => {
    const next = mcpTargets.filter((x) => (x === target ? on : selected.includes(x)));
    setPicking(true);
    setSent(next);
    // A failed save goes back to what the config holds.
    void Promise.resolve(onChange(next)).then((ok) => {
      if (ok !== false) return;
      setSent(null);
      setPicking(false);
    });
  };
  // The project's own list as on screen: a list just saved counts before it comes back.
  const ownList = sent ?? value;
  // Going back drops the project's own list, which then has to be ticked again.
  const inherit = () => {
    if (ownList?.length) return setAsking(true);
    setPicking(false);
    setSent(null);
    if (ownList !== undefined) return onChange(undefined);
  };
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-x-6 gap-y-3.5">
        <TargetToggles offered={offered} selected={selected} onToggle={pick} disabled={disabled} />
      </div>
      {own && (
        <Button
          variant="link"
          className="min-h-6 self-end !text-xs"
          onClick={inherit}
          disabled={disabled}
        >
          {t('mcp.projects.backToGlobal')}
        </Button>
      )}
      <ConfirmDialog
        open={asking}
        title={t('mcp.projects.inheritTitle')}
        message={t('mcp.projects.inheritMessage', {
          targets: (ownList ?? []).map(targetLabel).join(', '),
        })}
        confirmText={t('mcp.projects.inheritConfirm')}
        onCancel={() => setAsking(false)}
        onConfirm={() => {
          setAsking(false);
          setPicking(false);
          setSent(null);
          return onChange(undefined);
        }}
      />
    </div>
  );
}
