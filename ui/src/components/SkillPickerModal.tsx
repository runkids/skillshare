import { useState, useEffect, useMemo } from 'react';
import { Download, Search } from 'lucide-react';
import Button from './Button';
import DialogShell from './DialogShell';
import { Input, Checkbox } from './Input';
import { radius } from '../design';
import type { DiscoveredSkill } from '../api/client';
import KindBadge from './KindBadge';
import { useT } from '../i18n';

interface SkillPickerModalProps {
  open: boolean;
  source: string;
  skills: DiscoveredSkill[];
  onInstall: (selected: DiscoveredSkill[]) => void;
  onCancel: () => void;
  installing: boolean;
  singleSelect?: boolean;
}

export default function SkillPickerModal({
  open,
  source,
  skills,
  onInstall,
  onCancel,
  installing,
  singleSelect,
}: SkillPickerModalProps) {
  const t = useT();
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

  // Reset selection when modal opens
  useEffect(() => {
    if (open) {
      setSelected(singleSelect ? new Set() : new Set(skills.map((s) => s.path)));
      setFilter('');
    }
  }, [open, skills, singleSelect]);

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
    if (singleSelect) {
      // Radio behavior: only one selected at a time
      setSelected(selected.has(path) ? new Set() : new Set([path]));
      return;
    }
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

  const allAgents = skills.length > 0 && skills.every((s) => s.kind === 'agent');
  const someAgents = skills.some((s) => s.kind === 'agent');
  const singularLabel = allAgents ? 'agent' : someAgents ? 'resource' : 'skill';
  const pluralLabel = allAgents ? 'agents' : someAgents ? 'resources' : 'skills';

  return (
    <DialogShell open={open} onClose={onCancel} maxWidth="2xl" preventClose={installing}>
          <h3 className="text-xl font-bold text-pencil mb-1">
            {singleSelect
              ? t('skillPicker.titleSingle', { singular: singularLabel[0].toUpperCase() + singularLabel.slice(1) })
              : t('skillPicker.titleMulti', { plural: pluralLabel[0].toUpperCase() + pluralLabel.slice(1) })
            }
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
                placeholder={t('skillPicker.filterPlaceholder', { plural: pluralLabel })}
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="!pl-8 !py-1.5 !text-sm font-mono"
              />
            </div>
          )}

          {/* Select All (multi-select only) */}
          {!singleSelect && (
            <div className="flex items-center justify-between border-b border-dashed border-pencil-light/30 pb-2 mb-2">
              <Checkbox
                label={allFilteredSelected ? t('skillPicker.deselectAll') : t('skillPicker.selectAll')}
                checked={allFilteredSelected}
                onChange={toggleAll}
              />
              {filter && (
                <span className="text-xs text-muted-dark">
                  {t('skillPicker.filterCount', { filtered: filtered.length, total: skills.length, plural: pluralLabel })}
                </span>
              )}
            </div>
          )}

          {/* Single-select hint */}
          {singleSelect && (
            <div className="border-b border-dashed border-pencil-light/30 pb-2 mb-2">
              <span className="text-xs text-muted-dark">
                {filter
                  ? t('skillPicker.selectOneHintWithFilter', { singular: singularLabel, filtered: filtered.length, total: skills.length })
                  : t('skillPicker.selectOneHint', { singular: singularLabel })}
              </span>
            </div>
          )}

          {/* Skill list */}
          <div className="overflow-y-auto space-y-1.5 mb-4" style={{ maxHeight: '22rem' }}>
            {filtered.map((skill) => {
              const isSelected = selected.has(skill.path);
              return (
                <label
                  key={skill.path}
                  className="flex items-start gap-2.5 py-2.5 px-2 rounded-md cursor-pointer hover:bg-muted/30 transition-colors"
                  style={{ borderRadius: radius.sm }}
                >
                  {singleSelect ? (
                    <button
                      type="button"
                      aria-label={`Select ${skill.name}`}
                      onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        toggle(skill.path);
                      }}
                      className={`mt-1 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-2 transition-colors focus:outline-none focus:ring-2 focus:ring-blue/20 ${
                        isSelected ? 'border-info bg-info' : 'border-muted-dark'
                      }`}
                    >
                      {isSelected && <span className="w-1.5 h-1.5 rounded-full bg-white" />}
                    </button>
                  ) : (
                    <span className="mt-1">
                      <Checkbox
                        label=""
                        checked={isSelected}
                        onChange={() => toggle(skill.path)}
                        size="sm"
                      />
                    </span>
                  )}
                  <div className="min-w-0 flex-1" onClick={singleSelect ? () => toggle(skill.path) : undefined}>
                    <span className="font-bold text-pencil text-base inline-flex items-center gap-1.5">
                      {skill.kind && <KindBadge kind={skill.kind} />}
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
              );
            })}
          </div>

          {/* Footer */}
          <div className="flex gap-3 justify-end border-t border-dashed border-pencil-light/30 pt-3">
            <Button
              variant="ghost"
              size="sm"
              onClick={onCancel}
              disabled={installing}
            >
              {t('skillPicker.cancelButton')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleInstall}
              disabled={installing || selected.size === 0}
              loading={installing}
            >
              <Download size={14} strokeWidth={2.5} />
              {singleSelect ? t('skillPicker.installButton') : t('skillPicker.installSelectedButton', { count: selected.size })}
            </Button>
          </div>
    </DialogShell>
  );
}
