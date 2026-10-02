import { AbsoluteFill, useCurrentFrame, useVideoConfig, spring, interpolate } from 'remotion';
import { Typewriter } from '../components/Typewriter';
import { colors } from '../styles/colors';

const FPS = 30;

// Claude brand orange color
const CLAUDE_ORANGE = '#d97706';

export const AIFeature = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  // Timeline (4s):
  // 0-0.3s: Title flies in
  // 0.3-0.5s: Chat container appears
  // 0.5-2.2s: User message types (34 chars * 1.5 speed = ~1.7s)
  // 2.4s: Claude response row + thinking
  // 3.0s: Claude executes command
  // 3.5s: Success message
  // 4s: Hold

  // Calculate when user typing finishes
  const userText = "Sync my skills to all my AI tools";
  const typeSpeed = 1.0; // faster typing
  const typeStartFrame = 0.5 * fps;
  const typeEndFrame = typeStartFrame + userText.length * typeSpeed; // ~49 frames = 1.6s

  // Title entrance
  const titleEntrance = spring({
    frame,
    fps,
    config: { damping: 12, stiffness: 150 },
  });

  // Chat container entrance
  const chatEntrance = spring({
    frame: frame - 0.3 * fps,
    fps,
    config: { damping: 15 },
  });

  // User message entrance
  const userMsgEntrance = spring({
    frame: frame - 0.5 * fps,
    fps,
    config: { damping: 15 },
  });

  // Claude response entrance - wait for user to finish typing
  const claudeResponseEntrance = spring({
    frame: frame - (typeEndFrame + 3), // 3 frames after typing ends
    fps,
    config: { damping: 12 },
  });

  // Command execution - after thinking for a bit
  const commandEntrance = spring({
    frame: frame - (typeEndFrame + 12), // ~0.4s after Claude appears
    fps,
    config: { damping: 12 },
  });

  // Success checkmark with bouncy entrance
  const successEntrance = spring({
    frame: frame - (typeEndFrame + 22), // ~0.7s after Claude appears
    fps,
    config: { damping: 8, stiffness: 180 },
  });

  // Success glow pulse
  const successGlowPhase = (frame - (typeEndFrame + 27)) * 0.12;
  const successGlowIntensity = successEntrance > 0.8 ? 16 + 12 * Math.sin(successGlowPhase) : 16;

  return (
    <AbsoluteFill style={{ backgroundColor: colors.bgDark }}>
      {/* Main title */}
      <div
        style={{
          position: 'absolute',
          top: 60,
          left: '50%',
          transform: `translateX(-50%) scale(${titleEntrance}) translateY(${(1 - titleEntrance) * -30}px)`,
        }}
      >
        <h1
          style={{
            fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
            fontSize: '64px',
            fontWeight: 700,
            color: colors.textPrimary,
            margin: 0,
            textShadow: `0 0 60px ${colors.primary}60`,
          }}
        >
          Built-in AI
        </h1>
      </div>

      {/* Claude-style Chat interface */}
      <div
        style={{
          position: 'absolute',
          top: '50%',
          left: '50%',
          transform: `translate(-50%, -50%) scale(${chatEntrance})`,
          opacity: chatEntrance,
          width: 1200,
          backgroundColor: '#1c1c1c',
          borderRadius: 20,
          overflow: 'hidden',
          boxShadow: '0 25px 80px rgba(0,0,0,0.6)',
        }}
      >
        {/* User message row */}
        <div
          style={{
            backgroundColor: '#2a2a2a',
            padding: '36px 50px',
            borderBottom: '1px solid #4a4a4a',
            opacity: userMsgEntrance,
            transform: `translateY(${(1 - userMsgEntrance) * 10}px)`,
          }}
        >
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 24 }}>
            {/* User avatar */}
            <div
              style={{
                width: 48,
                height: 48,
                borderRadius: '50%',
                backgroundColor: '#5a5a5a',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
              }}
            >
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
                <circle cx="12" cy="8" r="4" fill="#aaa" />
                <path d="M4 20c0-4 4-6 8-6s8 2 8 6" stroke="#aaa" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </div>
            {/* User message */}
            <div style={{ flex: 1, paddingTop: 8 }}>
              <Typewriter
                text={userText}
                startFrame={typeStartFrame}
                speed={typeSpeed}
                showCursor={false}
                showPrompt={false}
                style={{
                  color: '#ffffff',
                  fontSize: '32px',
                  fontWeight: 500,
                  fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
                  lineHeight: 1.5,
                }}
              />
            </div>
          </div>
        </div>

        {/* Claude response row */}
        {claudeResponseEntrance > 0 && (
          <div
            style={{
              backgroundColor: '#1c1c1c',
              padding: '36px 50px',
              opacity: claudeResponseEntrance,
              transform: `translateY(${(1 - claudeResponseEntrance) * 10}px)`,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 24 }}>
              {/* Claude avatar */}
              <div
                style={{
                  width: 48,
                  height: 48,
                  borderRadius: 12,
                  background: `linear-gradient(135deg, ${CLAUDE_ORANGE}, #f59e0b)`,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  flexShrink: 0,
                  boxShadow: `0 4px 12px ${CLAUDE_ORANGE}40`,
                }}
              >
                <ClaudeLogo />
              </div>

              {/* Claude message content */}
              <div style={{ flex: 1, paddingTop: 4 }}>
                {commandEntrance > 0 ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
                    {/* Response text */}
                    <span
                      style={{
                        color: '#ffffff',
                        fontSize: '28px',
                        fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
                        lineHeight: 1.6,
                      }}
                    >
                      I'll sync your skills now. Running the command:
                    </span>

                    {/* Code block - Claude style */}
                    <div
                      style={{
                        backgroundColor: '#0d0d0d',
                        borderRadius: 10,
                        padding: '20px 28px',
                        border: '1px solid #444',
                        transform: `scale(${commandEntrance})`,
                        transformOrigin: 'left',
                      }}
                    >
                      <code
                        style={{
                          fontFamily: '"JetBrains Mono", "SF Mono", monospace',
                          fontSize: '26px',
                          color: '#4ade80',
                        }}
                      >
                        $ skillshare sync
                      </code>
                    </div>

                    {/* Success message */}
                    {successEntrance > 0 && (
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 14,
                          transform: `scale(${successEntrance})`,
                          transformOrigin: 'left',
                        }}
                      >
                        <div
                          style={{
                            width: 32,
                            height: 32,
                            borderRadius: '50%',
                            backgroundColor: '#22c55e',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            boxShadow: `0 0 ${successGlowIntensity}px #22c55e80`,
                          }}
                        >
                          <span style={{ color: '#fff', fontSize: '18px', fontWeight: 'bold' }}>✓</span>
                        </div>
                        <span
                          style={{
                            color: '#4ade80',
                            fontSize: '26px',
                            fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
                            fontWeight: 600,
                          }}
                        >
                          Synced 5 skills to 4 AI tools
                        </span>
                      </div>
                    )}
                  </div>
                ) : (
                  <ThinkingIndicator />
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </AbsoluteFill>
  );
};

// Claude sparkle logo (simplified)
const ClaudeLogo = () => (
  <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
    <path
      d="M12 2L14.5 9.5L22 12L14.5 14.5L12 22L9.5 14.5L2 12L9.5 9.5L12 2Z"
      fill="#fff"
      opacity="0.95"
    />
  </svg>
);

// Claude-style thinking indicator
const ThinkingIndicator = () => {
  const frame = useCurrentFrame();

  // Pulsing animation
  const pulse = Math.sin(frame * 0.15) * 0.5 + 0.5;

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
      {/* Pulsing cursor */}
      <div
        style={{
          width: 3,
          height: 24,
          backgroundColor: CLAUDE_ORANGE,
          borderRadius: 2,
          opacity: 0.4 + pulse * 0.6,
        }}
      />
      <span
        style={{
          color: '#888',
          fontSize: '18px',
          fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
          fontStyle: 'italic',
        }}
      >
        Thinking...
      </span>
    </div>
  );
};
