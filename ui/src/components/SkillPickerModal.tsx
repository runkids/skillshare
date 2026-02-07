import { useState, useEffect } from 'react';
import { Download } from 'lucide-react';
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

  // Select all by default when modal opens
  useEffect(() => {
    if (open) {
      setSelected(new Set(skills.map((s) => s.path)));
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

  const allSelected = selected.size === skills.length;

  const toggleAll = () => {
    if (allSelected) {
      setSelected(new Set());
    } else {
      setSelected(new Set(skills.map((s) => s.path)));
    }
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
        <Card decoration="tape">
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

          {/* Select All */}
          <div className="border-b-2 border-dashed border-muted pb-2 mb-2">
            <HandCheckbox
              label={allSelected ? 'Deselect All' : 'Select All'}
              checked={allSelected}
              onChange={toggleAll}
            />
          </div>

          {/* Skill list */}
          <div className="max-h-64 overflow-y-auto space-y-1 mb-4">
            {skills.map((skill) => (
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
