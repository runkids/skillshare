import { Download } from 'lucide-react';
import InstallForm from '../components/InstallForm';
import PageHeader from '../components/PageHeader';

export default function InstallPage() {
  return (
    <div className="animate-fade-in">
      <PageHeader icon={<Download size={24} strokeWidth={2.5} />} title="Install Skill" subtitle="Install a skill from a GitHub URL, owner/repo, or local path" />

      <InstallForm collapsible={false} defaultOpen />
    </div>
  );
}
