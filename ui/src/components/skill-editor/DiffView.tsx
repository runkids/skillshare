import { useMemo, type ReactNode } from 'react';
import { Check, Code2 } from 'lucide-react';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { useT } from '../../i18n';

type DiffOp = { t: 'eq'; a: string; b: string } | { t: 'del'; a: string } | { t: 'ins'; b: string };

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

interface DiffViewProps {
  open: boolean;
  oldText: string;
  newText: string;
  oldLabel?: string;
  newLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
  saving?: boolean;
}

export default function DiffView({
  open,
  oldText,
  newText,
  oldLabel,
  newLabel,
  onConfirm,
  onCancel,
  saving = false,
}: DiffViewProps) {
  const t = useT();
  const resolvedOldLabel = oldLabel ?? t('diffView.oldLabel');
  const resolvedNewLabel = newLabel ?? t('diffView.newLabel');
  const rows = useMemo(() => {
    const a = (oldText ?? '').split('\n');
    const b = (newText ?? '').split('\n');
    return diffLines(a, b);
  }, [oldText, newText]);

  const { adds, dels } = useMemo(() => {
    let adds = 0;
    let dels = 0;
    for (const r of rows) {
      if (r.t === 'ins') adds++;
      else if (r.t === 'del') dels++;
    }
    return { adds, dels };
  }, [rows]);

  const rendered = useMemo(() => {
    let la = 0;
    let lb = 0;
    return rows.map((r, idx) => {
      if (r.t === 'eq') {
        la++;
        lb++;
        return {
          idx,
          kind: 'eq' as const,
          left: { n: la, s: r.a },
          right: { n: lb, s: r.b },
        };
      }
      if (r.t === 'del') {
        la++;
        return { idx, kind: 'del' as const, left: { n: la, s: r.a }, right: null };
      }
      lb++;
      return { idx, kind: 'ins' as const, left: null, right: { n: lb, s: r.b } };
    });
  }, [rows]);

  const noChanges = adds === 0 && dels === 0;
  const changedCount = adds + dels;

  return (
    <DialogShell
      open={open}
      onClose={onCancel}
      maxWidth="3xl"
      className="!max-w-[60rem]"
      padding="none"
      preventClose={saving}
    >
      <div className="ss-skill-editor-diff flex flex-col max-h-[85vh]">
        <div className="flex items-center gap-3 px-5 py-4 border-b-2 border-muted">
          <Code2 size={18} className="text-pencil-light" />
          <h2 className="text-base font-bold text-pencil">{t('diffView.reviewTitle')}</h2>
          <div className="flex items-center gap-2 font-mono text-sm font-bold">
            <span className="text-success">+{adds}</span>
            <span className="text-danger">−{dels}</span>
          </div>
        </div>

        <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
          <div className="grid grid-cols-2 border-b border-muted bg-paper">
            <span className="px-4 py-2 text-[11px] font-bold uppercase tracking-wider text-danger bg-danger/5">
              {resolvedOldLabel}
            </span>
            <span className="px-4 py-2 text-[11px] font-bold uppercase tracking-wider text-success bg-success/5 border-l border-muted">
              {resolvedNewLabel}
            </span>
          </div>
          <div className="flex-1 overflow-auto bg-surface font-mono text-[12.5px] leading-relaxed">
            {rendered.map((row) => (
              <div className="grid grid-cols-2" key={row.idx}>
                <DiffCell side="left" kind={row.kind} data={row.left} />
                <DiffCell side="right" kind={row.kind} data={row.right} />
              </div>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-3 px-5 py-3 border-t-2 border-muted bg-paper">
          <span className="text-xs text-pencil-light">
            {noChanges
              ? t('diffView.noChanges')
              : changedCount === 1
                ? t('diffView.lineChanged', { count: changedCount })
                : t('diffView.linesChanged', { count: changedCount })}
          </span>
          <div className="flex-1" />
          <Button variant="secondary" size="md" onClick={onCancel} disabled={saving}>
            {t('diffView.keepEditing')}
          </Button>
          <Button
            variant="primary"
            size="md"
            onClick={onConfirm}
            disabled={noChanges || saving}
            loading={saving}
          >
            <Check size={14} /> {t('diffView.confirmSave')}
          </Button>
        </div>
      </div>
    </DialogShell>
  );
}

interface DiffCellProps {
  side: 'left' | 'right';
  kind: 'eq' | 'del' | 'ins';
  data: { n: number; s: string } | null;
}

function DiffCell({ side, kind, data }: DiffCellProps): ReactNode {
  const gutter =
    kind === 'del' && side === 'left'
      ? '−'
      : kind === 'ins' && side === 'right'
        ? '+'
        : '\u00a0';
  const bgClass =
    kind === 'del' && side === 'left'
      ? 'bg-danger/10'
      : kind === 'ins' && side === 'right'
        ? 'bg-success/10'
        : '';
  const gutterColor =
    kind === 'del' && side === 'left'
      ? 'text-danger'
      : kind === 'ins' && side === 'right'
        ? 'text-success'
        : 'text-muted-dark';
  const borderClass = side === 'right' ? 'border-l border-muted' : '';
  return (
    <div className={`grid grid-cols-[40px_16px_1fr] items-start py-px min-w-0 ${borderClass} ${bgClass}`}>
      <span className="text-right px-2 text-[11px] text-muted-dark select-none">
        {data ? data.n : ''}
      </span>
      <span className={`text-center font-bold select-none ${gutterColor}`}>{gutter}</span>
      <span className="pr-4 whitespace-pre-wrap break-words min-w-0">
        {data ? data.s || '\u00a0' : ''}
      </span>
    </div>
  );
}
