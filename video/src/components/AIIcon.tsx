import { useCurrentFrame, useVideoConfig, spring, interpolate } from 'remotion';
import { colors } from '../styles/colors';

export type AIToolName = 'claude' | 'opencode' | 'codex' | 'antigravity';

type AIIconProps = {
  name: AIToolName;
  size?: number;
  enterDelay?: number;
  showLabel?: boolean;
  labelText?: string;
};

const toolColors: Record<AIToolName, string> = {
  claude: '#d97706',      // Orange/amber
  opencode: '#3b82f6',    // Blue
  codex: '#10a37f',       // Green
  antigravity: '#8b5cf6', // Purple
};

const toolLabels: Record<AIToolName, string> = {
  claude: 'Claude',
  opencode: 'OpenCode',
  codex: 'Codex',
  antigravity: 'Antigravity',
};

const toolPaths: Record<AIToolName, string> = {
  claude: '~/.claude/skills',
  opencode: '~/.config/opencode/skills',
  codex: '~/.codex/skills',
  antigravity: '~/.gemini/antigravity/skills',
};

export const AIIcon = ({
  name,
  size = 80,
  enterDelay = 0,
  showLabel = true,
  labelText,
}: AIIconProps) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const entrance = spring({
    frame: frame - enterDelay,
    fps,
    config: { damping: 12, stiffness: 100 },
  });

  const scale = interpolate(entrance, [0, 1], [0, 1]);
  const opacity = interpolate(entrance, [0, 1], [0, 1], {
    extrapolateRight: 'clamp',
  });

  const bgColor = toolColors[name];

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'flex-start',
        gap: '8px',
        transform: `scale(${scale})`,
        opacity,
      }}
    >
      <div
        style={{
          width: size,
          height: size,
          borderRadius: '16px',
          backgroundColor: bgColor,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 4,
          boxShadow: `0 4px 16px rgba(0, 0, 0, 0.3), 0 0 20px ${bgColor}30`,
        }}
      >
        {/* Folder icon */}
        <svg width={size * 0.45} height={size * 0.45} viewBox="0 0 24 24" fill="none">
          <path
            d="M3 7V17C3 18.1046 3.89543 19 5 19H19C20.1046 19 21 18.1046 21 17V9C21 7.89543 20.1046 7 19 7H13L11 5H5C3.89543 5 3 5.89543 3 7Z"
            fill="rgba(255, 255, 255, 0.25)"
            stroke="rgba(255, 255, 255, 0.9)"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        <span
          style={{
            color: '#fff',
            fontSize: size * 0.15,
            fontWeight: 600,
            fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif',
            textShadow: '0 1px 2px rgba(0,0,0,0.3)',
          }}
        >
          {toolLabels[name]}
        </span>
      </div>
      {showLabel && (
        <span
          style={{
            color: colors.textSecondary,
            fontSize: '14px',
            fontFamily: '"JetBrains Mono", monospace',
          }}
        >
          {labelText || toolPaths[name]}
        </span>
      )}
    </div>
  );
};
