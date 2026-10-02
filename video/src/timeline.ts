import { interpolate, Easing } from 'remotion';

export const FPS = 30;
export const f = (seconds: number) => Math.round(seconds * FPS);

// Every beat, in seconds.
export const T = {
  diverge: 3.4, // two copies come forward
  untangle: 6.4, // the tangle collapses into one source
  card: 7.6, // the source card appears
  arrows: 8.0,
  write: 8.7, // the pencil writes the new line
  writeDur: 1.1,
  pulses: 10.0, // the change travels to every tool
  pulseDur: 0.7,
  join: 12.2, // a new tool joins
  scope: 14.8,
  closing: 17.4,
  fadeOut: 20.4,
  end: 21,
};

// Narration, like subtitles: [start, end, text].
export const captions: [number, number, string][] = [
  [0.3, 3.3, 'Every AI tool keeps its own copy of your setup.'],
  [3.5, 6.3, 'Edit one, and the others fall behind.'],
  [6.6, 8.6, 'skillshare keeps one source.'],
  [8.7, 12.0, 'Change it once. Every tool gets it.'],
  [12.2, 14.6, 'Switch tools? Your setup is already there.'],
  [14.9, 17.2, 'Skills, agents, rules, MCP and hooks. One place.'],
];

export const progress = (frame: number, start: number, duration: number, ease = Easing.inOut(Easing.cubic)) =>
  interpolate(frame, [f(start), f(start + duration)], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: ease,
  });

export const pop = (frame: number, start: number, duration = 0.45) => progress(frame, start, duration, Easing.out(Easing.back(1.8)));

export const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
