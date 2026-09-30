import CodeEditor from '../CodeEditor';
import { Select } from '../Input';
import { useT } from '../../i18n';
import { piExposures, type parsePiOptions } from './mcpView';

interface Props {
  optionsText: string;
  options: ReturnType<typeof parsePiOptions>;
  optionsError: string;
  onOptions: (text: string) => void;
  disabled: boolean;
  project: boolean;
  /** The server has a tool policy, which sets Pi's exposure. */
  toolsSet?: boolean;
}

/** Pi's tool exposure and other settings, shared by the form and a single pasted server. */
export default function PiSettingsFields({ optionsText, options, optionsError, onOptions, disabled, project, toolsSet = false }: Props) {
  const t = useT();
  return (
    <>
      <div className="ss-fld">
        <Select label={t('mcp.piExposure')} value={String(options.value?.exposure ?? '')} disabled={disabled || Boolean(optionsError) || toolsSet} onChange={(value) => {
          const next = { ...options.value };
          if (value) next.exposure = value; else delete next.exposure;
          onOptions(JSON.stringify(next, null, 2));
        }} options={[{ value: '', label: t('mcp.piExposureNone'), note: `· ${t('mcp.piExposureUnset')}` }, ...piExposures.map((value) => ({ value, label: value, note: `· ${t(`mcp.piExposure.${value}`)}` }))]} />
        <span className="hp">{t(toolsSet ? 'mcp.piExposureFromTools' : 'mcp.piExposureHint')}</span>
        <span className="hp">{t('mcp.piBuiltinHelp')}</span>
        <span className="hp">{t(project ? 'mcp.piProjectHint' : 'mcp.piGlobalHint')}</span>
        <a className="self-start text-xs text-link underline" href="https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md" target="_blank" rel="noopener noreferrer">Pi MCP · {t('plugins.officialDocs')}</a>
      </div>
      <div className="ss-fld">
        <label>{t('mcp.piOptions')}</label>
        <CodeEditor value={optionsText} onChange={onOptions} lang="json" placeholder={'{\n  "timeout": 120,\n  "toolExposure": {"delete_*": "hidden"}\n}'} ariaLabel={t('mcp.piOptions')} disabled={disabled} minHeight="96px" />
        {optionsError ? <span className="hp !text-bad">{optionsError}</span> : <span className="hp">{t('mcp.piOptionsHint')}</span>}
      </div>
    </>
  );
}
