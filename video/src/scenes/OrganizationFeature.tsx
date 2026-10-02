import { AbsoluteFill, useCurrentFrame, useVideoConfig, spring, interpolate } from 'remotion';
import { MacTerminal } from '../components/MacTerminal';
import { Typewriter } from '../components/Typewriter';
import { colors } from '../styles/colors';

const FPS = 30;

export const OrganizationFeature = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  // Timeline (1.5x speed):
  // 0-0.3s: Title flies in
  // 0.3-0.7s: Terminal + Organization visual appear
  // 0.7-2.3s: Typewriter animation
  // 2.3-2.7s: Hold

  // Main title entrance
  const titleEntrance = spring({
    frame,
    fps,
    config: { damping: 12, stiffness: 150 },
  });

  // Content entrance
  const contentEntrance = spring({
    frame: frame - 0.25 * fps,
    fps,
    config: { damping: 15 },
  });

  // Connection pulse animation
  const pulsePhase = (frame % 30) / 30;
  const pulseOpacity = 0.3 + 0.4 * Math.sin(pulsePhase * Math.PI * 2);

  const command = 'skillshare install org/skills --track';

  return (
    <AbsoluteFill
      style={{
        backgroundColor: colors.bgDark,
      }}
    >
      {/* Glowing background effect */}
      <div
        style={{
          position: 'absolute',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          width: 800,
          height: 800,
          borderRadius: '50%',
          background: `radial-gradient(circle, ${colors.primary}15 0%, transparent 70%)`,
          opacity: pulseOpacity,
        }}
      />

      {/* Main title - top */}
      <div
        style={{
          position: 'absolute',
          top: 80,
          left: '50%',
          transform: `translateX(-50%) scale(${titleEntrance}) translateY(${(1 - titleEntrance) * -30}px)`,
        }}
      >
        <h1
          style={{
            fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
            fontSize: '72px',
            fontWeight: 700,
            color: colors.textPrimary,
            margin: 0,
            textShadow: `0 0 60px ${colors.primary}60`,
          }}
        >
          Organization Skills
        </h1>
      </div>

      {/* Content container - centered layout */}
      <div
        style={{
          position: 'absolute',
          top: '50%',
          left: '50%',
          transform: `translate(-50%, -50%) scale(${contentEntrance})`,
          opacity: contentEntrance,
          display: 'flex',
          alignItems: 'center',
          gap: '60px',
        }}
      >
        {/* Left: Terminal with typewriter */}
        <MacTerminal title="Terminal" width={680} enterDelay={0}>
          <Typewriter
            text={command}
            startFrame={0.5 * fps}
            speed={1}
            showCursor={true}
          />
        </MacTerminal>

        {/* Right: Organization visualization - enlarged */}
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '24px',
            position: 'relative',
            minWidth: 320,
          }}
        >
          {/* Source node with pulse - Glassmorphism style */}
          <div style={{ position: 'relative' }}>
            {/* Pulse ring */}
            <PulseRing />
            {/* Glow layer */}
            <div
              style={{
                position: 'absolute',
                inset: -15,
                borderRadius: '50%',
                background: `radial-gradient(circle, ${colors.primary}40 0%, transparent 70%)`,
                filter: 'blur(12px)',
                zIndex: 1,
              }}
            />
            {/* Glass card */}
            <div
              style={{
                position: 'relative',
                width: 110,
                height: 110,
                borderRadius: '24px',
                background: `linear-gradient(135deg, rgba(99, 102, 241, 0.25) 0%, rgba(99, 102, 241, 0.1) 100%)`,
                backdropFilter: 'blur(16px)',
                WebkitBackdropFilter: 'blur(16px)',
                border: '1px solid rgba(255, 255, 255, 0.18)',
                boxShadow: `
                  0 8px 32px rgba(0, 0, 0, 0.3),
                  0 0 30px ${colors.primary}30,
                  inset 0 1px 0 rgba(255, 255, 255, 0.1)
                `,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 6,
                zIndex: 2,
              }}
            >
              {/* Folder icon */}
              <svg width="36" height="36" viewBox="0 0 24 24" fill="none">
                <path
                  d="M3 7V17C3 18.1046 3.89543 19 5 19H19C20.1046 19 21 18.1046 21 17V9C21 7.89543 20.1046 7 19 7H13L11 5H5C3.89543 5 3 5.89543 3 7Z"
                  stroke="rgba(255, 255, 255, 0.9)"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              <span
                style={{
                  color: 'rgba(255, 255, 255, 0.95)',
                  fontSize: '14px',
                  fontWeight: 600,
                  fontFamily: '-apple-system, sans-serif',
                  textShadow: '0 1px 2px rgba(0,0,0,0.3)',
                }}
              >
                Source
              </span>
            </div>
          </div>

          {/* Connection lines with animated particles */}
          <svg width="220" height="50" style={{ overflow: 'visible' }}>
            <defs>
              <linearGradient id="lineGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stopColor={colors.primary} stopOpacity="0.8" />
                <stop offset="100%" stopColor={colors.primaryLight} stopOpacity="0.4" />
              </linearGradient>
            </defs>
            {/* Lines */}
            <line x1="110" y1="0" x2="30" y2="50" stroke="url(#lineGradient)" strokeWidth="2.5" />
            <line x1="110" y1="0" x2="110" y2="50" stroke="url(#lineGradient)" strokeWidth="2.5" />
            <line x1="110" y1="0" x2="190" y2="50" stroke="url(#lineGradient)" strokeWidth="2.5" />

            {/* Animated particles */}
            <SyncParticle startX={110} startY={0} endX={30} endY={50} delay={0} />
            <SyncParticle startX={110} startY={0} endX={110} endY={50} delay={8} />
            <SyncParticle startX={110} startY={0} endX={190} endY={50} delay={16} />
          </svg>

          {/* Organization members with sync animation */}
          <div style={{ display: 'flex', gap: '24px' }}>
            {[0, 1, 2].map((i) => (
              <OrgMember key={i} index={i} />
            ))}
          </div>

          {/* Label */}
          <span
            style={{
              color: colors.textPrimary,
              fontSize: '22px',
              fontWeight: 600,
              fontFamily: '-apple-system, sans-serif',
              marginTop: '8px',
            }}
          >
            Share across your organization
          </span>
        </div>
      </div>
    </AbsoluteFill>
  );
};

// Pulse ring animation around source
const PulseRing = () => {
  const frame = useCurrentFrame();

  // Repeating pulse every 45 frames
  const cycleFrame = frame % 45;
  const pulseProgress = interpolate(cycleFrame, [0, 45], [0, 1]);
  const scale = interpolate(pulseProgress, [0, 1], [1, 1.8]);
  const opacity = interpolate(pulseProgress, [0, 0.5, 1], [0.5, 0.25, 0]);

  return (
    <div
      style={{
        position: 'absolute',
        top: '50%',
        left: '50%',
        width: 110,
        height: 110,
        borderRadius: '24px',
        border: `2px solid ${colors.primary}`,
        transform: `translate(-50%, -50%) scale(${scale})`,
        opacity,
        zIndex: 0,
      }}
    />
  );
};

// Animated sync particle
const SyncParticle = ({
  startX,
  startY,
  endX,
  endY,
  delay,
}: {
  startX: number;
  startY: number;
  endX: number;
  endY: number;
  delay: number;
}) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  // Start animation after typing starts (1.5x speed)
  const animStart = 1 * fps + delay;
  const cycleDuration = 30;
  const cycleFrame = Math.max(0, (frame - animStart) % cycleDuration);
  const progress = interpolate(cycleFrame, [0, cycleDuration], [0, 1]);

  const x = interpolate(progress, [0, 1], [startX, endX]);
  const y = interpolate(progress, [0, 1], [startY, endY]);
  const opacity = frame >= animStart ? interpolate(progress, [0, 0.5, 1], [0, 1, 0]) : 0;

  return (
    <circle
      cx={x}
      cy={y}
      r="5"
      fill={colors.primaryLight}
      opacity={opacity}
      style={{ filter: `drop-shadow(0 0 8px ${colors.primary})` }}
    />
  );
};

// Organization member avatar component with sync checkmark - enlarged
const OrgMember = ({ index }: { index: number }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const entrance = spring({
    frame: frame - 0.6 * fps - index * 4,
    fps,
    config: { damping: 12 },
  });

  // Checkmark appears after sync particle reaches (1.5x speed)
  const checkDelay = 1.5 * fps + index * 7;
  const checkProgress = spring({
    frame: frame - checkDelay,
    fps,
    config: { damping: 10, stiffness: 180 }, // Bouncy checkmark
  });

  return (
    <div style={{ position: 'relative' }}>
      <div
        style={{
          width: 72,
          height: 72,
          borderRadius: '50%',
          backgroundColor: colors.textSecondary,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          transform: `scale(${entrance})`,
          boxShadow: '0 6px 20px rgba(0,0,0,0.35)',
        }}
      >
        <svg width="36" height="36" viewBox="0 0 24 24" fill="none">
          <circle cx="12" cy="8" r="4" fill={colors.bgDark} />
          <path
            d="M4 20c0-4 4-6 8-6s8 2 8 6"
            stroke={colors.bgDark}
            strokeWidth="2.5"
            strokeLinecap="round"
          />
        </svg>
      </div>

      {/* Checkmark badge */}
      {checkProgress > 0 && (
        <div
          style={{
            position: 'absolute',
            top: -4,
            right: -4,
            width: 28,
            height: 28,
            borderRadius: '50%',
            backgroundColor: colors.success,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            transform: `scale(${checkProgress})`,
            boxShadow: `0 0 16px ${colors.success}60`,
          }}
        >
          <span style={{ color: '#fff', fontSize: '16px', fontWeight: 'bold' }}>✓</span>
        </div>
      )}
    </div>
  );
};
