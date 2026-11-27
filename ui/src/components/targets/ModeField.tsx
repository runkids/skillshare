import type { ReactNode } from 'react';
import { useT } from '../../i18n';
import SegmentedControl from '../SegmentedControl';

const MODES = ['merge', 'copy', 'symlink'] as const;
export type SyncMode = (typeof MODES)[number];

/** The sync mode for the rail: merge, copy or symlink, with what the chosen one does. */
export default function ModeField({
  kind,
  mode,
  onChange,
  disabled,
  children,
}: {
  kind: 'skill' | 'agent';
  mode: string;
  onChange: (mode: SyncMode) => void;
  disabled: boolean;
  /** Notes under the description, such as another target that shares the folder */
  children?: ReactNode;
}) {
  const t = useT();
  return (
    <div className="flex flex-col gap-2.5">
      <h3 className="text-[15px] font-semibold">{t('targetDetail.syncMode')}</h3>
      <SegmentedControl
        value={mode as SyncMode}
        onChange={onChange}
        options={MODES.map((m) => ({ value: m, label: m, disabled }))}
        className="self-start"
      />
      <p className="text-xs leading-normal text-ink-2">
        {mode === 'merge' && `${t('targetDetail.default')} · `}
        {t(mode === 'merge' ? `targetDetail.mode.merge.${kind}` : `targetDetail.mode.${mode}`)}
      </p>
      {children}
    </div>
  );
}
