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
  opencode: 'https://opencode.ai/docs/plugins/',
  copilot: 'https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/plugins-finding-installing',
  grok: 'https://docs.x.ai/build/features/skills-plugins-marketplaces',
  kimi: 'https://www.kimi.com/code/docs/en/kimi-code-cli/customization/plugins',
  hermes: 'https://hermes-agent.nousresearch.com/docs/user-guide/features/plugins',
  devin: 'https://docs.devin.ai/cli/extensibility/plugins/overview',
};

export default function PluginDocsLink({ target, label }: { target: string; label: string }) {
  const t = useT();
  if (!docs[target]) return null;
  return <a href={docs[target]} target="_blank" rel="noopener noreferrer" className="inline-flex shrink-0 items-center gap-1 text-xs text-link underline" aria-label={`${label} · ${t('plugins.officialDocs')}`}>
    {t('plugins.officialDocs')}<ExternalLink size={12} aria-hidden="true" />
  </a>;
}
