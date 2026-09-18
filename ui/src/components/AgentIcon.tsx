import ampColor from '@lobehub/icons-static-svg/icons/amp-color.svg?url';
import antigravityColor from '@lobehub/icons-static-svg/icons/antigravity-color.svg?url';
import baiduColor from '@lobehub/icons-static-svg/icons/baidu-color.svg?url';
import claudecodeColor from '@lobehub/icons-static-svg/icons/claudecode-color.svg?url';
import codebuddyColor from '@lobehub/icons-static-svg/icons/codebuddy-color.svg?url';
import codexColor from '@lobehub/icons-static-svg/icons/codex-color.svg?url';
import devinColor from '@lobehub/icons-static-svg/icons/devin-color.svg?url';
import geminiColor from '@lobehub/icons-static-svg/icons/gemini-color.svg?url';
import huaweiColor from '@lobehub/icons-static-svg/icons/huawei-color.svg?url';
import junieColor from '@lobehub/icons-static-svg/icons/junie-color.svg?url';
import kiroColor from '@lobehub/icons-static-svg/icons/kiro-color.svg?url';
import langchainColor from '@lobehub/icons-static-svg/icons/langchain-color.svg?url';
import mistralColor from '@lobehub/icons-static-svg/icons/mistral-color.svg?url';
import openclawColor from '@lobehub/icons-static-svg/icons/openclaw-color.svg?url';
import qwenColor from '@lobehub/icons-static-svg/icons/qwen-color.svg?url';
import replitColor from '@lobehub/icons-static-svg/icons/replit-color.svg?url';
import snowflakeColor from '@lobehub/icons-static-svg/icons/snowflake-color.svg?url';
import traeColor from '@lobehub/icons-static-svg/icons/trae-color.svg?url';
import zencoderColor from '@lobehub/icons-static-svg/icons/zencoder-color.svg?url';
import clineMono from '@lobehub/icons-static-svg/icons/cline.svg?url';
import cursorMono from '@lobehub/icons-static-svg/icons/cursor.svg?url';
import githubcopilotMono from '@lobehub/icons-static-svg/icons/githubcopilot.svg?url';
import gooseMono from '@lobehub/icons-static-svg/icons/goose.svg?url';
import grokMono from '@lobehub/icons-static-svg/icons/grok.svg?url';
import ibmMono from '@lobehub/icons-static-svg/icons/ibm.svg?url';
import kilocodeMono from '@lobehub/icons-static-svg/icons/kilocode.svg?url';
import kimiMono from '@lobehub/icons-static-svg/icons/kimi.svg?url';
import nousresearchMono from '@lobehub/icons-static-svg/icons/nousresearch.svg?url';
import opencodeMono from '@lobehub/icons-static-svg/icons/opencode.svg?url';
import openhandsMono from '@lobehub/icons-static-svg/icons/openhands.svg?url';
import piMono from '@lobehub/icons-static-svg/icons/pi.svg?url';
import qoderMono from '@lobehub/icons-static-svg/icons/qoder.svg?url';
import roocodeMono from '@lobehub/icons-static-svg/icons/roocode.svg?url';
import windsurfMono from '@lobehub/icons-static-svg/icons/windsurf.svg?url';
// Not in lobehub; from simple-icons (CC0).
import warpMono from '../assets/agents/warp.svg?url';
import zedMono from '../assets/agents/zed.svg?url';

// Brand marks from @lobehub/icons-static-svg, keyed by target name. Parent-company
// marks stand in where the product has none (cortex, vibe, comate, codearts, …).
const colored: Record<string, string> = {
  amp: ampColor,
  antigravity: antigravityColor,
  'antigravity-cli': antigravityColor,
  claude: claudecodeColor,
  codearts: huaweiColor,
  codebuddy: codebuddyColor,
  codex: codexColor,
  comate: baiduColor,
  cortex: snowflakeColor,
  deepagents: langchainColor,
  devin: devinColor,
  gemini: geminiColor,
  junie: junieColor,
  kiro: kiroColor,
  openclaw: openclawColor,
  qwen: qwenColor,
  replit: replitColor,
  trae: traeColor,
  'trae-cn': traeColor,
  vibe: mistralColor,
  'xcode-claude': claudecodeColor,
  'xcode-codex': codexColor,
  zencoder: zencoderColor,
};
// Black/white marks (or color files that rely on black/currentColor) are masked
// over currentColor so they follow the text color in light and dark themes.
const mono: Record<string, string> = {
  bob: ibmMono,
  cline: clineMono,
  copilot: githubcopilotMono,
  cursor: cursorMono,
  goose: gooseMono,
  grok: grokMono,
  hermes: nousresearchMono,
  kilocode: kilocodeMono,
  kimi: kimiMono,
  opencode: opencodeMono,
  openhands: openhandsMono,
  pi: piMono,
  qoder: qoderMono,
  roo: roocodeMono,
  vscode: githubcopilotMono,
  warp: warpMono,
  windsurf: windsurfMono,
  zed: zedMono,
};

export default function AgentIcon({ target, size = 16 }: { target: string; size?: number }) {
  // universal is the cross-client ~/.agents/skills convention: the Agent Skills hexagon in a
  // multi-color gradient so the shared path stands apart from single-vendor marks.
  if (target === 'universal') {
    return <span aria-hidden="true" className="inline-block shrink-0 bg-linear-135/oklch from-[#dc4538] via-[#f0b840] to-[#5cc98a] [clip-path:polygon(50%_2%,92%_26%,92%_74%,50%_98%,8%_74%,8%_26%)]" style={{ width: size, height: size }} />;
  }
  if (colored[target]) return <img src={colored[target]} alt="" aria-hidden="true" width={size} height={size} className="shrink-0" />;
  const src = mono[target];
  if (!src) {
    return <span aria-hidden="true" className="inline-flex items-center justify-center shrink-0 rounded-[var(--radius-sm)] bg-muted/60 font-bold uppercase text-pencil-light" style={{ width: size, height: size, fontSize: size * 0.55 }}>{target[0]}</span>;
  }
  const mask = `url("${src}") center / contain no-repeat`;
  return <span aria-hidden="true" className="inline-block shrink-0 bg-current" style={{ width: size, height: size, mask, WebkitMask: mask }} />;
}
