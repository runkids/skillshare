import { staticFile, useCurrentFrame } from 'remotion';
import { Canvas, CrayonCircle } from '../crayon/Crayon';
import { wobblyRect } from '../crayon/shapes';
import { Shape } from '../crayon/Crayon';
import { Sparkle } from '../crayon/Sparkle';
import { c } from '../crayon/palette';
import { hand } from '../styles/fonts';
import { T, lerp, pop, progress } from '../timeline';
import { R, TOOL_Y, arriveAt, toolX, tools } from '../layout';

// The AI tools stay on screen from the tangle until the scope beat, so the viewer follows them.
const place = (i: number, frame: number) => {
  if (i === 3) return { x: toolX(3, frame), y: TOOL_Y, s: pop(frame, T.join + 0.3, 0.5) };
  const t = progress(frame, T.untangle + 0.2, 1.0);
  return {
    x: lerp(tools[i].chaos[0], toolX(i, frame), t),
    y: lerp(tools[i].chaos[1], TOOL_Y, t),
    s: pop(frame, 0.2 + i * 0.15),
  };
};

export const Tools = () => {
  const frame = useCurrentFrame();
  const out = progress(frame, T.scope, 0.35);
  if (out >= 1) return null;

  const items = tools.map((tool, i) => {
    const { x, y, s } = place(i, frame);
    const arrive = arriveAt(i);
    const bounce = Math.sin(progress(frame, arrive, 0.35) * Math.PI) * 0.14;
    const scale = s * (1 + bounce);
    const tag = pop(frame, arrive + 0.05);
    return { tool, i, x, y, scale, tag, spark: pop(frame, arrive, 0.5) };
  });

  return (
    <Canvas
      opacity={1 - out}
      top={items.map(({ tool, x, y, scale, tag }) =>
        scale <= 0.01 ? null : (
          <g key={tool.name}>
            <g transform={`translate(${x} ${y}) scale(${scale})`}>
              <image href={staticFile(tool.logo)} x={-40} y={-40} width={80} height={80} />
              <text y={R + 58} textAnchor="middle" fontFamily={hand} fontWeight={700} fontSize={48} fill={c.navy}>
                {tool.name}
              </text>
            </g>
            {tag > 0.01 && (
              <g transform={`translate(${x + 128} ${y - R - 14}) rotate(4) scale(${tag})`}>
                <text textAnchor="middle" dominantBaseline="central" fontFamily={hand} fontWeight={700} fontSize={40} fill="#1F7A3A">
                  ✓ updated
                </text>
              </g>
            )}
          </g>
        ),
      )}
    >
      {items.map(({ tool, x, y, scale, tag, spark }) =>
        scale <= 0.01 ? null : (
          <g key={tool.name}>
            <g transform={`translate(${x} ${y}) scale(${scale})`}>
              <CrayonCircle cx={0} cy={0} r={R} fill={tool.fill} seed={tool.name} shade />
              <CrayonCircle cx={0} cy={0} r={60} fill={c.white} seed={tool.name + 'in'} outline={5} />
            </g>
            {tag > 0.01 && (
              <g transform={`translate(${x + 128} ${y - R - 14}) rotate(4) scale(${tag})`}>
                <Shape d={wobblyRect(-112, -32, 224, 64, 18, tool.name + 'tag')} fill="#D4F2CF" outline={5} />
              </g>
            )}
            <Sparkle x={x - 112} y={y - 92} size={28} p={spark} />
            <Sparkle x={x - 134} y={y - 40} size={16} p={spark} color={c.blueLight} />
          </g>
        ),
      )}
    </Canvas>
  );
};
