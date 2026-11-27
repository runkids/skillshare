import { ExternalLink } from 'lucide-react';
import { useT } from '../../i18n';

// Vendor-owned documentation; Skillshare's support limits are described separately.
const docs: Record<string, string> = {
  claude: 'https://code.claude.com/docs/en/discover-plugins',
  codex: 'https://developers.openai.com/codex/plugins/',
  cursor: 'https://prod.cursor.com/docs/plugins',
  antigravity: 'https://www.antigravity.google/docs/plugins',
  'antigravity-cli': 'https://www.antigravity.google/docs/plugins?tab=cli',
  pi: 'https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md',
  omp: 'https://github.com/can1357/oh-my-pi/blob/v18.6.1/docs/extensions.md',
  opencode: 'https://opencode.ai/docs/plugins/',
  copilot:
    'https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/plugins-finding-installing',
  grok: 'https://docs.x.ai/build/features/skills-plugins-marketplaces',
  kimi: 'https://www.kimi.com/code/docs/en/kimi-code-cli/customization/plugins',
  hermes: 'https://hermes-agent.nousresearch.com/docs/user-guide/features/plugins',
  devin: 'https://docs.devin.ai/cli/extensibility/plugins/overview',
};

export default function PluginDocsLink({
  target,
  label,
  iconOnly = false,
}: {
  target: string;
  label: string;
  /** Just the arrow, for a row whose reason already fills the line. */
  iconOnly?: boolean;
}) {
  const t = useT();
  if (!docs[target]) return null;
  return (
    <a
      href={docs[target]}
      target="_blank"
      rel="noopener noreferrer"
      className={
        iconOnly
          ? 'grid h-5 w-6 shrink-0 place-items-center text-ink-3 hover:text-ink'
          : 'inline-flex shrink-0 items-center gap-1 text-xs text-link underline'
      }
      aria-label={`${label} · ${t('plugins.officialDocs')}`}
      title={iconOnly ? t('plugins.officialDocs') : undefined}
    >
      {!iconOnly && t('plugins.officialDocs')}
      <ExternalLink size={iconOnly ? 14 : 12} aria-hidden="true" />
    </a>
  );
}
