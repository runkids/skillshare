import { random } from 'remotion';

type P = [number, number];

// Catmull-Rom through the points, written as cubic Beziers.
export const smoothPath = (pts: P[], closed = true) => {
  const n = pts.length;
  const at = (i: number) => pts[closed ? (i + n) % n : Math.max(0, Math.min(n - 1, i))];
  let d = `M${pts[0][0].toFixed(1)},${pts[0][1].toFixed(1)}`;
  const last = closed ? n : n - 1;
  for (let i = 0; i < last; i++) {
    const p0 = at(i - 1), p1 = at(i), p2 = at(i + 1), p3 = at(i + 2);
    const c1: P = [p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6];
    const c2: P = [p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6];
    d += ` C${c1[0].toFixed(1)},${c1[1].toFixed(1)} ${c2[0].toFixed(1)},${c2[1].toFixed(1)} ${p2[0].toFixed(1)},${p2[1].toFixed(1)}`;
  }
  return closed ? d + ' Z' : d;
};

const jitter = (pts: P[], seed: string, amp: number): P[] =>
  pts.map(([x, y], i) => [x + (random(`${seed}x${i}`) - 0.5) * 2 * amp, y + (random(`${seed}y${i}`) - 0.5) * 2 * amp]);

export const wobblyRect = (x: number, y: number, w: number, h: number, r: number, seed: string, amp = 3, step = 46) => {
  const pts: P[] = [];
  const edge = (x1: number, y1: number, x2: number, y2: number) => {
    const len = Math.hypot(x2 - x1, y2 - y1);
    const k = Math.max(1, Math.round(len / step));
    for (let i = 0; i < k; i++) pts.push([x1 + ((x2 - x1) * i) / k, y1 + ((y2 - y1) * i) / k]);
  };
  const arc = (cx: number, cy: number, a0: number) => {
    for (let i = 0; i < 3; i++) {
      const a = a0 + (i * Math.PI) / 6;
      pts.push([cx + r * Math.cos(a), cy + r * Math.sin(a)]);
    }
  };
  edge(x + r, y, x + w - r, y);
  arc(x + w - r, y + r, -Math.PI / 2);
  edge(x + w, y + r, x + w, y + h - r);
  arc(x + w - r, y + h - r, 0);
  edge(x + w - r, y + h, x + r, y + h);
  arc(x + r, y + h - r, Math.PI / 2);
  edge(x, y + h - r, x, y + r);
  arc(x + r, y + r, Math.PI);
  return smoothPath(jitter(pts, seed, amp));
};

export const wobblyCircle = (cx: number, cy: number, r: number, seed: string, amp = 3) => {
  const k = Math.max(10, Math.round((2 * Math.PI * r) / 40));
  const pts: P[] = Array.from({ length: k }, (_, i) => {
    const a = (i / k) * Math.PI * 2;
    return [cx + r * Math.cos(a), cy + r * Math.sin(a)];
  });
  return smoothPath(jitter(pts, seed, amp));
};

// Point and tangent angle on a cubic Bezier.
export const cubicAt = (p0: P, p1: P, p2: P, p3: P, t: number) => {
  const u = 1 - t;
  const x = u * u * u * p0[0] + 3 * u * u * t * p1[0] + 3 * u * t * t * p2[0] + t * t * t * p3[0];
  const y = u * u * u * p0[1] + 3 * u * u * t * p1[1] + 3 * u * t * t * p2[1] + t * t * t * p3[1];
  const dx = 3 * u * u * (p1[0] - p0[0]) + 6 * u * t * (p2[0] - p1[0]) + 3 * t * t * (p3[0] - p2[0]);
  const dy = 3 * u * u * (p1[1] - p0[1]) + 6 * u * t * (p2[1] - p1[1]) + 3 * t * t * (p3[1] - p2[1]);
  return { x, y, angle: Math.atan2(dy, dx) };
};
