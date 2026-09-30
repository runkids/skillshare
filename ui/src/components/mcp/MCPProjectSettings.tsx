import { useState } from 'react';
import { mcpTargets } from '../../api/mcp';
import ConfirmDialog from '../ConfirmDialog';
import SegmentedControl from '../SegmentedControl';
import { useT } from '../../i18n';
import { TargetToggles } from './TargetPicker';
import { targetLabel } from './mcpView';

interface TargetsProps {
  /** undefined follows mcp.targets. */
  value: string[] | undefined;
  defaults: string[];
  offered: readonly string[];
  onChange: (value: string[] | undefined) => void;
  disabled?: boolean;
}

/** A project's targets: the global ones, or its own pick. */
export function ProjectTargets({ value, defaults, offered, onChange, disabled }: TargetsProps) {
  const t = useT();
  // An empty own list is stored as "no key", which reads back as inherit. Without
  // local state the toggle would spring back whenever there is nothing to save yet:
  // both on the way in, and when a project that already had its own list is emptied.
  const [picking, setPicking] = useState(false);
  const [asking, setAsking] = useState(false);
  const own = value !== undefined || picking;
  const pick = (target: string, on: boolean) => {
    setPicking(true);
    onChange(mcpTargets.filter((x) => (x === target ? on : value?.includes(x))));
  };
  const mode = (v: 'inherit' | 'own') => {
    // Going back saves at once and drops the project's own list, which then has to be ticked again.
    if (v === 'inherit' && value?.length) return setAsking(true);
    setPicking(v === 'own');
    // Seeding from the global list saves at once; with nothing to seed, the empty
    // toggles are shown and the first tick is what saves.
    if (v === 'inherit') onChange(undefined);
    else if (defaults.length > 0) onChange(defaults);
  };
  return (
    <div className="flex flex-col gap-3">
      <SegmentedControl
        className="self-start"
        value={own ? 'own' : 'inherit'}
        onChange={mode}
        options={[{ value: 'inherit', label: t('mcp.projects.inherit') }, { value: 'own', label: t('mcp.projects.ownTargets') }]}
      />
      {own && <div className="flex flex-wrap gap-x-5 gap-y-3"><TargetToggles offered={offered} selected={value ?? []} onToggle={pick} disabled={disabled} /></div>}
      <ConfirmDialog open={asking} title={t('mcp.projects.inheritTitle')} message={t('mcp.projects.inheritMessage', { targets: (value ?? []).map(targetLabel).join(', ') })} confirmText={t('mcp.projects.inheritConfirm')} onCancel={() => setAsking(false)} onConfirm={() => { setAsking(false); setPicking(false); onChange(undefined); }} />
      <span className="hp text-xs text-ink-2">{defaults.length > 0 ? t('mcp.projects.targetsHint', { targets: defaults.map(targetLabel).join(', ') }) : t('mcp.projects.noGlobalTargets')}</span>
    </div>
  );
}

