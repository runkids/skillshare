import { ShieldOff, CircleCheck } from 'lucide-react';
import { radius } from '../../design';

export default function BlockStamp({ isBlocked }: { isBlocked: boolean }) {
  if (isBlocked) {
    return (
      <span
        className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-danger-light text-danger border border-danger font-bold text-sm uppercase tracking-wider"
        style={{ borderRadius: radius.sm }}
      >
        <ShieldOff size={14} strokeWidth={2.5} />
        Blocked
      </span>
    );
  }

  return (
    <span
      className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-success-light text-success border border-success font-medium text-sm"
      style={{ borderRadius: radius.sm }}
    >
      <CircleCheck size={14} strokeWidth={2.5} />
      Pass
    </span>
  );
}
