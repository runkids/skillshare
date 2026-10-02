import { random, useCurrentFrame } from 'remotion';
import { Canvas, CrayonRect, Stroke } from '../crayon/Crayon';
import { c } from '../crayon/palette';
import { hand } from '../styles/fonts';
import { T, lerp, pop, progress } from '../timeline';
import { tools } from '../layout';

// A wall of copied folders tangled together: the "before" picture.
type Folder = { x: number; y: number; color: string; seed: string };

const folderColors = [c.blue, c.yellow, c.orange, c.green, c.blueLight, c.gold, '#B49AE8'];
const lineColors = [c.blue, c.green, c.orange, '#8E6BD8', c.blueDeep, c.gold];

const folders: Folder[] = [];
for (let row = 0; row < 4; row++) {
  for (let col = 0; col < 8; col++) {
    const seed = `f${row}-${col}`;
    const x = 150 + col * 231 + (random(seed + 'x') - 0.5) * 90;
    const y = 120 + row * 205 + (random(seed + 'y') - 0.5) * 80;
    if (tools.slice(0, 3).some((t) => Math.hypot(t.chaos[0] - x, t.chaos[1] - y) < 190 || Math.hypot(t.chaos[0] - x, t.chaos[1] + 150 - y) < 170)) continue;
    folders.push({ x, y, color: folderColors[Math.floor(random(seed + 'c') * folderColors.length)], seed });
  }
}

const nodes: [number, number][] = [...folders.map((fo) => [fo.x, fo.y] as [number, number]), ...tools.slice(0, 3).map((t) => t.chaos)];

const tangle = Array.from({ length: 30 }, (_, k) => {
  const a = nodes[Math.floor(random(`a${k}`) * nodes.length)];
  let b = nodes[Math.floor(random(`b${k}`) * nodes.length)];
  if (a === b) b = nodes[(nodes.indexOf(a) + 3) % nodes.length];
  const mx = (a[0] + b[0]) / 2;
  const my = (a[1] + b[1]) / 2;
  const c1 = [mx + (random(`c${k}`) - 0.5) * 900, my + (random(`d${k}`) - 0.5) * 600];
  const c2 = [mx + (random(`e${k}`) - 0.5) * 900, my + (random(`g${k}`) - 0.5) * 600];
  return { d: `M${a[0]},${a[1]} C${c1[0]},${c1[1]} ${c2[0]},${c2[1]} ${b[0]},${b[1]}`, color: lineColors[k % lineColors.length] };
});

const crossed = [2, 7, 11, 16, 20].filter((i) => i < folders.length);

const notes = [
  { x: 560, y: 170, text: 'old copy?', rot: -6 },
  { x: 1290, y: 470, text: 'which one is current?', rot: 4 },
  { x: 700, y: 820, text: 'v1.0 ??', rot: -3 },
  { x: 1660, y: 560, text: 'missing!', rot: 6 },
  { x: 230, y: 640, text: 'copied by hand', rot: -4 },
];

const FW = 118;
const FH = 90;

const FolderShape = ({ fo }: { fo: Folder }) => (
  <>
    <CrayonRect x={fo.x - FW / 2} y={fo.y - FH / 2} w={FW * 0.45} h={30} r={8} fill={fo.color} seed={fo.seed + 't'} outline={5} />
    <CrayonRect x={fo.x - FW / 2 + 12} y={fo.y - FH / 2 + 8} w={FW - 24} h={44} r={4} fill={c.white} seed={fo.seed + 'p'} outline={4} />
    <CrayonRect x={fo.x - FW / 2} y={fo.y - FH / 2 + 22} w={FW} h={FH - 22} r={10} fill={fo.color} seed={fo.seed} outline={5} />
  </>
);

export const Chaos = () => {
  const frame = useCurrentFrame();
  const gone = progress(frame, T.untangle, 0.6);
  if (frame > (T.untangle + 1.4) * 30) return null;
  const notesOut = 1 - progress(frame, T.untangle, 0.3);

  return (
    <Canvas
      top={notes.map((n, k) => (
        <text
          key={n.text}
          x={n.x}
          y={n.y}
          textAnchor="middle"
          transform={`rotate(${n.rot} ${n.x} ${n.y})`}
          fontFamily={hand}
          fontWeight={700}
          fontSize={40}
          fill={c.red}
          opacity={progress(frame, 1.0 + k * 0.25, 0.3) * notesOut}
        >
          {n.text}
        </text>
      ))}
    >
      {tangle.map((l, k) => (
        <Stroke key={k} d={l.d} color={l.color} width={7} p={progress(frame, 0.05 + k * 0.035, 0.8) * (1 - gone)} />
      ))}
      {folders.map((fo, i) => {
        const dist = Math.hypot(fo.x - 960, fo.y - 260) / 1100;
        const t = progress(frame, T.untangle + dist * 0.3, 0.7);
        const s = pop(frame, 0.05 + i * 0.025) * (1 - 0.85 * t);
        const x = lerp(fo.x, 960, t);
        const y = lerp(fo.y, 260, t);
        if (s <= 0.01) return null;
        return (
          <g key={fo.seed} opacity={1 - t} transform={`translate(${x} ${y}) scale(${s}) rotate(${(random(fo.seed + 'r') - 0.5) * 14}) translate(${-fo.x} ${-fo.y})`}>
            <FolderShape fo={fo} />
          </g>
        );
      })}
      {crossed.map((i, k) => {
        const fo = folders[i];
        const p = progress(frame, 1.1 + k * 0.15, 0.25) * notesOut;
        return (
          <g key={i}>
            <Stroke d={`M${fo.x - 42},${fo.y - 34} L${fo.x + 42},${fo.y + 40}`} color={c.red} width={10} p={p * 2} />
            <Stroke d={`M${fo.x + 42},${fo.y - 34} L${fo.x - 42},${fo.y + 40}`} color={c.red} width={10} p={p * 2 - 1} />
          </g>
        );
      })}
    </Canvas>
  );
};
