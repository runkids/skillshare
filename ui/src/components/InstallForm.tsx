import { useState, useCallback } from 'react';
import { Download, Package, ChevronDown, ChevronUp, ShieldAlert, ShieldCheck } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import Card from './Card';
import Button from './Button';
import Badge from './Badge';
import { Input, Checkbox } from './Input';
import SkillPickerModal from './SkillPickerModal';
import ConfirmDialog from './ConfirmDialog';
import { useToast } from './Toast';
import { api, type InstallResult, type DiscoveredSkill } from '../api/client';
import { queryKeys } from '../lib/queryKeys';
import { radius } from '../design';

interface InstallFormProps {
  /** Called after a successful install with the result */
  onSuccess?: (result: InstallResult) => void;
  /** Whether the form starts expanded (default: false) */
  defaultOpen?: boolean;
  /** Whether to show the collapsible toggle header (default: true) */
  collapsible?: boolean;
  className?: string;
}

/** Parse audit error message into individual finding lines */
function parseAuditError(msg: string): string[] {
  return msg
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => /^(CRITICAL|HIGH|MEDIUM|LOW|INFO):/.test(l) || l.startsWith('"'));
}

/** Check if an error is an audit block */
function isAuditBlock(msg: string): boolean {
  return msg.includes('security audit failed');
}

/** Severity → badge variant + left-border color */
const severityStyle = (sev: string) => {
  if (sev === 'CRITICAL') return { variant: 'danger' as const, color: 'var(--color-danger)' };
  if (sev === 'HIGH') return { variant: 'warning' as const, color: 'var(--color-warning)' };
  return { variant: 'default' as const, color: 'var(--color-muted-dark)' };
};

interface GroupedFinding {
  skillName?: string;
  severity: string;
  description: string;
  snippets: string[];
}

/** Group flat audit lines into structured card data */
function groupAuditFindings(lines: string[]): GroupedFinding[] {
  const groups: GroupedFinding[] = [];
  let skill: string | undefined;
  for (const line of lines) {
    if (line.endsWith(':') && !/^(CRITICAL|HIGH|MEDIUM|LOW|INFO):/.test(line)) {
      skill = line.replace(/:$/, '');
    } else {
      const m = line.match(/^(CRITICAL|HIGH|MEDIUM|LOW|INFO):\s*(.*)/);
      if (m) groups.push({ skillName: skill, severity: m[1], description: m[2], snippets: [] });
      else if (line.startsWith('"') && groups.length > 0) groups[groups.length - 1].snippets.push(line);
    }
  }
  return groups;
}

/** Parse warning strings into structured card data */
function parseWarnings(warnings: string[]): GroupedFinding[] {
  return warnings.map((w) => {
    const lines = w.split('\n');
    const header = lines[0];
    const snippets = lines.slice(1).map((l) => l.trim()).filter(Boolean);
    const sm = header.match(/^(.+?):\s*(audit\s+.*)/);
    const skillName = sm ? sm[1] : undefined;
    const h = sm ? sm[2] : header;
    const severity = h.includes('CRITICAL') ? 'CRITICAL' : h.includes('HIGH') ? 'HIGH' : 'MEDIUM';
    const description = h.replace(/^audit\s+(HIGH|MEDIUM|CRITICAL|INFO):\s*/, '');
    return { skillName, severity, description, snippets };
  });
}

/** Saved install params for force-retry */
interface PendingInstall {
  type: 'single' | 'batch' | 'track';
  source: string;
  name?: string;
  into?: string;
  skills?: DiscoveredSkill[];
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
  const [into, setInto] = useState('');
  const [track, setTrack] = useState(false);
  const [force, setForce] = useState(false);
  const [skipAudit, setSkipAudit] = useState(false);
  const [branch, setBranch] = useState('');
  const [installing, setInstalling] = useState(false);
  const { toast } = useToast();
  const queryClient = useQueryClient();

  // Discovery flow state
  const [discoveredSkills, setDiscoveredSkills] = useState<DiscoveredSkill[]>([]);
  const [showPicker, setShowPicker] = useState(false);
  const [pendingSource, setPendingSource] = useState('');
  const [batchInstalling, setBatchInstalling] = useState(false);

  // Audit dialog state
  const [auditDialog, setAuditDialog] = useState<{
    findings: string[];
    pending: PendingInstall;
  } | null>(null);
  const [auditForcing, setAuditForcing] = useState(false);
  const [warningDialog, setWarningDialog] = useState<string[] | null>(null);
  const [severityFilter, setSeverityFilter] = useState<string | null>(null);

  const isGitSource = useCallback((s: string) => {
    const trimmed = s.trim();
    if (!trimmed) return false;
    if (/^[/~.]/.test(trimmed) || /^[a-zA-Z]:\\/.test(trimmed)) return false;
    return trimmed.includes('/') || trimmed.startsWith('git@') || trimmed.includes('://');
  }, []);

  const resetForm = () => {
    setName('');
    setInto('');
    setTrack(false);
    setForce(false);
    setSkipAudit(false);
    setBranch('');
    if (collapsible) setOpen(false);
  };

  const invalidateAfterInstall = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    queryClient.invalidateQueries({ queryKey: queryKeys.overview });
  };

  /** Handle install result: show warning dialog if warnings exist, otherwise just toast */
  const handleResult = useCallback(
    (res: InstallResult, label?: string) => {
      const prefix = label ? `${label}: ` : '';
      toast(`${prefix}Installed (${res.action})`, 'success');
      if (res.warnings && res.warnings.length > 0) {
        setWarningDialog(res.warnings);
      }
      resetForm();
      invalidateAfterInstall();
      onSuccess?.(res);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [onSuccess, toast],
  );

  /** Handle error: if audit block, show confirm dialog; otherwise error toast */
  const handleError = useCallback(
    (e: unknown, pending: PendingInstall) => {
      const msg = (e as Error).message;
      if (isAuditBlock(msg)) {
        setAuditDialog({ findings: parseAuditError(msg), pending });
      } else {
        toast(msg, 'error');
      }
    },
    [toast],
  );

  /** Force-install after audit confirm */
  const handleAuditForce = async () => {
    if (!auditDialog) return;
    const { pending } = auditDialog;
    setAuditForcing(true);
    try {
      if (pending.type === 'track') {
        const res = await api.install({
          source: pending.source,
          name: pending.name,
          into: pending.into,
          track: true,
          force: true,
          skipAudit,
          branch: branch.trim() || undefined,
        });
        handleResult(res, res.skillName ?? res.repoName);
      } else if (pending.type === 'batch') {
        const res = await api.installBatch({
          source: pending.source,
          skills: pending.skills!,
          into: pending.into,
          force: true,
          skipAudit,
          branch: branch.trim() || undefined,
        });
        toast(res.summary, 'success');
        const allWarnings: string[] = [];
        for (const item of res.results) {
          if (item.error) toast(`${item.name}: ${item.error}`, 'error');
          if (item.warnings?.length) allWarnings.push(...item.warnings.map((w) => `${item.name}: ${w}`));
        }
        if (allWarnings.length > 0) setWarningDialog(allWarnings);
        resetForm();
        invalidateAfterInstall();
        onSuccess?.({ action: 'installed', warnings: [], skillName: res.summary });
      } else {
        const res = await api.install({
          source: pending.source,
          name: pending.name,
          into: pending.into,
          force: true,
          skipAudit,
          branch: branch.trim() || undefined,
        });
        handleResult(res, res.skillName ?? res.repoName);
      }
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setAuditForcing(false);
      setAuditDialog(null);
      setSeverityFilter(null);
    }
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
          into: into.trim() || undefined,
          track: true,
          force,
          skipAudit,
          branch: branch.trim() || undefined,
        });
        handleResult(res, res.skillName ?? res.repoName);
      } catch (e: unknown) {
        handleError(e, { type: 'track', source: trimmed, name: name.trim() || undefined, into: into.trim() || undefined });
      } finally {
        setInstalling(false);
      }
      return;
    }

    // Discovery flow
    setInstalling(true);
    try {
      const disc = await api.discover(trimmed, branch.trim() || undefined);
      if (disc.skills.length > 1) {
        // Multiple skills found — open picker
        setDiscoveredSkills(disc.skills);
        setPendingSource(trimmed);
        setShowPicker(true);
      } else if (disc.skills.length === 1) {
        // Single discovered skill — install via batch
        const res = await api.installBatch({
          source: trimmed,
          skills: disc.skills,
          into: into.trim() || undefined,
          force,
          skipAudit,
          branch: branch.trim() || undefined,
        });
        const allWarnings: string[] = [];
        const auditFindings: string[] = [];
        const auditBlockedSkills: DiscoveredSkill[] = [];
        let installed = 0;
        for (const item of res.results) {
          if (item.error) {
            if (isAuditBlock(item.error)) {
              auditFindings.push(`${item.name}:`, ...parseAuditError(item.error));
              const skill = disc.skills.find((s) => s.name === item.name);
              if (skill) auditBlockedSkills.push(skill);
            } else {
              toast(`${item.name}: ${item.error}`, 'error');
            }
          } else {
            installed++;
          }
          if (item.warnings?.length) allWarnings.push(...item.warnings.map((w) => `${item.name}: ${w}`));
        }
        if (installed > 0) {
          const variant = auditBlockedSkills.length > 0 ? 'warning' : 'success';
          toast(res.summary, variant as 'success' | 'warning');
          if (allWarnings.length > 0) setWarningDialog(allWarnings);
          resetForm();
          invalidateAfterInstall();
          onSuccess?.({ action: 'installed', warnings: [], skillName: res.summary });
        }
        if (auditBlockedSkills.length > 0) {
          setAuditDialog({
            findings: auditFindings,
            pending: { type: 'batch', source: trimmed, skills: auditBlockedSkills },
          });
        }
      } else {
        // No skills discovered — direct install
        const res = await api.install({
          source: trimmed,
          name: name.trim() || undefined,
          into: into.trim() || undefined,
          force,
          skipAudit,
          branch: branch.trim() || undefined,
        });
        handleResult(res, res.skillName ?? res.repoName);
      }
    } catch (e: unknown) {
      handleError(e, { type: 'single', source: trimmed, name: name.trim() || undefined, into: into.trim() || undefined });
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
        into: into.trim() || undefined,
        force,
        skipAudit,
        name: selected.length === 1 && name.trim() ? name.trim() : undefined,
        branch: branch.trim() || undefined,
      });
      const allWarnings: string[] = [];
      const auditFindings: string[] = [];
      const auditBlockedSkills: DiscoveredSkill[] = [];
      let installed = 0;
      for (const item of res.results) {
        if (item.error) {
          if (isAuditBlock(item.error)) {
            auditFindings.push(`${item.name}:`, ...parseAuditError(item.error));
            const skill = selected.find((s) => s.name === item.name);
            if (skill) auditBlockedSkills.push(skill);
          } else {
            toast(`${item.name}: ${item.error}`, 'error');
          }
        } else {
          installed++;
        }
        if (item.warnings?.length) allWarnings.push(...item.warnings.map((w) => `${item.name}: ${w}`));
      }

      // Only show summary + close picker when at least one skill installed
      if (installed > 0) {
        const variant = auditBlockedSkills.length > 0 ? 'warning' : 'success';
        toast(res.summary, variant as 'success' | 'warning');
        if (allWarnings.length > 0) setWarningDialog(allWarnings);
        setShowPicker(false);
        resetForm();
        invalidateAfterInstall();
        onSuccess?.({ action: 'installed', warnings: [], skillName: res.summary });
      }
      // Show audit dialog for blocked items only (force-retry targets just those)
      if (auditBlockedSkills.length > 0) {
        setAuditDialog({
          findings: auditFindings,
          pending: { type: 'batch', source: pendingSource, skills: auditBlockedSkills },
        });
      }
    } catch (e: unknown) {
      handleError(e, { type: 'batch', source: pendingSource, into: into.trim() || undefined, skills: selected });
    } finally {
      setBatchInstalling(false);
    }
  };

  // Pre-compute grouped findings for card-based rendering
  const auditFindings = auditDialog ? groupAuditFindings(auditDialog.findings) : [];
  const auditCounts: Record<string, number> = {};
  for (const f of auditFindings) auditCounts[f.severity] = (auditCounts[f.severity] ?? 0) + 1;
  const warningFindings = warningDialog ? parseWarnings(warningDialog) : [];
  const warningCounts: Record<string, number> = {};
  for (const f of warningFindings) warningCounts[f.severity] = (warningCounts[f.severity] ?? 0) + 1;

  const filteredAudit = severityFilter ? auditFindings.filter((f) => f.severity === severityFilter) : auditFindings;
  const filteredWarnings = severityFilter ? warningFindings.filter((f) => f.severity === severityFilter) : warningFindings;

  const filterBadge = (severity: string, count: number | undefined, variant: 'danger' | 'warning' | 'default', label: string) =>
    count ? (
      <button
        onClick={() => setSeverityFilter(severityFilter === severity ? null : severity)}
        className={`cursor-pointer transition-opacity ${severityFilter && severityFilter !== severity ? 'opacity-40' : ''}`}
        style={{ background: 'none', border: 'none', padding: 0 }}
      >
        <Badge variant={variant}>{count} {label}</Badge>
      </button>
    ) : null;

  const formContent = (
    <Card variant="default" className="animate-fade-in">
      <div className="space-y-5">
        {/* Source + Branch — inline when git source detected */}
        <div>
          <div className={isGitSource(source) ? 'grid grid-cols-[7fr_3fr] gap-3' : ''}>
            <Input
              label="Source"
              type="text"
              placeholder="owner/repo, git URL, or local path"
              value={source}
              onChange={(e) => setSource(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleInstall()}
            />
            {isGitSource(source) && (
              <Input
                label="Branch"
                type="text"
                placeholder="default"
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleInstall()}
              />
            )}
          </div>
          <p className="text-xs text-muted-dark mt-1.5">
            e.g. <span className="font-mono">owner/repo</span>, <span className="font-mono">https://github.com/…</span>, or <span className="font-mono">~/local/path</span>
            {isGitSource(source) && <> · Leave branch empty for remote default</>}
          </p>
        </div>

        {/* Optional overrides */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <Input
              label="Custom name"
              type="text"
              placeholder="my-skill"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <p className="text-xs text-muted-dark mt-1">Only applies to single skill install</p>
          </div>
          <Input
            label="Into directory"
            type="text"
            placeholder="frontend/react"
            value={into}
            onChange={(e) => setInto(e.target.value)}
          />
        </div>

        {/* Options */}
        <div className="border-t border-dashed border-pencil-light/30 pt-4">
          <div className="flex items-center gap-6 flex-wrap">
            <Checkbox
              label="Track"
              checked={track}
              onChange={setTrack}
            />
            <Checkbox
              label="Force overwrite"
              checked={force}
              onChange={setForce}
            />
            <Checkbox
              label="Skip audit"
              checked={skipAudit}
              onChange={setSkipAudit}
            />
          </div>
          <p className="text-xs text-muted-dark mt-2">
            Track keeps the git repo linked for updates · Force overwrites existing skills · Skip audit bypasses security scan
          </p>
        </div>

        {/* Install button */}
        <Button
          onClick={handleInstall}
          disabled={installing || !source.trim()}
          variant="primary"
          size="md"
          loading={installing}
        >
          <Download size={16} strokeWidth={2.5} />
          Install
        </Button>
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
      singleSelect={!!name.trim()}
    />
  );

  const auditConfirmDialog = (
    <ConfirmDialog
      open={!!auditDialog}
      variant="danger"
      wide
      title="Security Threats Detected"
      message={
        <div className="text-left space-y-3">
          <div className="flex items-center gap-2 justify-center mb-1">
            <ShieldAlert size={20} className="text-danger" />
            <span>Critical issues found during security audit</span>
          </div>
          <div className="flex flex-wrap gap-1.5 items-center text-xs text-pencil-light">
            <span>{auditFindings.length} {auditFindings.length === 1 ? 'finding' : 'findings'}:</span>
            {filterBadge('CRITICAL', auditCounts.CRITICAL, 'danger', 'Critical')}
            {filterBadge('HIGH', auditCounts.HIGH, 'warning', 'High')}
            {filterBadge('MEDIUM', auditCounts.MEDIUM, 'default', 'Medium')}
            {severityFilter && (
              <span className="text-pencil-light">— showing {filteredAudit.length}</span>
            )}
          </div>
          <div className="space-y-2 max-h-[32rem] overflow-y-auto pr-1">
            {filteredAudit.map((f, i) => {
              const s = severityStyle(f.severity);
              return (
                <div
                  key={i}
                  className="border border-muted bg-paper p-3 space-y-1.5"
                  style={{ borderRadius: radius.md }}
                >
                  {f.skillName && <div className="text-xs text-pencil-light font-mono">{f.skillName}</div>}
                  <div className="flex items-center gap-2 flex-wrap">
                    <span
                      className="w-2 h-2 rounded-full shrink-0"
                      style={{ backgroundColor: s.color }}
                    />
                    <Badge variant={s.variant}>{f.severity}</Badge>
                    <span className="text-sm leading-relaxed">{f.description}</span>
                  </div>
                  {f.snippets.length > 0 && (
                    <div className="ml-1 pl-3 space-y-0.5">
                      {f.snippets.map((sn, j) => (
                        <div key={j} className="text-xs text-pencil-light font-mono break-all">{sn}</div>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
          <p className="text-xs text-pencil-light">
            Force install will bypass the security check. Proceed with caution.
          </p>
        </div>
      }
      confirmText="Force Install"
      cancelText="Cancel"
      onConfirm={handleAuditForce}
      onCancel={() => { setAuditDialog(null); setSeverityFilter(null); }}
      loading={auditForcing}
    />
  );

  const warningConfirmDialog = (
    <ConfirmDialog
      open={!!warningDialog}
      variant="default"
      wide
      title="Security Warnings"
      message={
        <div className="text-left space-y-3">
          <div className="flex items-center gap-2 mb-4">
            <ShieldCheck size={20} className="text-warning" />
            <span>Skill installed with audit warnings</span>
          </div>
          <div className="flex flex-wrap gap-1.5 items-center text-xs text-pencil-light">
            <span>{warningFindings.length} {warningFindings.length === 1 ? 'warning' : 'warnings'}:</span>
            {filterBadge('CRITICAL', warningCounts.CRITICAL, 'danger', 'Critical')}
            {filterBadge('HIGH', warningCounts.HIGH, 'warning', 'High')}
            {filterBadge('MEDIUM', warningCounts.MEDIUM, 'default', 'Medium')}
            {severityFilter && (
              <span className="text-pencil-light">— showing {filteredWarnings.length}</span>
            )}
          </div>
          <div className="space-y-2 max-h-128 overflow-y-auto pr-1">
            {filteredWarnings.map((f, i) => {
              const s = severityStyle(f.severity);
              return (
                <div
                  key={i}
                  className="border border-muted bg-paper p-3 space-y-1.5"
                  style={{ borderRadius: radius.md }}
                >
                  {f.skillName && <div className="text-xs text-pencil-light font-mono">{f.skillName}</div>}
                  <div className="flex items-center gap-2 flex-wrap">
                    <span
                      className="w-2 h-2 rounded-full shrink-0"
                      style={{ backgroundColor: s.color }}
                    />
                    <Badge variant={s.variant}>{f.severity}</Badge>
                    <span className="text-sm leading-relaxed">{f.description}</span>
                  </div>
                  {f.snippets.length > 0 && (
                    <div className="ml-1 pl-3 space-y-0.5">
                      {f.snippets.map((sn, j) => (
                        <div key={j} className="text-xs text-pencil-light font-mono break-all">{sn}</div>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      }
      confirmText="OK"
      cancelText=""
      onConfirm={() => { setWarningDialog(null); setSeverityFilter(null); }}
      onCancel={() => { setWarningDialog(null); setSeverityFilter(null); }}
    />
  );

  if (!collapsible) {
    return (
      <div className={className}>
        {formContent}
        {pickerModal}
        {auditConfirmDialog}
        {warningConfirmDialog}
      </div>
    );
  }

  return (
    <div className={className}>
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 text-pencil-light hover:text-pencil transition-colors cursor-pointer mb-3"
        style={{
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
      {auditConfirmDialog}
      {warningConfirmDialog}
    </div>
  );
}
