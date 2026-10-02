import { useCurrentFrame } from 'remotion';
import { Canvas, CrayonRect } from '../crayon/Crayon';
import { c } from '../crayon/palette';
import { hand } from '../styles/fonts';
import { captions, progress } from '../timeline';

// Narration strip along the bottom, one sentence per beat.
export const Caption = () => {
  const frame = useCurrentFrame();
  const cap = captions.find(([s, e]) => frame >= s * 30 && frame < e * 30);
  if (!cap) return null;
  const [start, end, text] = cap;
  const p = progress(frame, start, 0.25) * (1 - progress(frame, end - 0.25, 0.25));
  const w = text.length * 27 + 100;

  return (
    <Canvas
      opacity={p}
      top={
        <text x={960} y={980 + (1 - p) * 16} textAnchor="middle" dominantBaseline="central" fontFamily={hand} fontWeight={700} fontSize={58} fill={c.navy}>
          {text}
        </text>
      }
    >
      <g transform={`translate(0 ${(1 - p) * 16})`}>
        <CrayonRect x={960 - w / 2} y={930} w={w} h={100} r={26} fill={c.white} seed={text} outline={6} />
      </g>
    </Canvas>
  );
};
