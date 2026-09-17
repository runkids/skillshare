import { Fragment, useMemo } from 'react';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { useT } from '../../i18n';

type DiffOp = { t: 'eq'; a: string; b: string } | { t: 'del'; a: string } | { t: 'ins'; b: string };

const CONTEXT = 3;

export function diffLines(a: string[], b: string[]): DiffOp[] {
  const n = a.length;
  const m = b.length;
  const dp: Int32Array[] = Array.from({ length: n + 1 }, () => new Int32Array(m + 1));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const out: DiffOp[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ t: 'eq', a: a[i], b: b[j] });
      i++;
      j++;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      out.push({ t: 'del', a: a[i] });
      i++;
    } else {
      out.push({ t: 'ins', b: b[j] });
      j++;
    }
  }
  while (i < n) out.push({ t: 'del', a: a[i++] });
  while (j < m) out.push({ t: 'ins', b: b[j++] });
  return out;
}

interface Props {
  oldText: string;
  newText: string;
  saving: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/** Unified diff of the pending save, with unchanged runs cut down to a few lines of context. */
export default function DiffView({ oldText, newText, saving, onConfirm, onCancel }: Props) {
  const t = useT();
  const ops = useMemo(() => diffLines(oldText.split('\n'), newText.split('\n')), [oldText, newText]);
  const changed = ops.filter((op) => op.t !== 'eq').length;
  // Distance from each line to the nearest change decides whether an unchanged line is shown
  const near = useMemo(() => {
    const dist = ops.map(() => Infinity);
    let last = -Infinity;
    ops.forEach((op, i) => {
      if (op.t !== 'eq') last = i;
      dist[i] = i - last;
    });
    last = Infinity;
    for (let i = ops.length - 1; i >= 0; i--) {
      if (ops[i].t !== 'eq') last = i;
      dist[i] = Math.min(dist[i], last - i);
    }
    return dist.map((d) => d <= CONTEXT);
  }, [ops]);
  const title = t('skillEditor.review.title');

  return (
    <DialogShell open onClose={onCancel} padding="none" className="!max-w-[760px]" ariaLabel={title} preventClose={saving}>
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t(changed === 1 ? 'skillEditor.review.line' : 'skillEditor.review.lines', { count: changed })}</p>
        </div>
      </div>
      <div className="db">
        <div className="ss-code !overflow-auto max-h-[60vh]">
          <div className="min-w-max">
            {ops.map((op, i) => {
              if (!near[i]) return near[i - 1] ? <span key={i} className="block text-ink-3">⋯</span> : null;
              return (
                <Fragment key={i}>
                  {op.t === 'eq' && <span className="block">{'  '}{op.a}</span>}
                  {op.t === 'del' && <span className="del">- {op.a}</span>}
                  {op.t === 'ins' && <span className="add">+ {op.b}</span>}
                </Fragment>
              );
            })}
          </div>
        </div>
      </div>
      <div className="df">
        <span className="flex-1" />
        <Button variant="ghost" onClick={onCancel} disabled={saving}>{t('common.back')}</Button>
        <Button variant="primary" loading={saving} onClick={onConfirm}>{t('skillEditor.saveButton')}</Button>
      </div>
    </DialogShell>
  );
}
