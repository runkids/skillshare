import { c } from './palette';
import { Shape } from './Crayon';

// Four-point star burst, scaled by p (0..1).
export const Sparkle = ({ x, y, size = 40, p = 1, color = c.yellow }: { x: number; y: number; size?: number; p?: number; color?: string }) => {
  if (p <= 0) return null;
  const s = size * p;
  const k = s * 0.28;
  const d = `M0,${-s} Q${k},${-k} ${s},0 Q${k},${k} 0,${s} Q${-k},${k} ${-s},0 Q${-k},${-k} 0,${-s} Z`;
  return (
    <g transform={`translate(${x},${y}) rotate(${p * 20})`}>
      <Shape d={d} fill={color} outline={5} />
    </g>
  );
};
