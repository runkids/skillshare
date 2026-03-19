import { useMemo } from 'react';
import { BookOpen, HelpCircle } from 'lucide-react';
import { fieldDocs, type FieldDoc } from '../../lib/fieldDocs';
import Badge from '../Badge';

interface FieldDocsProps {
  fieldPath: string | null;
}

/** Lookup with progressive fallback for dynamic keys (target names, extra names).
 *  Tries removing middle segments one at a time to find a match.
 *  "targets.universal.mode" → "targets.mode" (hit)
 *  "extras.name.targets.path" → "extras.targets.path" (hit)
 *  "extras.name.targets.path.mode" → "extras.targets.mode" (hit)
 *  "targets.universal" → "targets" (hit)
 */
function lookupFieldDoc(fieldPath: string | null): FieldDoc | null {
  if (!fieldPath) return null;

  let doc = fieldDocs[fieldPath];
  if (!doc) {
    const parts = fieldPath.split('.');

    // Find the best matching fieldDocs key that is a subsequence of the path.
    // Prioritize keys whose last segment matches the path's last segment.
    const lastPart = parts[parts.length - 1];
    let bestKey = '';
    let bestScore = -1;
    for (const key of Object.keys(fieldDocs)) {
      const keyParts = key.split('.');
      if (keyParts.length > parts.length) continue;

      // Check if keyParts is a subsequence of parts
      let ki = 0;
      for (let pi = 0; pi < parts.length && ki < keyParts.length; pi++) {
        if (parts[pi] === keyParts[ki]) ki++;
      }
      if (ki !== keyParts.length) continue;

      // Score: prioritize last-segment match, then longer key
      const lastMatch = keyParts[keyParts.length - 1] === lastPart ? 1000 : 0;
      const score = lastMatch + key.length;
      if (score > bestScore) {
        bestKey = key;
        bestScore = score;
      }
    }
    if (bestKey) doc = fieldDocs[bestKey];

    // Fallback: just the root key (for dynamic child keys like target names)
    // targets.universal → targets
    if (!doc && parts.length >= 2) {
      doc = fieldDocs[parts[0]];
    }
  }

  return doc ?? null;
}

export default function FieldDocs({ fieldPath }: FieldDocsProps) {
  const doc = useMemo(() => lookupFieldDoc(fieldPath), [fieldPath]);

  if (!fieldPath) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-10 px-4 text-center text-pencil-light animate-fade-in">
        <BookOpen size={32} strokeWidth={1.2} className="opacity-25" />
        <p className="text-sm">Move cursor to a field to see documentation</p>
      </div>
    );
  }

  if (!doc) {
    return (
      <div className="flex flex-col items-start gap-2 px-3 py-4 animate-fade-in">
        <Badge variant="warning">
          <HelpCircle size={12} strokeWidth={2} />
          Unknown field
        </Badge>
        <p className="text-sm text-pencil-light font-mono break-all">{fieldPath}</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3 px-3 py-3 text-sm animate-fade-in">
      {/* Field path */}
      <div className="flex items-center gap-1.5">
        <BookOpen size={13} strokeWidth={2} className="text-info flex-shrink-0" />
        <Badge variant="info">{fieldPath}</Badge>
      </div>

      {/* Separator */}
      <div className="border-t border-dashed border-pencil-light/30" />

      {/* Description */}
      <p className="text-sm text-pencil leading-relaxed">{doc.description}</p>

      {/* Type */}
      <div className="flex items-center gap-2">
        <span className="text-[10px] font-medium text-muted-dark uppercase tracking-wider">Type</span>
        <Badge variant="default">{doc.type}</Badge>
      </div>

      {/* Allowed values */}
      {doc.allowedValues && doc.allowedValues.length > 0 && (
        <>
          <div className="border-t border-dashed border-pencil-light/30" />
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-[10px] font-medium text-muted-dark uppercase tracking-wider">Values</span>
            <div className="flex flex-wrap gap-1">
              {doc.allowedValues.map(val => (
                <Badge key={val} variant="accent" size="sm">{val}</Badge>
              ))}
            </div>
          </div>
        </>
      )}

      {/* Example */}
      <div className="border-t border-dashed border-pencil-light/30" />
      <div className="flex flex-col gap-1.5">
        <span className="text-[10px] font-medium text-muted-dark uppercase tracking-wider">Example</span>
        <pre className="text-xs bg-paper border border-dashed border-pencil-light/30 rounded-lg p-2.5 font-mono text-pencil overflow-x-auto whitespace-pre-wrap break-all">
          {doc.example}
        </pre>
      </div>
    </div>
  );
}
