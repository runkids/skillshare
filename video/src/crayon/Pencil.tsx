import { c } from './palette';
import { Shape } from './Crayon';

// Logo-style pencil, tip at (0,0), pointing down-left. Place with a transform.
export const Pencil = ({ x, y, scale = 1, rotate = -35 }: { x: number; y: number; scale?: number; rotate?: number }) => (
  <g transform={`translate(${x},${y}) rotate(${rotate}) scale(${scale})`}>
    <Shape d="M-26,-60 L26,-60 L26,-330 L-26,-330 Z" fill={c.orange} />
    <Shape d="M-26,-160 L0,-160 L0,-330 L-26,-330 Z" fill={c.gold} outline={0} />
    <Shape d="M-26,-330 L26,-330 L26,-372 Q0,-392 -26,-372 Z" fill="#F4A3B4" />
    <Shape d="M-26,-60 L26,-60 L0,0 Z" fill="#F6D2A2" />
    <Shape d="M-9,-21 L9,-21 L0,0 Z" fill={c.navy} outline={3} />
  </g>
);
