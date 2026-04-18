import { Download, GitBranch, FolderOpen, Globe } from 'lucide-react';
import InstallForm from '../components/InstallForm';
import PageHeader from '../components/PageHeader';
import { useT } from '../i18n';

export default function InstallPage() {
  const t = useT();

  const EXAMPLES = [
    { icon: Globe, label: 'owner/repo', desc: t('install.example.githubShorthand') },
    { icon: GitBranch, label: 'https://github.com/…', desc: t('install.example.anyGitUrl') },
    { icon: GitBranch, label: 'git@host:org/repo', desc: t('install.example.sshPrivateRepos') },
    { icon: FolderOpen, label: '~/local/path', desc: t('install.example.localDirectory') },
  ];

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<Download size={24} strokeWidth={2.5} />}
        title={t('install.title')}
        subtitle={t('install.subtitle')}
      />

      <div data-tour="install-form">
        <InstallForm collapsible={false} defaultOpen />
      </div>

      {/* Quick reference */}
      <div className="flex flex-wrap gap-4">
        {EXAMPLES.map(({ icon: Icon, label, desc }) => (
          <div
            key={label}
            className="flex items-center gap-2 text-sm text-pencil-light"
          >
            <Icon size={14} strokeWidth={2} className="text-muted-dark shrink-0" />
            <span className="font-mono text-xs">{label}</span>
            <span className="text-muted-dark">— {desc}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
