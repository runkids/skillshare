import { Composition } from 'remotion';
import { Promo } from './Promo';
import { FPS, f, T } from './timeline';

export const RemotionRoot = () => (
  <Composition id="PromoVideo" component={Promo} durationInFrames={f(T.end)} fps={FPS} width={1920} height={1080} />
);
