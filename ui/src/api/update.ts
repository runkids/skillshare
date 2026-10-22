import { apiFetch, BASE, createSSEStream } from './http';
import type { UpdateResultItem, UpdateStreamSummary } from './types/update';

export const updateApi = {
  update: (opts: { name?: string; kind?: 'skill' | 'agent'; force?: boolean; all?: boolean; skipAudit?: boolean }) =>
    apiFetch<{ results: UpdateResultItem[] }>('/update', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  updateAllStream: (
    onStart: (total: number) => void,
    onResult: (item: UpdateResultItem) => void,
    onDone: (data: { results: UpdateResultItem[]; summary: UpdateStreamSummary }) => void,
    onError: (err: Error) => void,
    opts?: { names?: string[]; kind?: 'skill' | 'agent'; force?: boolean; skipAudit?: boolean },
  ): EventSource => {
    const params = new URLSearchParams();
    if (opts?.names?.length) params.set('names', opts.names.join(','));
    if (opts?.kind) params.set('kind', opts.kind);
    if (opts?.force) params.set('force', 'true');
    if (opts?.skipAudit) params.set('skipAudit', 'true');
    return createSSEStream(`${BASE}/update/stream?${params.toString()}`, {
      start: (d) => onStart(d.total),
      result: onResult,
      done: onDone,
    }, onError, 'Update stream failed');
  },
};
