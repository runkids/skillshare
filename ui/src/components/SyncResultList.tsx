import { Check, Minus } from 'lucide-react';

import type { SyncResult } from '../api/client';

import Badge from './Badge';
import Card from './Card';
import { useT } from '../i18n';

export default function SyncResultList({ results }: { results: SyncResult[] }) {
  const t = useT();

  if (results.length === 0) {
    return (
      <Card variant="outlined">
        <p className="text-pencil-light text-center py-4">{t('syncResult.empty')}</p>
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
        const hasChanges = linked > 0 || updated > 0 || pruned > 0;

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
                {r.dir_created && (
                  <Badge variant="info">{t('syncResult.dirCreated')}</Badge>
                )}
                {linked > 0 && <Badge variant="success">{t('syncResult.linked', { count: linked })}</Badge>}
                {updated > 0 && <Badge variant="info">{t('syncResult.updated', { count: updated })}</Badge>}
                {skipped > 0 && <Badge variant="default">{t('syncResult.skipped', { count: skipped })}</Badge>}
                {pruned > 0 && <Badge variant="warning">{t('syncResult.pruned', { count: pruned })}</Badge>}
              </div>
            </div>
          </Card>
        );
      })}
    </div>
  );
}
