import { ShieldOff, CircleCheck } from 'lucide-react';
import { wobbly } from '../../design';

export default function BlockStamp({ isBlocked }: { isBlocked: boolean }) {
  if (isBlocked) {
    return (
      <div
        className="flex items-center gap-1.5 px-3 py-1.5 border-[3px] border-danger bg-danger-light"
        style={{
          borderRadius: wobbly.sm,
          boxShadow: '3px 3px 0px 0px rgba(192, 57, 43, 0.3)',
          transform: 'rotate(-2deg)',
        }}
      >
        <ShieldOff size={16} strokeWidth={3} className="text-danger" />
        <span
          className="text-danger font-bold text-sm uppercase tracking-wider"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Blocked
        </span>
      </div>
    );
  }

  return (
    <div
      className="flex items-center gap-1.5 px-3 py-1.5 border-2 border-success bg-success-light"
      style={{
        borderRadius: wobbly.sm,
      }}
    >
      <CircleCheck size={14} strokeWidth={2.5} className="text-success" />
      <span
        className="text-success font-medium text-sm"
        style={{ fontFamily: 'var(--font-hand)' }}
      >
        Pass
      </span>
    </div>
  );
}
