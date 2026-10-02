import { staticFile, useCurrentFrame } from 'remotion';
import { Canvas, CrayonCircle, CrayonRect, Stroke } from '../crayon/Crayon';
import { c } from '../crayon/palette';
import { hand } from '../styles/fonts';
import { T, pop, progress } from '../timeline';

// Two copies of the same skill, side by side: one edited today, one left behind.
const copies = [
  { x: 290, rot: -2, fill: c.white, tool: 'agents/claudecode-color.svg', badge: c.yellow, tagFill: '#D4F2CF', tagInk: '#1F7A3A', tag: 'v1.1 · edited today ✓', fresh: true },
  { x: 1030, rot: 2, fill: '#F7E7C4', tool: 'agents/codex-color.svg', badge: c.blueLight, tagFill: '#FFD6CC', tagInk: c.red, tag: 'v1.0 · 3 months ago', fresh: false },
];
const Y = 150;
const W = 600;
const H = 560;

export const Diverge = () => {
  const frame = useCurrentFrame();
  const veil = progress(frame, T.diverge, 0.4) * (1 - progress(frame, T.untangle - 0.1, 0.4));
  if (veil <= 0) return null;
  const out = progress(frame, T.untangle - 0.2, 0.35);
  const neq = progress(frame, T.diverge + 1.0, 0.4) * (1 - out);
  const ring = progress(frame, T.diverge + 1.6, 0.5) * (1 - out);

  const place = (k: number) => {
    const cx = copies[k].x + W / 2;
    const cy = Y + H / 2;
    const p = pop(frame, T.diverge + 0.1 + k * 0.15, 0.55);
    return `translate(${cx} ${cy + (1 - p) * 600}) scale(${1 - out * 0.6}) rotate(${copies[k].rot}) translate(${-cx} ${-cy})`;
  };

  return (
    <>
      <div style={{ position: 'absolute', inset: 0, backgroundColor: c.paper, opacity: veil * 0.82 }} />
      <Canvas
        opacity={1 - out}
        top={copies.map((cp, k) => (
          <g key={k} transform={place(k)}>
            <image href={staticFile(cp.tool)} x={cp.x - 18} y={Y - 33} width={56} height={56} />
            <g fontFamily={hand} fill={c.ink}>
              <text x={cp.x + 70} y={Y + 110} fontSize={60} fontWeight={700} fill={c.navy}>code-review</text>
              <text x={cp.x + 70} y={Y + 200} fontSize={44}>Review diffs for bugs</text>
              <text x={cp.x + 70} y={Y + 265} fontSize={44}>Check error handling</text>
              {cp.fresh ? (
                <text x={cp.x + 70} y={Y + 330} fontSize={44} fontWeight={700} fill={c.blueDeep}>+ Flag untested changes</text>
              ) : (
                <text x={cp.x + 470} y={Y + 335} fontSize={56} fontWeight={700} fill={c.red}>?</text>
              )}
              <text x={cp.x + W / 2} y={Y + 460} textAnchor="middle" dominantBaseline="central" fontSize={42} fontWeight={700} fill={cp.tagInk}>
                {cp.tag}
              </text>
            </g>
          </g>
        ))}
      >
        {copies.map((cp, k) => (
          <g key={k} transform={place(k)}>
            <CrayonRect x={cp.x} y={Y} w={W} h={H} r={30} fill={cp.fill} seed={`dv${k}`} />
            {!cp.fresh && <Stroke d={`M${cp.x + 70},${Y + 318} L${cp.x + 450},${Y + 318}`} color={c.navy} width={4} />}
            <CrayonRect x={cp.x + 50} y={Y + 415} w={W - 100} h={90} r={22} fill={cp.tagFill} seed={`dt${k}`} outline={5} />
            <CrayonCircle cx={cp.x + 10} cy={Y - 5} r={58} fill={cp.badge} seed={`db${k}`} />
            <CrayonCircle cx={cp.x + 10} cy={Y - 5} r={40} fill={c.white} seed={`dbi${k}`} outline={5} />
            {!cp.fresh && (
              <Stroke
                d={`M${cp.x + 30},${Y + 470} C${cp.x + 20},${Y + 380} ${cp.x + W + 10},${Y + 380} ${cp.x + W - 10},${Y + 460} C${cp.x + W - 20},${Y + 540} ${cp.x + 40},${Y + 545} ${cp.x + 50},${Y + 430}`}
                color={c.red}
                width={8}
                p={ring}
              />
            )}
          </g>
        ))}
        <Stroke d="M895,398 L1025,394" color={c.red} width={15} p={neq * 3} />
        <Stroke d="M895,452 L1025,448" color={c.red} width={15} p={neq * 3 - 1} />
        <Stroke d="M992,362 L928,488" color={c.red} width={15} p={neq * 3 - 2} />
      </Canvas>
    </>
  );
};
