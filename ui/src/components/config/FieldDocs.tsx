import { useMemo } from 'react';
import { fieldDocs, type FieldDoc } from '../../lib/fieldDocs';
import { useT } from '../../i18n';

interface FieldDocsProps {
  fieldPath: string | null;
  docs?: Record<string, FieldDoc>;
}

/** Lookup with progressive fallback for dynamic keys (target names, extra names).
 *  Tries removing middle segments one at a time to find a match.
 *  "targets.universal.mode" → "targets.mode" (hit)
 *  "extras.name.targets.path" → "extras.targets.path" (hit)
 *  "extras.name.targets.path.mode" → "extras.targets.mode" (hit)
 *  "targets.universal" → "targets" (hit)
 */
function lookupFieldDoc(fieldPath: string | null, docsMap: Record<string, FieldDoc>): FieldDoc | null {
  if (!fieldPath) return null;

  // MCP server names and env/header names are user-defined, even when they
  // happen to match a schema field such as "targets" or "url".
  if (fieldPath.startsWith('mcp.servers.')) {
    const parts = fieldPath.slice('mcp.servers.'.length).split('.');
    const field = parts[1];
    if (!field) return docsMap['mcp.servers'] ?? null;
    const suffix = (field === 'env' || field === 'headers')
      ? (parts.length > 3 ? '.fromEnv' : '')
      : (parts[2] === 'fromEnv' ? '.fromEnv' : '');
    return docsMap[`mcp.servers.${field}${suffix}`] ?? null;
  }

  let doc = docsMap[fieldPath];
  if (!doc) {
    const parts = fieldPath.split('.');

    // Find the best matching docsMap key that is a subsequence of the path.
    // Prioritize keys whose last segment matches the path's last segment.
    const lastPart = parts[parts.length - 1];
    let bestKey = '';
    let bestScore = -1;
    for (const key of Object.keys(docsMap)) {
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
    if (bestKey) doc = docsMap[bestKey];

    // Fallback: dynamic value under a known section → treat as a name
    // targets.agents → targets.name, extras.rules → extras.name
    if (!doc && parts.length >= 2) {
      const nameKey = parts[0] + '.name';
      doc = docsMap[nameKey] ?? docsMap[parts[0]];
    }
  }

  return doc ?? null;
}

export default function FieldDocs({ fieldPath, docs }: FieldDocsProps) {
  const t = useT();
  const docsMap = docs ?? fieldDocs;
  const doc = useMemo(() => lookupFieldDoc(fieldPath, docsMap), [fieldPath, docsMap]);

  if (!fieldPath) return <p className="text-[13px] text-ink-3">{t('config.panel.fieldHint')}</p>;

  if (!doc) {
    return (
      <div className="flex flex-col items-start gap-2">
        <span className="ss-tag warn">{t('config.panel.unknownField')}</span>
        <p className="break-all font-mono text-[13px] text-ink-2">{fieldPath}</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <span className="ss-tag self-start font-mono">{fieldPath}</span>
      <p className="text-[13px] leading-relaxed">{doc.description}</p>
      <dl className="ss-kv !grid-cols-[64px_minmax(0,1fr)]">
        <dt>{t('resources.table.type')}</dt>
        <dd className="font-mono">{doc.type}</dd>
        {doc.allowedValues && doc.allowedValues.length > 0 && (
          <>
            <dt>{t('config.panel.fieldValues')}</dt>
            <dd className="font-mono">{doc.allowedValues.join(', ')}</dd>
          </>
        )}
      </dl>
      <div className="flex flex-col gap-1.5">
        <span className="text-[12px] font-semibold text-ink-3">{t('config.panel.fieldExample')}</span>
        <pre className="ss-code !whitespace-pre-wrap break-all !px-3 !py-2.5">{doc.example}</pre>
      </div>
    </div>
  );
}
