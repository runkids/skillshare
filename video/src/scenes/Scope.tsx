import { useCurrentFrame } from 'remotion';
import { Canvas, CrayonCircle, CrayonRect, Stroke } from '../crayon/Crayon';
import { c } from '../crayon/palette';
import { Icon } from '../components/Icon';
import type { IconName } from '../components/Icon';
import { hand } from '../styles/fonts';
import { T, pop, progress } from '../timeline';
import { CARD } from '../layout';

// Everything else skillshare manages, hung from the same source card.
const kinds: { label: string; icon: IconName; fill: string; size?: number; note?: string }[] = [
  { label: 'Skills', icon: 'sparkles', fill: c.yellow },
  { label: 'Agents', icon: 'bot', fill: c.blueLight },
  { label: 'Rules & commands', icon: 'fileText', fill: '#C9EBB4', size: 31 },
  { label: 'MCP servers', icon: 'plug', fill: '#D9C8F5' },
  { label: 'Hooks', icon: 'webhook', fill: '#FFC9D9', note: 'native format per tool' },
];
const W = 300;
const GAP = 30;
const X0 = (1920 - (5 * W + 4 * GAP)) / 2;
const Y = 560;
const H = 250;

export const Scope = () => {
  const frame = useCurrentFrame();
  if (frame < T.scope * 30) return null;
  const fade = 1 - progress(frame, T.closing, 0.4);
  if (fade <= 0) return null;

  const items = kinds.map((k, i) => {
    const x = X0 + i * (W + GAP);
    return { ...k, i, x, cx: x + W / 2, p: pop(frame, T.scope + 0.3 + i * 0.12, 0.5), string: progress(frame, T.scope + 0.2 + i * 0.12, 0.35) };
  });

  return (
    <Canvas
      opacity={fade}
      top={items.map((k) =>
        k.p <= 0.01 ? null : (
          <g key={k.label} transform={`translate(${k.cx} ${Y + H / 2}) scale(${k.p}) translate(${-k.cx} ${-(Y + H / 2)})`} fontFamily={hand}>
            <text x={k.cx} y={Y + 170} textAnchor="middle" fontSize={k.size ?? 48} fontWeight={700} fill={c.navy}>{k.label}</text>
            {k.note && <text x={k.cx} y={Y + 215} textAnchor="middle" fontSize={28} fill={c.ink}>{k.note}</text>}
          </g>
        ),
      )}
    >
      {items.map((k) => (
        <Stroke key={k.label} d={`M${960 + (k.i - 2) * 100},${CARD.y + CARD.h} C${960 + (k.i - 2) * 100},${CARD.y + CARD.h + 60} ${k.cx},${Y - 70} ${k.cx},${Y}`} color={c.navy} width={5} p={k.string} />
      ))}
      {items.map((k) =>
        k.p <= 0.01 ? null : (
          <g key={k.label} transform={`translate(${k.cx} ${Y + H / 2}) scale(${k.p}) rotate(${(k.i - 2) * 1.5}) translate(${-k.cx} ${-(Y + H / 2)})`}>
            <CrayonRect x={k.x} y={Y} w={W} h={H} r={26} fill={k.fill} seed={`k${k.i}`} shade />
            <CrayonCircle cx={k.cx} cy={Y + 4} r={12} fill={c.red} seed={`pin${k.i}`} outline={4} />
            <g transform={`translate(${k.cx - 48} ${Y + 30})`}>
              <Icon name={k.icon} size={96} color={c.navy} strokeWidth={2.2} />
            </g>
          </g>
        ),
      )}
    </Canvas>
  );
};
