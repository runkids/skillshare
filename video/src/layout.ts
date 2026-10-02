import { c } from './crayon/palette';
import { T, lerp, progress } from './timeline';

export const CARD = { x: 600, y: 80, w: 720, h: 360 };
export const TOOL_Y = 690;
export const R = 92;

export type Tool = { name: string; logo: string; fill: string; chaos: [number, number] };

export const tools: Tool[] = [
  { name: 'Claude Code', logo: 'agents/claudecode-color.svg', fill: c.yellow, chaos: [330, 380] },
  { name: 'Codex', logo: 'agents/codex-color.svg', fill: c.blueLight, chaos: [1010, 610] },
  { name: 'Pi', logo: 'agents/pi-color.svg', fill: c.yellow, chaos: [1590, 300] },
  // Joins later to show that switching tools keeps your setup.
  { name: 'Antigravity', logo: 'agents/antigravity-color.svg', fill: c.blueLight, chaos: [1620, TOOL_Y] },
];

const ROW3 = [380, 960, 1540];
const ROW4 = [300, 740, 1180, 1620];

export const joinProgress = (frame: number) => progress(frame, T.join, 0.5);

export const toolX = (i: number, frame: number) => (i < 3 ? lerp(ROW3[i], ROW4[i], joinProgress(frame)) : ROW4[3]);

// When the change reaches each tool.
export const arriveAt = (i: number) => (i < 3 ? T.pulses + i * 0.1 + T.pulseDur : T.join + 1.6);
