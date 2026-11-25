import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import type { LocalFileChange } from '../api/client';
import { diffExcerpt } from '../lib/diffExcerpt';
import { queryKeys } from '../lib/queryKeys';
import { useT } from '../i18n';
import Button from './Button';
import Spinner from './Spinner';

/** What a remote skill changed since install, each file diffed against the installed version. */
export function LocalChanges({ relPath }: { relPath: string }) {
  const t = useT();
  const { data, error, isPending } = useQuery({
    queryKey: queryKeys.skills.localChanges(relPath),
    queryFn: () => api.getLocalChanges(relPath),
    staleTime: 0,
  });
  if (isPending)
    return (
      <div className="flex justify-center py-6">
        <Spinner />
      </div>
    );
  if (error)
    return (
      <div className="ss-note bad mt-3">
        {t('resourceDetail.modified.loadFailed', { message: (error as Error).message })}
      </div>
    );
  return (
    <div className="mt-3">
      <FileDiffList files={data.files} />
    </div>
  );
}

// textdiff prints this line in place of a diff when either side is binary.
const BINARY = '(binary file differs)';
// Past this many lines a diff shows its changes with some context, and the rest on demand,
// so one long file does not bury the others.
const SHOWN_LINES = 40;

/** Each changed file with its line diff, as textdiff prints it ("+ " added, "- " removed). */
export default function FileDiffList({ files }: { files: LocalFileChange[] }) {
  const t = useT();
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const status = {
    modified: t('resourceDetail.modified.status.modified'),
    added: t('resourceDetail.modified.status.added'),
    deleted: t('resourceDetail.modified.status.deleted'),
  };
  return (
    <div className="flex flex-col gap-3">
      {files.map((f) => {
        const lines = f.diff.replace(/\n$/, '').split('\n');
        const cut = lines.length > SHOWN_LINES && !expanded.has(f.path);
        return (
          <div key={f.path}>
            <p className="mb-1 font-mono text-[12.5px] break-all">
              {f.path} <span className="font-sans text-ink-2">· {status[f.status]}</span>
            </p>
            {f.diff.trim() === BINARY ? (
              <p className="m-0 text-[13px] text-ink-3">{t('update.preview.binary')}</p>
            ) : (
              <pre
                className="ss-code !max-h-[40vh] !overflow-auto !whitespace-pre-wrap"
                style={{ overflowWrap: 'anywhere' }}
              >
                {(cut ? diffExcerpt(lines, SHOWN_LINES) : lines).map((l, i) =>
                  l === null ? (
                    <span key={i} className="block text-ink-3" aria-hidden="true">
                      ⋯
                    </span>
                  ) : (
                    <span
                      key={i}
                      className={l.startsWith('+ ') ? 'add' : l.startsWith('- ') ? 'del' : 'block'}
                    >
                      {l || ' '}
                    </span>
                  ),
                )}
              </pre>
            )}
            {cut && (
              <Button
                variant="ghost"
                size="sm"
                className="mt-1"
                onClick={() => setExpanded((prev) => new Set(prev).add(f.path))}
              >
                {t('update.preview.showAllLines', { count: lines.length })}
              </Button>
            )}
          </div>
        );
      })}
    </div>
  );
}
