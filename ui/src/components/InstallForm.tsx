import { useState } from 'react';
import { Download, Package, ChevronDown, ChevronUp } from 'lucide-react';
import Card from './Card';
import HandButton from './HandButton';
import { HandInput, HandCheckbox } from './HandInput';
import SkillPickerModal from './SkillPickerModal';
import { useToast } from './Toast';
import { api, type InstallResult, type DiscoveredSkill } from '../api/client';

interface InstallFormProps {
  /** Called after a successful install with the result */
  onSuccess?: (result: InstallResult) => void;
  /** Whether the form starts expanded (default: false) */
  defaultOpen?: boolean;
  /** Whether to show the collapsible toggle header (default: true) */
  collapsible?: boolean;
  className?: string;
}

export default function InstallForm({
  onSuccess,
  defaultOpen = false,
  collapsible = true,
  className = '',
}: InstallFormProps) {
  const [open, setOpen] = useState(defaultOpen);
  const [source, setSource] = useState('');
  const [name, setName] = useState('');
  const [track, setTrack] = useState(false);
  const [force, setForce] = useState(false);
  const [installing, setInstalling] = useState(false);
  const { toast } = useToast();

  // Discovery flow state
  const [discoveredSkills, setDiscoveredSkills] = useState<DiscoveredSkill[]>([]);
  const [showPicker, setShowPicker] = useState(false);
  const [pendingSource, setPendingSource] = useState('');
  const [batchInstalling, setBatchInstalling] = useState(false);

  const resetForm = () => {
    setSource('');
    setName('');
    setTrack(false);
    setForce(false);
    if (collapsible) setOpen(false);
  };

  const handleInstall = async () => {
    if (!source.trim()) return;
    const trimmed = source.trim();

    // Track mode → direct install (no discovery needed)
    if (track) {
      setInstalling(true);
      try {
        const res = await api.install({
          source: trimmed,
          name: name.trim() || undefined,
          track: true,
          force,
        });
        toast(
          `Installed: ${res.skillName ?? res.repoName} (${res.action})`,
          'success',
        );
        if (res.warnings?.length > 0) {
          res.warnings.forEach((w) => toast(w, 'warning'));
        }
        resetForm();
        onSuccess?.(res);
      } catch (e: unknown) {
        toast((e as Error).message, 'error');
      } finally {
        setInstalling(false);
      }
      return;
    }

    // Discovery flow
    setInstalling(true);
    try {
      const disc = await api.discover(trimmed);
      if (disc.skills.length > 1) {
        // Multiple skills found — open picker
        setDiscoveredSkills(disc.skills);
        setPendingSource(trimmed);
        setShowPicker(true);
      } else if (disc.skills.length === 1) {
        // Single discovered skill — install via batch (uses InstallFromDiscovery)
        const res = await api.installBatch({
          source: trimmed,
          skills: disc.skills,
          force,
        });
        toast(res.summary, 'success');
        for (const item of res.results) {
          if (item.error) {
            toast(`${item.name}: ${item.error}`, 'error');
          }
          if (item.warnings?.length) {
            item.warnings.forEach((w) => toast(`${item.name}: ${w}`, 'warning'));
          }
        }
        resetForm();
        onSuccess?.({ action: 'installed', warnings: [], skillName: res.summary });
      } else {
        // No skills discovered (discovery failed or non-git) — direct install
        const res = await api.install({
          source: trimmed,
          name: name.trim() || undefined,
          force,
        });
        toast(
          `Installed: ${res.skillName ?? res.repoName} (${res.action})`,
          'success',
        );
        if (res.warnings?.length > 0) {
          res.warnings.forEach((w) => toast(w, 'warning'));
        }
        resetForm();
        onSuccess?.(res);
      }
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setInstalling(false);
    }
  };

  const handleBatchInstall = async (selected: DiscoveredSkill[]) => {
    setBatchInstalling(true);
    try {
      const res = await api.installBatch({
        source: pendingSource,
        skills: selected,
        force,
      });
      toast(res.summary, 'success');
      for (const item of res.results) {
        if (item.error) {
          toast(`${item.name}: ${item.error}`, 'error');
        }
        if (item.warnings?.length) {
          item.warnings.forEach((w) => toast(`${item.name}: ${w}`, 'warning'));
        }
      }
      setShowPicker(false);
      resetForm();
      // Notify parent with a synthetic result
      onSuccess?.({ action: 'installed', warnings: [], skillName: res.summary });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setBatchInstalling(false);
    }
  };

  const formContent = (
    <Card variant="postit" className="animate-sketch-in">
      <div className="space-y-4">
        <HandInput
          label="Source (GitHub URL, owner/repo, or local path)"
          type="text"
          placeholder="owner/repo or https://github.com/..."
          value={source}
          onChange={(e) => setSource(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleInstall()}
        />
        <HandInput
          label="Name override (optional)"
          type="text"
          placeholder="custom-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <div className="flex items-center gap-6">
          <HandCheckbox
            label="Track (git repo)"
            checked={track}
            onChange={setTrack}
          />
          <HandCheckbox
            label="Force overwrite"
            checked={force}
            onChange={setForce}
          />
        </div>
        <HandButton
          onClick={handleInstall}
          disabled={installing || !source.trim()}
          variant="primary"
          size="sm"
        >
          <Download size={14} strokeWidth={2.5} />
          {installing ? 'Installing...' : 'Install'}
        </HandButton>
      </div>
    </Card>
  );

  const pickerModal = (
    <SkillPickerModal
      open={showPicker}
      source={pendingSource}
      skills={discoveredSkills}
      onInstall={handleBatchInstall}
      onCancel={() => setShowPicker(false)}
      installing={batchInstalling}
    />
  );

  if (!collapsible) {
    return (
      <div className={className}>
        {formContent}
        {pickerModal}
      </div>
    );
  }

  return (
    <div className={className}>
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 text-pencil-light hover:text-pencil transition-colors cursor-pointer mb-3"
        style={{
          fontFamily: 'var(--font-hand)',
          background: 'none',
          border: 'none',
          padding: 0,
        }}
      >
        <Package size={16} strokeWidth={2.5} />
        <span className="text-base">Install from URL / Path</span>
        {open ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
      </button>
      {open && formContent}
      {pickerModal}
    </div>
  );
}
