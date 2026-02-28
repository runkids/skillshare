import { useState, useEffect, useRef, useMemo } from 'react';
import { Download, Search } from 'lucide-react';
import Card from './Card';
import HandButton from './HandButton';
import { HandCheckbox } from './HandInput';
import { wobbly } from '../design';
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
  const filterRef = useRef<HTMLInputElement>(null);

  const filtered = useMemo(() => {
    if (!filter) return skills;
    const q = filter.toLowerCase();
    return skills.filter(
      (s) => s.name.toLowerCase().includes(q) || s.path.toLowerCase().includes(q),
    );
  }, [skills, filter]);

  // Select all by default when modal opens; reset filter
  useEffect(() => {
    if (open) {
      setSelected(new Set(skills.map((s) => s.path)));
      setFilter('');
      setTimeout(() => filterRef.current?.focus(), 0);
    }
  }, [open, skills]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !installing) onCancel();
    };
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [open, installing, onCancel]);

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
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget && !installing) onCancel();
      }}
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-pencil/30" />

      {/* Dialog */}
      <div
        className="relative w-full max-w-md animate-sketch-in"
        style={{ borderRadius: wobbly.md }}
      >
        <Card decoration="tape" className="!overflow-clip">
          <h3
            className="text-xl font-bold text-pencil mb-1"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Select Skills to Install
          </h3>
          <p
            className="text-sm text-pencil-light mb-4 truncate"
            style={{ fontFamily: "'Courier New', monospace" }}
          >
            {source}
          </p>

          {/* Filter */}
          {skills.length > 5 && (
            <div className="relative mb-3">
              <Search
                size={14}
                className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
              />
              <input
                ref={filterRef}
                type="text"
                placeholder="Filter skills..."
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="w-full pl-8 pr-3 py-1.5 text-sm border-2 border-muted bg-paper text-pencil placeholder:text-muted-dark outline-none focus:border-pencil-light"
                style={{
                  borderRadius: wobbly.sm,
                  fontFamily: "'Courier New', monospace",
                }}
              />
            </div>
          )}

          {/* Select All */}
          <div className="flex items-center justify-between border-b-2 border-dashed border-muted pb-2 mb-2">
            <HandCheckbox
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
              <div key={skill.path} className="flex items-start gap-2 py-1">
                <HandCheckbox
                  label=""
                  checked={selected.has(skill.path)}
                  onChange={() => toggle(skill.path)}
                />
                <div className="min-w-0">
                  <span
                    className="font-bold text-pencil text-base"
                    style={{ fontFamily: 'var(--font-heading)' }}
                  >
                    {skill.name}
                  </span>
                  {skill.path !== '.' && skill.path !== skill.name && (
                    <span
                      className="block text-xs text-muted-dark truncate"
                      style={{ fontFamily: "'Courier New', monospace" }}
                    >
                      {skill.path}
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>

          {/* Footer */}
          <div className="flex gap-3 justify-end">
            <HandButton
              variant="ghost"
              size="sm"
              onClick={onCancel}
              disabled={installing}
            >
              Cancel
            </HandButton>
            <HandButton
              variant="primary"
              size="sm"
              onClick={handleInstall}
              disabled={installing || selected.size === 0}
            >
              <Download size={14} strokeWidth={2.5} />
              {installing
                ? 'Installing...'
                : `Install Selected (${selected.size})`}
            </HandButton>
          </div>
        </Card>
      </div>
    </div>
  );
}
