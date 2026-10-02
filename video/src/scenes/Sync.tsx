import { Easing, staticFile, useCurrentFrame } from 'remotion';
import { Canvas, CrayonArrow, CrayonRect, Shape, Stroke } from '../crayon/Crayon';
import { Pencil } from '../crayon/Pencil';
import { c } from '../crayon/palette';
import { cubicAt, wobblyRect } from '../crayon/shapes';
import { hand } from '../styles/fonts';
import { T, lerp, pop, progress } from '../timeline';
import { CARD, R, TOOL_Y, joinProgress, toolX } from '../layout';

type P = [number, number];

const NEW_LINE = '+ Flag untested changes';
const LINE_X = CARD.x + 70;
const LINE_Y = CARD.y + 300;
const LINE_W = 520;

const arrowFor = (i: number, frame: number) => {
  const j = joinProgress(frame);
  const spread3 = (i - 1) * 140;
  const spread4 = (i - 1.5) * 110;
  const from: P = [960 + lerp(spread3, spread4, i < 3 ? j : 1), CARD.y + CARD.h + 4];
  const to: P = [toolX(i, frame), TOOL_Y - R - 20];
  return { from, c1: [from[0], from[1] + 110] as P, c2: [to[0], to[1] - 130] as P, to };
};

const arrowGrow = (i: number, frame: number) => (i < 3 ? progress(frame, T.arrows + i * 0.12, 0.6) : progress(frame, T.join + 0.6, 0.5));
const pulseAt = (i: number, frame: number) => (i < 3 ? progress(frame, T.pulses + i * 0.1, T.pulseDur) : progress(frame, T.join + 1.1, 0.5));

// One source card, a pencil adds a line, and the change rides the arrows to every tool.
export const Sync = () => {
  const frame = useCurrentFrame();
  if (frame < T.untangle * 30) return null;
  const fade = 1 - progress(frame, T.closing, 0.4);
  if (fade <= 0) return null;
  const arrowsOut = 1 - progress(frame, T.scope, 0.3);

  // Logo: spins in at the centre, then settles on the card's corner.
  const logoIn = pop(frame, T.untangle + 0.2, 0.7);
  const spin = (1 - progress(frame, T.untangle + 0.2, 1.0, Easing.out(Easing.cubic))) * -240;
  const settle = progress(frame, T.card, 0.5);
  const logoSize = lerp(340, 170, settle) * logoIn;
  const logoX = lerp(960, CARD.x - 5, settle);
  const logoY = lerp(330, CARD.y + 15, settle);

  const card = pop(frame, T.card + 0.4, 0.5);
  const written = progress(frame, T.write, T.writeDur, Easing.linear);
  const pencil = progress(frame, T.write - 0.3, 0.2) * (1 - progress(frame, T.write + T.writeDur + 0.2, 0.3));
  const marker = progress(frame, T.write + T.writeDur, 0.3);
  const cardT = `translate(960 ${CARD.y + CARD.h / 2}) scale(${card}) rotate(-1.5) translate(-960 ${-(CARD.y + CARD.h / 2)})`;

  const count = frame >= T.join * 30 ? 4 : 3;
  const arrows = Array.from({ length: count }, (_, i) => ({ i, ...arrowFor(i, frame), p: arrowGrow(i, frame), pulse: pulseAt(i, frame) }));

  return (
    <Canvas
      opacity={fade}
      top={
        <>
          {card > 0.01 && (
            <g transform={cardT} fontFamily={hand}>
              <text x={LINE_X} y={CARD.y + 100} fontSize={66} fontWeight={700} fill={c.navy}>code-review</text>
              <text x={LINE_X} y={CARD.y + 175} fontSize={46} fill={c.ink}>Review diffs for bugs</text>
              <text x={LINE_X} y={CARD.y + 237} fontSize={46} fill={c.ink}>Check error handling</text>
              <clipPath id="written">
                <rect x={LINE_X - 10} y={LINE_Y - 60} width={written * (LINE_W + 20)} height={90} />
              </clipPath>
              <text x={LINE_X} y={LINE_Y} fontSize={46} fontWeight={700} fill={c.blueDeep} clipPath="url(#written)">{NEW_LINE}</text>
            </g>
          )}
          {logoSize > 1 && (
            <g transform={`translate(${logoX} ${logoY}) rotate(${spin})`}>
              <image href={staticFile('logo.png')} x={-logoSize / 2} y={-logoSize / 2} width={logoSize} height={logoSize} />
            </g>
          )}
        </>
      }
    >
      <g opacity={arrowsOut}>
        {arrows.map((a) => (
          <CrayonArrow key={a.i} from={a.from} c1={a.c1} c2={a.c2} to={a.to} color={a.i % 2 ? c.gold : c.blue} light={a.i % 2 ? c.yellow : c.blueLight} p={a.p} />
        ))}
        {arrows.map((a) => {
          if (a.pulse <= 0 || a.pulse >= 1) return null;
          const q = cubicAt(a.from, a.c1, a.c2, a.to, a.pulse);
          return (
            <g key={a.i} transform={`translate(${q.x} ${q.y}) rotate(${Math.sin(a.pulse * 9) * 8})`}>
              <Shape d={wobblyRect(-30, -38, 60, 76, 8, `pd${a.i}`)} fill={c.yellow} outline={5} />
              <Stroke d="M-16,-14 L16,-14" color={c.navy} width={5} />
              <Stroke d="M-16,2 L16,2" color={c.navy} width={5} />
              <Stroke d="M-16,18 L6,18" color={c.blueDeep} width={5} />
            </g>
          );
        })}
      </g>
      {card > 0.01 && (
        <g transform={cardT}>
          <CrayonRect x={CARD.x} y={CARD.y} w={CARD.w} h={CARD.h} r={30} fill={c.white} seed="src" />
          <Stroke d={`M${LINE_X - 15},${LINE_Y - 10} C${LINE_X + 150},${LINE_Y - 14} ${LINE_X + 380},${LINE_Y - 8} ${LINE_X + LINE_W + 10},${LINE_Y - 14}`} color={c.yellow} width={46} p={marker} />
          {pencil > 0.01 && (
            <g opacity={pencil}>
              <Pencil x={LINE_X + written * LINE_W + 6} y={LINE_Y + 4 + Math.sin(frame * 1.3) * 5} scale={0.55} rotate={28} />
            </g>
          )}
        </g>
      )}
    </Canvas>
  );
};
