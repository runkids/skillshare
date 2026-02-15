import { useState, useRef } from 'react';
import { X } from 'lucide-react';
import { wobbly } from '../design';

interface FilterTagInputProps {
  label: string;
  patterns: string[];
  onChange: (patterns: string[]) => void;
  color: 'blue' | 'danger';
}

export default function FilterTagInput({ label, patterns, onChange, color }: FilterTagInputProps) {
  const [input, setInput] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  const colorClasses = color === 'blue'
    ? { chip: 'bg-info-light text-blue border-blue', ring: 'focus:border-blue focus:ring-blue/20' }
    : { chip: 'bg-danger-light text-danger border-danger', ring: 'focus:border-danger focus:ring-danger/20' };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && input.trim()) {
      e.preventDefault();
      const val = input.trim();
      if (!patterns.includes(val)) {
        onChange([...patterns, val]);
      }
      setInput('');
    } else if (e.key === 'Backspace' && !input && patterns.length > 0) {
      onChange(patterns.slice(0, -1));
    }
  };

  const removePattern = (idx: number) => {
    onChange(patterns.filter((_, i) => i !== idx));
  };

  return (
    <div>
      <label
        className="block text-sm font-bold text-pencil-light mb-1"
        style={{ fontFamily: 'var(--font-hand)' }}
      >
        {label}
      </label>
      <div
        className="flex flex-wrap items-center gap-1.5 p-2 bg-white border-2 border-pencil min-h-[2.5rem] cursor-text"
        style={{ borderRadius: wobbly.sm }}
        onClick={() => inputRef.current?.focus()}
      >
        {patterns.map((p, i) => (
          <span
            key={p + i}
            className={`inline-flex items-center gap-1 text-xs font-bold px-2 py-0.5 border ${colorClasses.chip}`}
            style={{ borderRadius: wobbly.sm }}
          >
            {p}
            <button
              onClick={(e) => {
                e.stopPropagation();
                removePattern(i);
              }}
              className="hover:opacity-70 cursor-pointer"
            >
              <X size={12} strokeWidth={3} />
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={patterns.length === 0 ? 'Type pattern + Enter' : ''}
          className={`flex-1 min-w-[8rem] text-sm text-pencil bg-transparent border-none outline-none placeholder:text-muted-dark ${colorClasses.ring}`}
          style={{ fontFamily: "'Courier New', monospace" }}
        />
      </div>
    </div>
  );
}
