import { useContext, useState } from 'react';
import type { MCPSettings } from '../../api/mcp';
import { useT } from '../../i18n';
import { TargetPill, TargetToggles } from './TargetPicker';
import { MCPTargetOrder } from './targetOrder';

interface Props {
  targets: string[];
  offered: readonly string[];
  onSave: (settings: MCPSettings) => void;
}

/** mcp.targets: what a server without its own targets falls back to. */
export default function MCPDefaults({ targets, offered, onSave }: Props) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const order = useContext(MCPTargetOrder);
  const shown = order.filter((x) => offered.includes(x) || targets.includes(x));
  return (
    <section className="mt-4 flex flex-col">
      <div className="ss-sec"><h2>{t('mcp.defaults')}</h2></div>
      <div className="ss-box !p-0">
        <div className="ss-setrow">
          <div className="l"><b>{t('mcp.targets')}</b><span>{t('mcp.defaults.targetsHint')}</span></div>
          <TargetPill selected={targets} text={`${targets.length}/${shown.length}`} expanded={open} label={t('mcp.defaults.chooseAgents')} onClick={() => setOpen(!open)} />
        </div>
        {open && (
          <div className="flex flex-wrap gap-x-6 gap-y-3.5 px-[18px] pb-4">
            <TargetToggles offered={shown} selected={targets} onToggle={(target, on) => onSave({ targets: order.filter((x) => (x === target ? on : targets.includes(x))) })} />
          </div>
        )}
      </div>
    </section>
  );
}
