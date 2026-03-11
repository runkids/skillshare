import { Download, GitBranch, FolderOpen, Globe } from 'lucide-react';
import InstallForm from '../components/InstallForm';
import PageHeader from '../components/PageHeader';

const EXAMPLES = [
  { icon: Globe, label: 'owner/repo', desc: 'GitHub shorthand' },
  { icon: GitBranch, label: 'https://github.com/…', desc: 'Any git URL' },
  { icon: GitBranch, label: 'git@host:org/repo', desc: 'SSH (private repos)' },
  { icon: FolderOpen, label: '~/local/path', desc: 'Local directory' },
];

export default function InstallPage() {
  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<Download size={24} strokeWidth={2.5} />}
        title="Install Skill"
        subtitle="Install from any git repository or local path"
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
