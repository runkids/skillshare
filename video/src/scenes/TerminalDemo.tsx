import { AbsoluteFill, useCurrentFrame, useVideoConfig, spring, interpolate } from 'remotion';
import { MacTerminal } from '../components/MacTerminal';
import { Typewriter } from '../components/Typewriter';
import { Cursor } from '../components/Cursor';
import { colors } from '../styles/colors';

const FPS = 30;

// Output lines for init command (updated tool names)
const initOutputLines = [
  { text: '✓ Detected: claude, opencode, codex, antigravity', color: colors.terminalGreen },
  { text: '✓ Created ~/.config/skillshare/skills', color: colors.terminalGreen },
  { text: '✓ Ready to sync', color: colors.terminalGreen },
];

// Output for sync command
const syncOutputLine = { text: '✓ Synced 5 skills to 4 targets', color: colors.terminalGreen };

export const TerminalDemo = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  // Timeline (1.5x speed):
  // 0-0.7s: Terminal enters
  // 0.7-1.8s: Type "skillshare init"
  // 1.8-2.7s: Show init output
  // 2.7-4s: Type "skillshare sync"
  // 4-5.3s: Show sync output

  const initCommand = 'skillshare init';
  const syncCommand = 'skillshare sync';

  // Command typing timings (1.5x speed)
  const initTypeStart = 0.7 * fps;
  const initTypeDuration = initCommand.length * 1.2; // 1.2 frames per char
  const initOutputStart = initTypeStart + initTypeDuration + 8;

  const syncTypeStart = 2.7 * fps;
  const syncTypeDuration = syncCommand.length * 1.2;
  const syncOutputStart = syncTypeStart + syncTypeDuration + 8;

  // Calculate which output lines to show
  const showInitOutput = frame >= initOutputStart;
  const showSyncOutput = frame >= syncOutputStart;

  // Output line stagger animation with slide-in effect
  const getLineAnimation = (lineIndex: number, outputStart: number) => {
    const lineDelay = lineIndex * 5; // 5 frames between each line
    const lineFrame = frame - outputStart - lineDelay;
    if (lineFrame < 0) return { opacity: 0, transform: 'translateX(-20px)', scale: 0.95 };

    const progress = spring({
      frame: lineFrame,
      fps,
      config: { damping: 15, stiffness: 200 },
    });

    const opacity = interpolate(progress, [0, 1], [0, 1], {
      extrapolateRight: 'clamp',
    });
    const translateX = interpolate(progress, [0, 1], [-20, 0], {
      extrapolateRight: 'clamp',
    });
    const scale = interpolate(progress, [0, 1], [0.95, 1], {
      extrapolateRight: 'clamp',
    });

    return { opacity, transform: `translateX(${translateX}px)`, scale };
  };


  return (
    <AbsoluteFill
      style={{
        backgroundColor: colors.bgDark,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <MacTerminal title="skillshare — Terminal" width={1100} enterDelay={0}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {/* Init command */}
          <Typewriter
            text={initCommand}
            startFrame={initTypeStart}
            speed={2}
            showCursor={!showInitOutput && frame >= initTypeStart}
          />

          {/* Init output */}
          {showInitOutput && (
            <div style={{ marginTop: '8px', marginBottom: '16px' }}>
              {initOutputLines.map((line, i) => {
                const anim = getLineAnimation(i, initOutputStart);

                return (
                  <div
                    key={i}
                    style={{
                      fontFamily: '"JetBrains Mono", monospace',
                      fontSize: '20px',
                      color: line.color,
                      opacity: anim.opacity,
                      transform: `${anim.transform} scale(${anim.scale})`,
                      marginBottom: '4px',
                      transformOrigin: 'left center',
                    }}
                  >
                    {line.text}
                  </div>
                );
              })}
            </div>
          )}

          {/* Sync command */}
          {frame >= syncTypeStart - 10 && (
            <Typewriter
              text={syncCommand}
              startFrame={syncTypeStart}
              speed={2}
              showCursor={!showSyncOutput && frame >= syncTypeStart}
            />
          )}

          {/* Sync output */}
          {showSyncOutput && (() => {
            const anim = getLineAnimation(0, syncOutputStart);

            return (
              <div
                style={{
                  fontFamily: '"JetBrains Mono", monospace',
                  fontSize: '20px',
                  color: syncOutputLine.color,
                  opacity: anim.opacity,
                  transform: `${anim.transform} scale(${anim.scale})`,
                  marginTop: '8px',
                  transformOrigin: 'left center',
                }}
              >
                {syncOutputLine.text}
              </div>
            );
          })()}

          {/* Final cursor */}
          {showSyncOutput && frame >= syncOutputStart + 15 && (
            <div style={{ marginTop: '8px' }}>
              <span
                style={{
                  fontFamily: '"JetBrains Mono", monospace',
                  fontSize: '24px',
                  color: colors.terminalGreen,
                }}
              >
                ${' '}
              </span>
              <Cursor />
            </div>
          )}
        </div>
      </MacTerminal>
    </AbsoluteFill>
  );
};
