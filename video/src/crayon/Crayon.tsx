import type { ReactNode } from 'react';
import { c } from './palette';
import { cubicAt, wobblyCircle, wobblyRect } from './shapes';

type P = [number, number];

// One crayon pass over a whole layer: speckled grain where the paper shows through, plus a
// slight warp. Seeds stay fixed: re-seeding per frame ("line boil") changes every drawn pixel
// and pushes the README video past GitHub's 10 MB upload limit.
const CrayonDefs = () => {
  const seed = 1;
  return (
    <defs>
      <filter id="cr-layer" filterUnits="userSpaceOnUse" x={0} y={0} width={1920} height={1080}>
        <feTurbulence type="fractalNoise" baseFrequency="0.85" numOctaves="2" seed={7} result="noise" />
        <feColorMatrix in="noise" type="matrix" values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 7 -2.5" result="mask" />
        <feComposite in="SourceGraphic" in2="mask" operator="in" result="speck" />
        <feTurbulence type="fractalNoise" baseFrequency="0.022" numOctaves="2" seed={seed} result="warp" />
        <feDisplacementMap in="speck" in2="warp" scale="6" xChannelSelector="R" yChannelSelector="G" />
      </filter>
      <pattern id="cr-hatch" width="12" height="12" patternUnits="userSpaceOnUse" patternTransform="rotate(-38)">
        <line x1="0" y1="0" x2="0" y2="12" stroke="#fff" strokeWidth="3.2" opacity="0.38" />
      </pattern>
      <pattern id="cr-shade" width="10" height="10" patternUnits="userSpaceOnUse" patternTransform="rotate(52)">
        <line x1="0" y1="0" x2="0" y2="10" stroke={c.navy} strokeWidth="2.2" opacity="0.13" />
      </pattern>
    </defs>
  );
};

// Full-frame SVG. `children` get the crayon pass; `top` (text, logos) stays crisp.
export const Canvas = ({ children, top, opacity = 1 }: { children?: ReactNode; top?: ReactNode; opacity?: number }) => (
  <svg width={1920} height={1080} viewBox="0 0 1920 1080" style={{ position: 'absolute', inset: 0, opacity }}>
    <CrayonDefs />
    <g filter="url(#cr-layer)">{children}</g>
    {top}
  </svg>
);

// A filled shape with hatching and a thick navy outline.
export const Shape = ({ d, fill, outline = 7, shade = false }: { d: string; fill: string; outline?: number; shade?: boolean }) => (
  <g>
    <path d={d} fill={fill} />
    <path d={d} fill="url(#cr-hatch)" />
    {shade && <path d={d} fill="url(#cr-shade)" />}
    {outline > 0 && <path d={d} fill="none" stroke={c.navy} strokeWidth={outline} strokeLinejoin="round" />}
  </g>
);

export const CrayonRect = (p: { x: number; y: number; w: number; h: number; r?: number; fill: string; seed: string; outline?: number; shade?: boolean }) => (
  <Shape d={wobblyRect(p.x, p.y, p.w, p.h, p.r ?? 28, p.seed)} fill={p.fill} outline={p.outline} shade={p.shade} />
);

export const CrayonCircle = (p: { cx: number; cy: number; r: number; fill: string; seed: string; outline?: number; shade?: boolean }) => (
  <Shape d={wobblyCircle(p.cx, p.cy, p.r, p.seed)} fill={p.fill} outline={p.outline} shade={p.shade} />
);

// Open stroke drawn on by p (underline, scribble, cross, tangle line).
export const Stroke = ({ d, color, width = 8, p = 1 }: { d: string; color: string; width?: number; p?: number }) =>
  p <= 0 ? null : (
    <path
      d={d}
      fill="none"
      stroke={color}
      strokeWidth={width}
      strokeLinecap="round"
      strokeLinejoin="round"
      pathLength={1}
      strokeDasharray="1 1"
      strokeDashoffset={1 - Math.min(1, p)}
    />
  );

export const squiggle = (x: number, y: number, w: number, amp = 9, waves = 9) => {
  let d = `M${x},${y}`;
  const step = w / (waves * 2);
  for (let i = 0; i < waves * 2; i++) d += ` q${step / 2},${i % 2 ? amp : -amp} ${step},0`;
  return d;
};

// A fat curved arrow like the ones circling the logo, grown along a cubic by p.
export const CrayonArrow = ({ from, c1, c2, to, color, light, width = 32, p = 1 }: { from: P; c1: P; c2: P; to: P; color: string; light: string; width?: number; p?: number }) => {
  if (p <= 0) return null;
  const steps = 28;
  const pts = Array.from({ length: steps + 1 }, (_, i) => cubicAt(from, c1, c2, to, (i / steps) * p * 0.88));
  const d = pts.map((q, i) => `${i ? 'L' : 'M'}${q.x.toFixed(1)},${q.y.toFixed(1)}`).join(' ');
  const tip = cubicAt(from, c1, c2, to, p);
  const base = pts[pts.length - 1];
  const a = base.angle;
  const hw = width * 1.2;
  const side = (s: number) => `${(base.x + Math.cos(a + (s * Math.PI) / 2) * hw).toFixed(1)},${(base.y + Math.sin(a + (s * Math.PI) / 2) * hw).toFixed(1)}`;
  return (
    <g>
      <path d={d} fill="none" stroke={c.navy} strokeWidth={width + 14} strokeLinecap="round" strokeLinejoin="round" />
      <path d={d} fill="none" stroke={color} strokeWidth={width} strokeLinecap="round" strokeLinejoin="round" />
      <path d={d} fill="none" stroke={light} strokeWidth={width * 0.3} strokeLinecap="round" strokeLinejoin="round" transform={`translate(${Math.sin(a) * 5},${-Math.cos(a) * 5})`} />
      <Shape d={`M${side(1)} L${tip.x.toFixed(1)},${tip.y.toFixed(1)} L${side(-1)} Z`} fill={color} outline={7} />
    </g>
  );
};
