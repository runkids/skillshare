import InstallForm from '../components/InstallForm';

export default function InstallPage() {
  return (
    <div className="animate-sketch-in">
      <div className="mb-6">
        <h2
          className="text-3xl md:text-4xl font-bold text-pencil mb-2"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Install Skill
        </h2>
        <p className="text-pencil-light">
          Install a skill from a GitHub URL, owner/repo, or local path
        </p>
      </div>

      <InstallForm collapsible={false} defaultOpen />
    </div>
  );
}
