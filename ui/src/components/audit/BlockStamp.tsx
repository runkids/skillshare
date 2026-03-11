import { ShieldOff, CircleCheck } from 'lucide-react';

export default function BlockStamp({ isBlocked }: { isBlocked: boolean }) {
  if (isBlocked) {
    return (
      <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded bg-danger-light text-danger border border-danger font-bold text-sm uppercase tracking-wider">
        <ShieldOff size={14} strokeWidth={2.5} />
        Blocked
      </span>
    );
  }

  return (
    <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded bg-success-light text-success border border-success font-medium text-sm">
      <CircleCheck size={14} strokeWidth={2.5} />
      Pass
    </span>
  );
}
