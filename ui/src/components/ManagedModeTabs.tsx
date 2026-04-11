import Card from './Card';
import HandButton from './HandButton';

export type ManagedRulesMode = 'managed' | 'discovered';

interface ManagedModeTabsProps {
  mode: ManagedRulesMode;
  onChange: (mode: ManagedRulesMode) => void;
  managedCount?: number;
  discoveredCount?: number;
  label?: string;
  managedLabel?: string;
}

function tabLabel(label: string, count?: number) {
  return count === undefined ? label : `${label} (${count})`;
}

export default function ManagedModeTabs({
  mode,
  onChange,
  managedCount,
  discoveredCount,
  label = 'Rules mode',
  managedLabel = 'Managed',
}: ManagedModeTabsProps) {
  return (
    <Card className="p-2">
      <div role="tablist" aria-label={label} className="flex flex-wrap gap-2">
        <HandButton
          type="button"
          role="tab"
          aria-selected={mode === 'managed'}
          variant={mode === 'managed' ? 'primary' : 'ghost'}
          size="sm"
          onClick={() => onChange('managed')}
        >
          {tabLabel(managedLabel, managedCount)}
        </HandButton>
        <HandButton
          type="button"
          role="tab"
          aria-selected={mode === 'discovered'}
          variant={mode === 'discovered' ? 'primary' : 'ghost'}
          size="sm"
          onClick={() => onChange('discovered')}
        >
          {tabLabel('Discovered', discoveredCount)}
        </HandButton>
      </div>
    </Card>
  );
}
