import { useContext } from 'react';
import { TargetAgents } from './targetAgents';
import alibabaColor from '@lobehub/icons-static-svg/icons/alibaba-color.svg?url';
import alibabacloudColor from '@lobehub/icons-static-svg/icons/alibabacloud-color.svg?url';
import ampColor from '@lobehub/icons-static-svg/icons/amp-color.svg?url';
import antigravityColor from '@lobehub/icons-static-svg/icons/antigravity-color.svg?url';
import baiduColor from '@lobehub/icons-static-svg/icons/baidu-color.svg?url';
import claudeColor from '@lobehub/icons-static-svg/icons/claude-color.svg?url';
import claudecodeColor from '@lobehub/icons-static-svg/icons/claudecode-color.svg?url';
import codebuddyColor from '@lobehub/icons-static-svg/icons/codebuddy-color.svg?url';
import codexColor from '@lobehub/icons-static-svg/icons/codex-color.svg?url';
import deepseekColor from '@lobehub/icons-static-svg/icons/deepseek-color.svg?url';
import devinColor from '@lobehub/icons-static-svg/icons/devin-color.svg?url';
import geminiColor from '@lobehub/icons-static-svg/icons/gemini-color.svg?url';
import huaweiColor from '@lobehub/icons-static-svg/icons/huawei-color.svg?url';
import junieColor from '@lobehub/icons-static-svg/icons/junie-color.svg?url';
import kiroColor from '@lobehub/icons-static-svg/icons/kiro-color.svg?url';
import langchainColor from '@lobehub/icons-static-svg/icons/langchain-color.svg?url';
import mistralColor from '@lobehub/icons-static-svg/icons/mistral-color.svg?url';
import openclawColor from '@lobehub/icons-static-svg/icons/openclaw-color.svg?url';
import qoderColor from '@lobehub/icons-static-svg/icons/qoder-color.svg?url';
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
import kimiMono from '@lobehub/icons-static-svg/icons/kimi.svg?url';
import nousresearchMono from '@lobehub/icons-static-svg/icons/nousresearch.svg?url';
import opencodeMono from '@lobehub/icons-static-svg/icons/opencode.svg?url';
import openhandsMono from '@lobehub/icons-static-svg/icons/openhands.svg?url';
import qoderMono from '@lobehub/icons-static-svg/icons/qoder.svg?url';
import roocodeMono from '@lobehub/icons-static-svg/icons/roocode.svg?url';
import windsurfMono from '@lobehub/icons-static-svg/icons/windsurf.svg?url';
import zaiMono from '@lobehub/icons-static-svg/icons/zai.svg?url';
// Not in lobehub; from simple-icons (CC0).
import atlassianColor from '../assets/agents/atlassian-color.svg?url';
import kilocodeColor from '../assets/agents/kilocode-color.svg?url';
import lmstudioColor from '../assets/agents/lmstudio-color.svg?url';
import ompColor from '../assets/agents/omp-color.svg?url';
import positColor from '../assets/agents/posit-color.svg?url';
// Vendor logo (MIT repo): https://raw.githubusercontent.com/esengine/DeepSeek-Reasonix/studio/desktop/electron/assets/icon.svg
import reasonixColor from '../assets/agents/reasonix-color.svg?url';
// Vendor logos, used to identify the product. Each has an open-source license covering the file in the vendor's own repo:
// jazz: MIT, https://github.com/lvndry/jazz/blob/HEAD/packages/website/public/favicon.svg
import jazzColor from '../assets/agents/jazz-color.svg?url';
// mcpjam: Apache-2.0 (LICENSE excludes only /server/services), https://github.com/MCPJam/inspector/blob/main/mcpjam-inspector/client/public/mcp_jam.svg
import mcpjamColor from '../assets/agents/mcpjam-color.svg?url';
// pi: https://pi.dev/logo-auto.svg, cropped to the mark; its colors are the brand's in the MIT repo,
// https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/modes/interactive/components/pi-logo.ts
import piColor from '../assets/agents/pi-color.svg?url';
// pochi: Apache-2.0, https://github.com/TabbyML/pochi/blob/main/packages/vscode/assets/icons/pochi-logo.svg
import pochiMono from '../assets/agents/pochi.svg?url';
import coderMono from '../assets/agents/coder.svg?url';
import factoryMono from '../assets/agents/factory.svg?url';
import warpMono from '../assets/agents/warp.svg?url';
import zedMono from '../assets/agents/zed.svg?url';

// Brand marks from @lobehub/icons-static-svg, keyed by target name. Parent-company
// marks stand in where the product has none (cortex, vibe, comate, codearts, …).
const colored: Record<string, string> = {
  amp: ampColor,
  antigravity: antigravityColor,
  'antigravity-cli': antigravityColor,
  claude: claudecodeColor,
  'claude-desktop': claudeColor,
  codearts: huaweiColor,
  codebuddy: codebuddyColor,
  codex: codexColor,
  comate: baiduColor,
  cortex: snowflakeColor,
  deepagents: langchainColor,
  'deepseek-harness': deepseekColor,
  devin: devinColor,
  gemini: geminiColor,
  iflow: alibabaColor,
  jazz: jazzColor,
  junie: junieColor,
  kilocode: kilocodeColor,
  kiro: kiroColor,
  lingma: alibabacloudColor,
  lmstudio: lmstudioColor,
  mcpjam: mcpjamColor,
  omp: ompColor,
  openclaw: openclawColor,
  pi: piColor,
  'posit-assistant': positColor,
  'qoder-cn': qoderColor,
  qwen: qwenColor,
  reasonix: reasonixColor,
  replit: replitColor,
  rovodev: atlassianColor,
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
  droid: factoryMono,
  factory: factoryMono,
  goose: gooseMono,
  grok: grokMono,
  hermes: nousresearchMono,
  kimi: kimiMono,
  'kimi-code': kimiMono,
  // Mux is made by Coder; the parent-company mark stands in.
  mux: coderMono,
  opencode: opencodeMono,
  openhands: openhandsMono,
  pochi: pochiMono,
  qoder: qoderMono,
  roo: roocodeMono,
  vscode: githubcopilotMono,
  warp: warpMono,
  windsurf: windsurfMono,
  zcode: zaiMono,
  zed: zedMono,
};

export default function AgentIcon({ target, size = 16 }: { target: string; size?: number }) {
  // A project's target is `<project>@<tool>`; the logo is the tool's.
  target = target.slice(target.lastIndexOf('@') + 1);
  target = useContext(TargetAgents)[target] ?? target;
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
