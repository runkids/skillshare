interface SegmentedFieldProps {
  value: string;
  onChange: (next: string) => void;
  options: { value: string; label: string }[];
}

export default function SegmentedField({ value, onChange, options }: SegmentedFieldProps) {
  return (
    <div className="seg-group seg-group-field" role="radiogroup">
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          role="radio"
          aria-checked={value === opt.value}
          className={`seg-btn ${value === opt.value ? 'active' : ''}`}
          onClick={() => onChange(opt.value)}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
