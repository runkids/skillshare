import { Easing, staticFile, useCurrentFrame } from 'remotion';
import { Canvas, CrayonRect, Stroke, squiggle } from '../crayon/Crayon';
import { c } from '../crayon/palette';
import { hand, mono, sans } from '../styles/fonts';
import { T, pop, progress } from '../timeline';

export const Closing = () => {
  const frame = useCurrentFrame();
  if (frame < T.closing * 30) return null;
  const fade = 1 - progress(frame, T.fadeOut, 0.6);
  const logo = pop(frame, T.closing + 0.3, 0.7);
  const spin = (1 - progress(frame, T.closing + 0.3, 1.0, Easing.out(Easing.cubic))) * -240;
  const head = progress(frame, T.closing + 0.6, 0.4);
  const under = progress(frame, T.closing + 1.0, 0.6);
  const sub = progress(frame, T.closing + 1.3, 0.4);
  const chip = pop(frame, T.closing + 1.6, 0.5);

  return (
    <Canvas
      opacity={fade}
      top={
        <>
          <g transform={`translate(960 270) rotate(${spin}) scale(${logo})`}>
            <image href={staticFile('logo.png')} x={-170} y={-170} width={340} height={340} />
          </g>
          <text x={960} y={590 + (1 - head) * 30} opacity={head} textAnchor="middle" fontFamily={hand} fontWeight={700} fontSize={104} fill={c.navy}>
            Your AI coding setup, everywhere.
          </text>
          <text x={960} y={720 + (1 - sub) * 20} opacity={sub} textAnchor="middle" fontFamily={hand} fontWeight={700} fontSize={56} fill={c.ink}>
            Switch tools. Keep your setup.
          </text>
          <g opacity={Math.min(1, chip * 1.4)}>
            <text x={960} y={826} textAnchor="middle" dominantBaseline="central" fontFamily={mono} fontWeight={700} fontSize={42} fill={c.navy}>
              brew install skillshare
            </text>
            <text x={960} y={945} textAnchor="middle" fontFamily={sans} fontWeight={600} fontSize={36} fill={c.blueDeep}>
              skillshare.runkids.cc
            </text>
          </g>
        </>
      }
    >
      <Stroke d={squiggle(540, 625, 840)} color={c.red} width={9} p={under} />
      {chip > 0.01 && (
        <g transform={`translate(960 826) scale(${chip}) translate(-960 -826)`}>
          <CrayonRect x={610} y={778} w={700} h={96} r={48} fill={c.yellow} seed="chip" />
        </g>
      )}
    </Canvas>
  );
};
