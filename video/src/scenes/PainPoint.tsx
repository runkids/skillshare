import { AbsoluteFill, useCurrentFrame, useVideoConfig, spring, interpolate, Easing } from 'remotion';
import { AIIcon, AIToolName } from '../components/AIIcon';
import { colors } from '../styles/colors';

const FPS = 30;

// Scattered positions for 4 icons (faster entrance)
const iconPositions: { name: AIToolName; x: number; y: number; delay: number }[] = [
  { name: 'claude', x: 300, y: 250, delay: 0 },
  { name: 'opencode', x: 1450, y: 220, delay: 3 },
  { name: 'codex', x: 350, y: 650, delay: 6 },
  { name: 'antigravity', x: 1400, y: 680, delay: 9 },
];

export const PainPoint = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  // Text animation: fade in at 1.3s with spring bounce
  const textEntrance = spring({
    frame: frame - 1.3 * fps,
    fps,
    config: { damping: 12, stiffness: 120 },
  });
  const textOpacity = interpolate(textEntrance, [0, 1], [0, 1], {
    extrapolateRight: 'clamp',
  });
  const textScale = interpolate(textEntrance, [0, 1], [0.9, 1], {
    extrapolateRight: 'clamp',
  });
  const textTranslateY = interpolate(textEntrance, [0, 1], [30, 0], {
    extrapolateRight: 'clamp',
  });

  // Question mark floating animation (continuous after text appears)
  const questionMarkFloat = textEntrance > 0.5 ? Math.sin((frame - 1.5 * fps) * 0.08) * 5 : 0;

  // Lines animation: appear at 2.3s with drawing effect
  const linesStart = 2.3 * fps;
  const linesProgress = interpolate(frame, [linesStart, linesStart + 0.5 * fps], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: Easing.out(Easing.quad),
  });

  // Line stroke animation (for drawing effect)
  const lineStrokeDashoffset = (1 - linesProgress) * 2000;

  // Wobble animation for chaos effect (0.7-2.5s, extended and enhanced)
  const wobblePhase = frame * 0.18;
  const wobbleActive = frame >= 0.7 * fps && frame < 2.5 * fps;
  const wobbleAmount = wobbleActive ? 5 : 0;
  const wobbleRotation = wobbleActive ? 2 : 0;

  return (
    <AbsoluteFill style={{ backgroundColor: colors.bgDark }}>
      {/* Scattered AI icons */}
      {iconPositions.map(({ name, x, y, delay }) => {
        const wobbleX = Math.sin(wobblePhase + delay) * wobbleAmount;
        const wobbleY = Math.cos(wobblePhase + delay * 1.3) * wobbleAmount;
        const wobbleRot = Math.sin(wobblePhase + delay * 0.7) * wobbleRotation;
        const wobbleScale = wobbleActive ? 1 + 0.03 * Math.sin(wobblePhase + delay * 1.5) : 1;

        return (
          <div
            key={name}
            style={{
              position: 'absolute',
              left: x + wobbleX,
              top: y + wobbleY,
              transform: `rotate(${wobbleRot}deg) scale(${wobbleScale})`,
            }}
          >
            <AIIcon name={name} enterDelay={delay} size={90} />
          </div>
        );
      })}

      {/* Connecting dashed lines (chaos visualization) */}
      <svg
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          pointerEvents: 'none',
        }}
      >
        {linesProgress > 0 && (
          <>
            <defs>
              <filter id="lineGlow" x="-20%" y="-20%" width="140%" height="140%">
                <feGaussianBlur stdDeviation="2" result="coloredBlur" />
                <feMerge>
                  <feMergeNode in="coloredBlur" />
                  <feMergeNode in="SourceGraphic" />
                </feMerge>
              </filter>
            </defs>
            {/* Lines that follow wobble animation */}
            {(() => {
              // Calculate wobble positions for each icon
              const getIconCenter = (i: number) => {
                const { x, y, delay } = iconPositions[i];
                const wobbleX = Math.sin(wobblePhase + delay) * wobbleAmount;
                const wobbleY = Math.cos(wobblePhase + delay * 1.3) * wobbleAmount;
                return {
                  x: x + 45 + wobbleX,
                  y: y + 45 + wobbleY,
                };
              };
              const p0 = getIconCenter(0);
              const p1 = getIconCenter(1);
              const p2 = getIconCenter(2);
              const p3 = getIconCenter(3);

              return (
                <>
                  <line
                    x1={p0.x} y1={p0.y} x2={p1.x} y2={p1.y}
                    stroke={colors.error}
                    strokeWidth="2"
                    strokeDasharray="10,10"
                    strokeDashoffset={lineStrokeDashoffset * 0.8}
                    opacity={linesProgress * 0.7}
                    filter="url(#lineGlow)"
                  />
                  <line
                    x1={p1.x} y1={p1.y} x2={p3.x} y2={p3.y}
                    stroke={colors.error}
                    strokeWidth="2"
                    strokeDasharray="10,10"
                    strokeDashoffset={lineStrokeDashoffset * 0.9}
                    opacity={linesProgress * 0.7}
                    filter="url(#lineGlow)"
                  />
                  <line
                    x1={p2.x} y1={p2.y} x2={p3.x} y2={p3.y}
                    stroke={colors.error}
                    strokeWidth="2"
                    strokeDasharray="10,10"
                    strokeDashoffset={lineStrokeDashoffset}
                    opacity={linesProgress * 0.7}
                    filter="url(#lineGlow)"
                  />
                  <line
                    x1={p0.x} y1={p0.y} x2={p2.x} y2={p2.y}
                    stroke={colors.error}
                    strokeWidth="2"
                    strokeDasharray="10,10"
                    strokeDashoffset={lineStrokeDashoffset * 1.1}
                    opacity={linesProgress * 0.7}
                    filter="url(#lineGlow)"
                  />
                </>
              );
            })()}
          </>
        )}
      </svg>

      {/* Main text */}
      <div
        style={{
          position: 'absolute',
          top: '50%',
          left: '50%',
          transform: `translate(-50%, -50%) translateY(${textTranslateY}px) scale(${textScale})`,
          opacity: textOpacity,
        }}
      >
        <h1
          style={{
            fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
            fontSize: '64px',
            fontWeight: 600,
            color: colors.textPrimary,
            textAlign: 'center',
            margin: 0,
            textShadow: '0 4px 20px rgba(0, 0, 0, 0.5)',
          }}
        >
          Skills scattered everywhere
          <span
            style={{
              display: 'inline-block',
              transform: `translateY(${questionMarkFloat}px)`,
              color: colors.error,
            }}
          >
            ?
          </span>
        </h1>
      </div>
    </AbsoluteFill>
  );
};
