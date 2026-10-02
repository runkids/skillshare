import { AbsoluteFill } from 'remotion';
import { c } from './crayon/palette';
import { Chaos } from './scenes/Chaos';
import { Sync } from './scenes/Sync';
import { Tools } from './scenes/Tools';
import { Diverge } from './scenes/Diverge';
import { Scope } from './scenes/Scope';
import { Closing } from './scenes/Closing';
import { Caption } from './scenes/Caption';

// Illustrated in the style of the cartoon logo: crayon fills, navy outlines, paper.
export const Promo = () => (
  <AbsoluteFill style={{ backgroundColor: c.paper }}>
    <svg width={1920} height={1080} style={{ position: 'absolute', inset: 0 }}>
      <filter id="paper">
        <feTurbulence type="fractalNoise" baseFrequency="0.6" numOctaves="3" seed="2" />
        <feColorMatrix type="matrix" values="0 0 0 0 0.45  0 0 0 0 0.35  0 0 0 0 0.2  0 0 0 0.09 0" />
      </filter>
      <rect width={1920} height={1080} filter="url(#paper)" />
    </svg>
    <Chaos />
    <Sync />
    <Tools />
    <Diverge />
    <Scope />
    <Closing />
    <Caption />
  </AbsoluteFill>
);
