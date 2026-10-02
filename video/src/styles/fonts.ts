import { loadFont as loadInter } from '@remotion/google-fonts/Inter';
import { loadFont as loadKalam } from '@remotion/google-fonts/Kalam';
import { loadFont as loadJetBrainsMono } from '@remotion/google-fonts/JetBrainsMono';

// Load fonts explicitly: the render container has no system fonts.
// Kalam is the dashboard's Playful heading font.
export const { fontFamily: hand } = loadKalam('normal', { weights: ['400', '700'], subsets: ['latin'] });

export const { fontFamily: sans } = loadInter('normal', {
  weights: ['400', '500', '600', '700'],
  subsets: ['latin'],
});

export const { fontFamily: mono } = loadJetBrainsMono('normal', {
  weights: ['400', '500', '700'],
  subsets: ['latin'],
});
