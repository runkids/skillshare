import { scalarText } from '../../lib/scalarText';
import { Info } from 'lucide-react';
import { Checkbox } from '../Checkbox';
import CodeEditor from '../CodeEditor';
import { Select } from '../Input';
import Tooltip from '../Tooltip';
import { useT } from '../../i18n';
import { piExposures, type parsePiOptions } from './mcpView';

interface Props {
  optionsText: string;
  options: ReturnType<typeof parsePiOptions>;
  optionsError: string;
  onOptions: (text: string) => void;
  disabled: boolean;
  project: boolean;
  /** The server has a tool policy, which writes Pi's toolExposure. */
  toolsSet?: boolean;
}

/** An info icon that explains in a tooltip, reachable by keyboard. */
export function InfoTip({ label, content }: { label: string; content: string }) {
  return (
    <Tooltip content={content}>
      <button type="button" className="ss-ib !h-6 !w-6" aria-label={label}>
        <Info size={14} />
      </button>
    </Tooltip>
  );
}

/** Pi's tool exposure and other settings, shared by the form and a single pasted server. */
export default function PiSettingsFields({
  optionsText,
  options,
  optionsError,
  onOptions,
  disabled,
  project,
  toolsSet = false,
}: Props) {
  const t = useT();
  const locked = disabled || Boolean(optionsError);
  // The controls edit the JSON; nothing left shows the editor's placeholder rather than "{}".
  const setOption = (key: string, value: unknown) => {
    const next = { ...options.value };
    if (value === undefined) delete next[key];
    else next[key] = value;
    onOptions(Object.keys(next).length ? JSON.stringify(next, null, 2) : '');
  };
  return (
    <>
      <div className="flex items-center gap-1">
        <span className="text-[13px] font-semibold">{t('mcp.piSettings')}</span>
        <InfoTip
          label={t('mcp.piSettingsInfo')}
          content={`${t('mcp.piBuiltinHelp')}\n${t(project ? 'mcp.piProjectHint' : 'mcp.piGlobalHint')}`}
        />
        <a
          className="ml-1 text-xs text-link underline"
          href="https://github.com/earendil-works/pi/blob/v0.99.2/packages/coding-agent/docs/mcp.md"
          target="_blank"
          rel="noopener noreferrer"
        >
          Pi MCP · {t('plugins.officialDocs')}
        </a>
      </div>
      <div className="ss-fld">
        <Checkbox
          label={t('mcp.piEnabled')}
          size="sm"
          checked={options.value?.enabled !== false}
          disabled={locked}
          onChange={(on) => setOption('enabled', on ? undefined : false)}
        />
        <span className="hp">{t('mcp.piEnabledHint')}</span>
      </div>
      <div className="ss-fld">
        <span className="flex items-center gap-1">
          <label>{t('mcp.piExposure')}</label>
          <InfoTip label={t('mcp.piExposureInfo')} content={t('mcp.piExposureTip')} />
        </span>
        <Select
          ariaLabel={t('mcp.piExposure')}
          value={scalarText(options.value?.exposure)}
          disabled={locked}
          onChange={(value) => setOption('exposure', value || undefined)}
          options={[
            { value: '', label: t('mcp.piExposureNone'), note: `· ${t('mcp.piExposureUnset')}` },
            ...piExposures.map((value) => ({
              value,
              label: value,
              note: `· ${t(`mcp.piExposure.${value}`)}`,
            })),
          ]}
        />
      </div>
      <div className="ss-fld">
        <label>{t('mcp.piOptions')}</label>
        {/* With Tools set, toolExposure is refused, so the example must not look like a value that was accepted. */}
        <CodeEditor
          value={optionsText}
          onChange={onOptions}
          lang="json"
          placeholder={
            toolsSet
              ? '{\n  "timeout": 120\n}'
              : '{\n  "timeout": 120,\n  "toolExposure": {"delete_*": "hidden"}\n}'
          }
          ariaLabel={t('mcp.piOptions')}
          disabled={disabled}
          minHeight="96px"
        />
        {optionsError ? (
          <span className="hp !text-bad">{optionsError}</span>
        ) : (
          <span className="hp">{t('mcp.piOptionsHint')}</span>
        )}
      </div>
    </>
  );
}
