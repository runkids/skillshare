import { Check, Minus } from 'lucide-react';

import type { SyncResult } from '../api/client';

import Badge from './Badge';
import Card from './Card';

interface SyncResultListProps {
  results: SyncResult[];
  detailed?: boolean;
}

export default function SyncResultList({ results, detailed = false }: SyncResultListProps) {
  if (results.length === 0) {
    return (
      <Card variant="outlined">
        <p className="text-pencil-light text-center py-4">No results to show.</p>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      {results.map((r, i) => {
        const linked = r.linked?.length ?? 0;
        const updated = r.updated?.length ?? 0;
        const skipped = r.skipped?.length ?? 0;
        const pruned = r.pruned?.length ?? 0;
        const hasChanges = linked > 0 || updated > 0;

        return (
          <Card key={r.target} style={{ animation: `fadeInUp 0.3s ease-out ${i * 100}ms both` }}>
            <div className="flex items-center gap-3">
              {hasChanges ? (
                <Check size={18} className="text-success shrink-0" />
              ) : (
                <Minus size={18} className="text-pencil-light shrink-0" />
              )}
              <span className="text-pencil font-medium flex-1">{r.target}</span>
              <div className="flex gap-2 flex-wrap">
                {linked > 0 && <Badge variant="success">{linked} linked</Badge>}
                {updated > 0 && <Badge variant="info">{updated} updated</Badge>}
                {skipped > 0 && <Badge variant="default">{skipped} skipped</Badge>}
                {pruned > 0 && <Badge variant="warning">{pruned} pruned</Badge>}
              </div>
            </div>

            {detailed && hasChanges && (
              <div className="mt-3 pt-3 border-t border-pencil/10 space-y-2 text-sm">
                {linked > 0 && (
                  <div>
                    <span className="text-success font-medium">Linked:</span>{' '}
                    <span className="text-pencil-light">{r.linked.join(', ')}</span>
                  </div>
                )}
                {updated > 0 && (
                  <div>
                    <span className="text-info font-medium">Updated:</span>{' '}
                    <span className="text-pencil-light">{r.updated.join(', ')}</span>
                  </div>
                )}
                {pruned > 0 && (
                  <div>
                    <span className="text-warning font-medium">Pruned:</span>{' '}
                    <span className="text-pencil-light">{r.pruned.join(', ')}</span>
                  </div>
                )}
              </div>
            )}
          </Card>
        );
      })}
    </div>
  );
}
