import { useState, useEffect, useMemo } from 'react';
import { Download, Search } from 'lucide-react';
import Card from './Card';
import Button from './Button';
import DialogShell from './DialogShell';
import { Input, Checkbox } from './Input';
import { radius } from '../design';
import type { DiscoveredSkill } from '../api/client';

interface SkillPickerModalProps {
  open: boolean;
  source: string;
  skills: DiscoveredSkill[];
  onInstall: (selected: DiscoveredSkill[]) => void;
  onCancel: () => void;
  installing: boolean;
}

export default function SkillPickerModal({
  open,
  source,
  skills,
  onInstall,
  onCancel,
  installing,
}: SkillPickerModalProps) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState('');

  const filtered = useMemo(() => {
    if (!filter) return skills;
    const q = filter.toLowerCase();
    return skills.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.path.toLowerCase().includes(q) ||
        (s.description?.toLowerCase().includes(q) ?? false),
    );
  }, [skills, filter]);

  // Select all by default when modal opens; reset filter
  useEffect(() => {
    if (open) {
      setSelected(new Set(skills.map((s) => s.path)));
      setFilter('');
    }
  }, [open, skills]);

  if (!open) return null;

  const filteredPaths = new Set(filtered.map((s) => s.path));
  const allFilteredSelected = filtered.length > 0 && filtered.every((s) => selected.has(s.path));

  const toggleAll = () => {
    const next = new Set(selected);
    if (allFilteredSelected) {
      for (const p of filteredPaths) next.delete(p);
    } else {
      for (const p of filteredPaths) next.add(p);
    }
    setSelected(next);
  };

  const toggle = (path: string) => {
    const next = new Set(selected);
    if (next.has(path)) {
      next.delete(path);
    } else {
      next.add(path);
    }
    setSelected(next);
  };

  const handleInstall = () => {
    const items = skills.filter((s) => selected.has(s.path));
    if (items.length > 0) onInstall(items);
  };

  return (
    <DialogShell open={open} onClose={onCancel} maxWidth="md" preventClose={installing}>
      <Card className="!overflow-clip">
          <h3 className="text-xl font-bold text-pencil mb-1">
            Select Skills to Install
          </h3>
          <p className="text-sm text-pencil-light mb-4 truncate font-mono">
            {source}
          </p>

          {/* Filter */}
          {skills.length > 5 && (
            <div className="relative mb-3">
              <Search
                size={14}
                strokeWidth={2.5}
                className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
              />
              <Input
                type="text"
                placeholder="Filter skills..."
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="!pl-8 !py-1.5 !text-sm font-mono"
              />
            </div>
          )}

          {/* Select All */}
          <div className="flex items-center justify-between border-b border-dashed border-pencil-light/30 pb-2 mb-2">
            <Checkbox
              label={allFilteredSelected ? 'Deselect All' : 'Select All'}
              checked={allFilteredSelected}
              onChange={toggleAll}
            />
            {filter && (
              <span className="text-xs text-muted-dark">
                {filtered.length} of {skills.length} skills
              </span>
            )}
          </div>

          {/* Skill list */}
          <div className="overflow-y-auto space-y-1 mb-4" style={{ maxHeight: '16rem' }}>
            {filtered.map((skill) => (
              <label
                key={skill.path}
                className="flex items-start gap-2 py-1.5 px-1 rounded-md cursor-pointer hover:bg-muted/30 transition-colors"
                style={{ borderRadius: radius.sm }}
              >
                <Checkbox
                  label=""
                  checked={selected.has(skill.path)}
                  onChange={() => toggle(skill.path)}
                  size="sm"
                />
                <div className="min-w-0 flex-1">
                  <span className="font-bold text-pencil text-base">
                    {skill.name}
                  </span>
                  {skill.path !== '.' && skill.path !== skill.name && (
                    <span className="block text-xs text-muted-dark truncate font-mono">
                      {skill.path}
                    </span>
                  )}
                  {skill.description && (
                    <span className="block text-sm text-pencil-light truncate">
                      {skill.description}
                    </span>
                  )}
                </div>
              </label>
            ))}
          </div>

          {/* Footer */}
          <div className="flex gap-3 justify-end border-t border-dashed border-pencil-light/30 pt-3">
            <Button
              variant="ghost"
              size="sm"
              onClick={onCancel}
              disabled={installing}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleInstall}
              disabled={installing || selected.size === 0}
              loading={installing}
            >
              <Download size={14} strokeWidth={2.5} />
              Install Selected ({selected.size})
            </Button>
          </div>
        </Card>
    </DialogShell>
  );
}
