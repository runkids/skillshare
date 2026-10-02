import { AbsoluteFill, useCurrentFrame, useVideoConfig, spring, interpolate, Easing } from 'remotion';
import { AIIcon, AIToolName } from '../components/AIIcon';
import { colors } from '../styles/colors';

const FPS = 30;

// Target positions arranged in a circle around center (4 tools)
const targetPositions: { name: AIToolName; angle: number; delay: number }[] = [
  { name: 'claude', angle: -90, delay: 0 },
  { name: 'opencode', angle: 0, delay: 4 },
  { name: 'codex', angle: 90, delay: 8 },
  { name: 'antigravity', angle: 180, delay: 12 },
];

const RADIUS = 280;
const CENTER_X = 960;
const CENTER_Y = 420; // Move up to make room for text

export const Architecture = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  // Timeline (compressed to 2.7s):
  // 0-0.4s: Source block appears with label
  // 0.4-1s: Targets fly in
  // 1-1.6s: Arrows animate
  // 1.3s: Checkmarks start appearing
  // 1.5-1.8s: "All in sync" text fades in
  // 1.8-2.7s: Full display (0.9s visibility)

  // Source entrance
  const sourceEntrance = spring({
    frame,
    fps,
    config: { damping: 15, stiffness: 200 },
  });

  // Source breathing pulse effect (continuous after entrance)
  const breathePhase = (frame - 0.3 * fps) * 0.08;
  const breatheScale = sourceEntrance > 0.9 ? 1 + 0.03 * Math.sin(breathePhase) : sourceEntrance;
  const breatheGlow = sourceEntrance > 0.9 ? 60 + 30 * Math.sin(breathePhase) : 60 * sourceEntrance;

  // Source label with spring entrance
  const labelEntrance = spring({
    frame: frame - 0.2 * fps,
    fps,
    config: { damping: 12, stiffness: 150 },
  });
  const labelOpacity = interpolate(labelEntrance, [0, 1], [0, 1], {
    extrapolateRight: 'clamp',
  });
  const labelTranslateY = interpolate(labelEntrance, [0, 1], [15, 0], {
    extrapolateRight: 'clamp',
  });

  // Arrow animation (1-1.8s)
  const arrowStart = 1 * fps;
  const arrowProgress = interpolate(frame, [arrowStart, arrowStart + 0.6 * fps], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: Easing.out(Easing.quad),
  });

  // Checkmark animation (1.3-2.7s, earlier for more impact)
  const checkStart = 1.3 * fps;
  const checkProgress = spring({
    frame: frame - checkStart,
    fps,
    config: { damping: 12 },
  });

  // "All in sync" text with spring entrance
  const syncTextEntrance = spring({
    frame: frame - 1.5 * fps,
    fps,
    config: { damping: 12, stiffness: 150 },
  });
  const syncTextOpacity = interpolate(syncTextEntrance, [0, 1], [0, 1], {
    extrapolateRight: 'clamp',
  });
  const syncTextScale = interpolate(syncTextEntrance, [0, 1], [0.8, 1], {
    extrapolateRight: 'clamp',
  });

  // Particle flow along arrows (continuous after arrows appear)
  const particleCycleDuration = 40; // frames per cycle
  const getParticlePosition = (angle: number, particleIndex: number) => {
    if (arrowProgress < 0.5) return null;
    const cycleOffset = particleIndex * (particleCycleDuration / 3);
    const cycleProgress = ((frame - arrowStart + cycleOffset) % particleCycleDuration) / particleCycleDuration;

    const targetPos = getTargetPosition(angle);
    const targetX = targetPos.x + 45;
    const targetY = targetPos.y + 45;

    // Particle moves from center to target
    const particleX = CENTER_X + (targetX - CENTER_X) * cycleProgress;
    const particleY = CENTER_Y + (targetY - CENTER_Y) * cycleProgress;
    const particleOpacity = interpolate(cycleProgress, [0, 0.3, 0.7, 1], [0, 1, 1, 0]);

    return { x: particleX, y: particleY, opacity: particleOpacity };
  };

  // Calculate target positions
  const getTargetPosition = (angle: number) => {
    const rad = (angle * Math.PI) / 180;
    return {
      x: CENTER_X + RADIUS * Math.cos(rad) - 45,
      y: CENTER_Y + RADIUS * Math.sin(rad) - 45,
    };
  };

  return (
    <AbsoluteFill style={{ backgroundColor: colors.bgDark }}>
      {/* Source block (center) - Glassmorphism style */}
      <div
        style={{
          position: 'absolute',
          left: CENTER_X - 80,
          top: CENTER_Y - 80,
          transform: `scale(${breatheScale})`,
        }}
      >
        {/* Glow layer behind */}
        <div
          style={{
            position: 'absolute',
            inset: -20,
            borderRadius: '50%',
            background: `radial-gradient(circle, ${colors.primary}40 0%, transparent 70%)`,
            filter: `blur(${breatheGlow * 0.3}px)`,
          }}
        />
        {/* Glass card */}
        <div
          style={{
            position: 'relative',
            width: 160,
            height: 160,
            borderRadius: '28px',
            background: `linear-gradient(135deg, rgba(99, 102, 241, 0.25) 0%, rgba(99, 102, 241, 0.1) 100%)`,
            backdropFilter: 'blur(20px)',
            WebkitBackdropFilter: 'blur(20px)',
            border: '1px solid rgba(255, 255, 255, 0.18)',
            boxShadow: `
              0 8px 32px rgba(0, 0, 0, 0.3),
              0 0 ${breatheGlow}px ${colors.primary}30,
              inset 0 1px 0 rgba(255, 255, 255, 0.1)
            `,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 8,
          }}
        >
          {/* Folder/Sync icon */}
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none">
            <path
              d="M3 7V17C3 18.1046 3.89543 19 5 19H19C20.1046 19 21 18.1046 21 17V9C21 7.89543 20.1046 7 19 7H13L11 5H5C3.89543 5 3 5.89543 3 7Z"
              stroke="rgba(255, 255, 255, 0.9)"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
            <circle
              cx="12"
              cy="13"
              r="2.5"
              stroke={colors.primaryLight}
              strokeWidth="1.5"
            />
            <path
              d="M12 10.5V8"
              stroke={colors.primaryLight}
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
          <span
            style={{
              color: 'rgba(255, 255, 255, 0.95)',
              fontSize: '18px',
              fontWeight: 600,
              fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
              letterSpacing: '0.5px',
              textShadow: '0 1px 2px rgba(0,0,0,0.3)',
            }}
          >
            Source
          </span>
        </div>
      </div>

      {/* Source label */}
      <div
        style={{
          position: 'absolute',
          left: CENTER_X,
          top: CENTER_Y + 100,
          transform: `translateX(-50%) translateY(${labelTranslateY}px)`,
          opacity: labelOpacity,
        }}
      >
        <span
          style={{
            color: colors.textSecondary,
            fontSize: '18px',
            fontFamily: '"JetBrains Mono", monospace',
          }}
        >
          Single Source of Truth
        </span>
      </div>

      {/* Arrows (SVG) */}
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
        <defs>
          <linearGradient id="arrowGradient" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor={colors.primary} />
            <stop offset="100%" stopColor={colors.primaryLight} />
          </linearGradient>
          <filter id="particleGlow" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="3" result="coloredBlur" />
            <feMerge>
              <feMergeNode in="coloredBlur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>
        {targetPositions.map(({ name, angle }) => {
          const pos = getTargetPosition(angle);
          const targetX = pos.x + 45;
          const targetY = pos.y + 45;

          // Animated arrow length
          const currentX = CENTER_X + (targetX - CENTER_X) * arrowProgress;
          const currentY = CENTER_Y + (targetY - CENTER_Y) * arrowProgress;

          return (
            <g key={name}>
              <line
                x1={CENTER_X}
                y1={CENTER_Y}
                x2={currentX}
                y2={currentY}
                stroke="url(#arrowGradient)"
                strokeWidth="3"
                opacity={arrowProgress > 0 ? 0.8 : 0}
              />
              {/* Flowing particles along the arrow */}
              {[0, 1, 2].map((particleIndex) => {
                const particle = getParticlePosition(angle, particleIndex);
                if (!particle) return null;
                return (
                  <circle
                    key={particleIndex}
                    cx={particle.x}
                    cy={particle.y}
                    r={4}
                    fill={colors.primaryLight}
                    opacity={particle.opacity * 0.9}
                    filter="url(#particleGlow)"
                  />
                );
              })}
            </g>
          );
        })}
      </svg>

      {/* Target icons */}
      {targetPositions.map(({ name, angle, delay }) => {
        const pos = getTargetPosition(angle);

        // Target entrance (0.4-1s, compressed)
        const targetEntrance = spring({
          frame: frame - 0.4 * fps - delay * 0.4,
          fps,
          config: { damping: 12, stiffness: 150 },
        });

        return (
          <div
            key={name}
            style={{
              position: 'absolute',
              left: pos.x,
              top: pos.y,
              transform: `scale(${targetEntrance})`,
            }}
          >
            <AIIcon name={name} size={90} enterDelay={0} showLabel={false} />

            {/* Checkmark overlay */}
            {checkProgress > 0 && (
              <div
                style={{
                  position: 'absolute',
                  top: -10,
                  right: -10,
                  width: 32,
                  height: 32,
                  borderRadius: '50%',
                  backgroundColor: colors.success,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  transform: `scale(${checkProgress})`,
                  boxShadow: `0 0 20px ${colors.success}60`,
                }}
              >
                <span style={{ color: '#fff', fontSize: '18px', fontWeight: 'bold' }}>✓</span>
              </div>
            )}
          </div>
        );
      })}

      {/* "All in sync" text */}
      <div
        style={{
          position: 'absolute',
          bottom: 80,
          left: '50%',
          transform: `translateX(-50%) scale(${syncTextScale})`,
          opacity: syncTextOpacity,
          zIndex: 100,
        }}
      >
        <h2
          style={{
            fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
            fontSize: '56px',
            fontWeight: 700,
            color: colors.success,
            margin: 0,
            textShadow: `0 0 40px ${colors.success}60, 0 4px 20px rgba(0,0,0,0.5)`,
            background: `linear-gradient(180deg, ${colors.success} 0%, #16a34a 100%)`,
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
            backgroundClip: 'text',
          }}
        >
          All in sync ✓
        </h2>
      </div>
    </AbsoluteFill>
  );
};
