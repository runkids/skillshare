import { useState } from 'react';
import AgentIcon from '../AgentIcon';
import CodeEditor from '../CodeEditor';
import { Select } from '../Input';
import { useT } from '../../i18n';
import { InfoTip } from './PiSettingsFields';
import { targetLabel } from './mcpView';

interface Props {
  /** The Agents to offer, the one to open on first. */
  agents: string[];
  texts: Record<string, string>;
  error: string;
  onChange: (agent: string, text: string) => void;
  disabled: boolean;
}

const examples: Record<string, string> = {
  codex: '{\n  "default_tools_approval_mode": "approve"\n}',
  claude: '{\n  "timeout": 600000\n}',
};

/** Other Agents' native per-server fields, edited one Agent at a time so many ticked Agents stay short. */
export default function AgentOptionsFields({ agents, texts, error, onChange, disabled }: Props) {
  const t = useT();
  const [picked, setPicked] = useState('');
  const agent = agents.includes(picked) ? picked : agents[0];
  const label = t('mcp.agentOptions', { agent: targetLabel(agent) });
  return (
    <>
      <div className="flex items-center gap-1">
        <span className="text-[13px] font-semibold">{t('mcp.agentSettings')}</span>
        <InfoTip label={t('mcp.agentSettingsInfo')} content={t('mcp.agentSettingsTip')} />
      </div>
      <div className="ss-fld">
        <label>{t('mcp.agentSettingsAgent')}</label>
        <Select
          ariaLabel={t('mcp.agentSettingsAgent')}
          value={agent}
          onChange={setPicked}
          disabled={disabled}
          options={agents.map((a) => ({ value: a, label: targetLabel(a), icon: <AgentIcon target={a} size={16} />, ...(texts[a]?.trim() && { note: `· ${t('mcp.agentOptionsSet')}` }) }))}
        />
      </div>
      <div className="ss-fld">
        <label>{label}</label>
        <CodeEditor key={agent} value={texts[agent] ?? ''} onChange={(text) => onChange(agent, text)} lang="json" placeholder={examples[agent] ?? '{\n  "field": "value"\n}'} ariaLabel={label} disabled={disabled} minHeight="96px" />
        {error ? <span className="hp !text-bad">{error}</span> : <span className="hp">{t('mcp.agentOptionsHint')}</span>}
      </div>
    </>
  );
}
