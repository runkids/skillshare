import { AbsoluteFill, useCurrentFrame, useVideoConfig, spring, interpolate } from 'remotion';
import { colors } from '../styles/colors';
import { ReactNode } from 'react';

type MacTerminalProps = {
  children: ReactNode;
  title?: string;
  width?: number;
  enterDelay?: number;
};

export const MacTerminal = ({
  children,
  title = 'skillshare',
  width = 1000,
  enterDelay = 0,
}: MacTerminalProps) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  // Spring animation for entrance
  const entrance = spring({
    frame: frame - enterDelay,
    fps,
    config: { damping: 15, stiffness: 100 },
  });

  const scale = interpolate(entrance, [0, 1], [0.9, 1]);
  const translateY = interpolate(entrance, [0, 1], [50, 0]);
  const opacity = interpolate(entrance, [0, 1], [0, 1], {
    extrapolateRight: 'clamp',
  });

  return (
    <div
      style={{
        width,
        borderRadius: '12px',
        overflow: 'hidden',
        boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.5)',
        transform: `scale(${scale}) translateY(${translateY}px)`,
        opacity,
      }}
    >
      {/* Title bar */}
      <div
        style={{
          backgroundColor: '#e5e5e5',
          padding: '12px 16px',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
        }}
      >
        {/* Traffic lights */}
        <div style={{ display: 'flex', gap: '8px' }}>
          <div
            style={{
              width: '14px',
              height: '14px',
              borderRadius: '50%',
              backgroundColor: colors.trafficRed,
            }}
          />
          <div
            style={{
              width: '14px',
              height: '14px',
              borderRadius: '50%',
              backgroundColor: colors.trafficYellow,
            }}
          />
          <div
            style={{
              width: '14px',
              height: '14px',
              borderRadius: '50%',
              backgroundColor: colors.trafficGreen,
            }}
          />
        </div>

        {/* Title */}
        <div
          style={{
            flex: 1,
            textAlign: 'center',
            fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
            fontSize: '14px',
            color: '#666',
            marginRight: '54px', // Balance the traffic lights
          }}
        >
          {title}
        </div>
      </div>

      {/* Terminal content */}
      <div
        style={{
          backgroundColor: colors.terminalBg,
          padding: '24px',
          minHeight: '300px',
        }}
      >
        {children}
      </div>
    </div>
  );
};
