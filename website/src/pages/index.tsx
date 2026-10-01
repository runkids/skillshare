import type {ReactNode, RefObject} from 'react';
import {useEffect, useRef, useState} from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import Translate from '@docusaurus/Translate';
import {Copy, Check, Monitor, ShieldCheck} from 'lucide-react';

import {COMMAND_COUNT, FEATURE_GROUPS, TARGET_COUNT} from '../data/featureMap';
import styles from './index.module.css';

// ---------------------------------------------------------------------------
// Board scaling: boards are laid out at a fixed design size and scaled to fit.
// ---------------------------------------------------------------------------

function useBoardScale(ref: RefObject<HTMLDivElement | null>, baseWidth: number): number {
  const [scale, setScale] = useState(1);
  useEffect(() => {
    const el = ref.current;
    if (!el) return undefined;
    const ro = new ResizeObserver(([entry]) => {
      setScale(Math.min(1, entry.contentRect.width / baseWidth));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref, baseWidth]);
  return scale;
}

function ScaledBoard({
  width,
  height,
  className,
  children,
}: {
  width: number;
  height: number;
  className?: string;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const scale = useBoardScale(ref, width);
  return (
    <div ref={ref} className={styles.scaleWrap} style={{height: height * scale}}>
      <div
        className={`${styles.board} ${className ?? ''}`}
        style={{width, height, transform: `scale(${scale})`}}
      >
        {children}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Hero board: source logo strung to 8 of the 67 targets.
// ---------------------------------------------------------------------------

type Tool = {
  id: string;
  name: string;
  path: string;
  local: number;
  left: number;
  top: number;
  rotate: number;
  string: string;
  defaultOn: boolean;
};

const TOOLS: Tool[] = [
  {id: 'claude', name: 'Claude Code', path: '~/.claude/skills', local: 3, left: 110, top: 60, rotate: -1.5, string: 'M584 200 Q384 177 185 74', defaultOn: true},
  {id: 'cursor', name: 'Cursor', path: '~/.cursor/skills', local: 1, left: 390, top: 30, rotate: 1, string: 'M584 200 Q524 162 465 44', defaultOn: true},
  {id: 'codex', name: 'Codex', path: '~/.agents/skills', local: 2, left: 760, top: 30, rotate: -0.8, string: 'M584 200 Q710 162 835 44', defaultOn: true},
  {id: 'gemini', name: 'Gemini CLI', path: '~/.gemini/skills', local: 0, left: 1010, top: 80, rotate: 1.6, string: 'M584 200 Q834 187 1085 94', defaultOn: true},
  {id: 'opencode', name: 'OpenCode', path: '~/.config/opencode/skills', local: 0, left: 90, top: 430, rotate: 1.2, string: 'M584 456 Q374 490 165 444', defaultOn: true},
  {id: 'copilot', name: 'GitHub Copilot', path: '~/.copilot/skills', local: 0, left: 380, top: 530, rotate: -1, string: 'M584 456 Q520 540 455 544', defaultOn: false},
  {id: 'windsurf', name: 'Windsurf', path: '~/.codeium/windsurf/skills', local: 1, left: 740, top: 530, rotate: 0.9, string: 'M584 456 Q700 540 815 544', defaultOn: false},
  {id: 'zed', name: 'Zed', path: '~/.agents/skills', local: 0, left: 1020, top: 440, rotate: -1.4, string: 'M584 456 Q840 495 1095 454', defaultOn: false},
];

const SKILL_COUNT = 14;

function usePinned() {
  const [on, setOn] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(TOOLS.map((t) => [t.id, t.defaultOn])),
  );
  const toggle = (id: string) => setOn((prev) => ({...prev, [id]: !prev[id]}));
  const pinned = TOOLS.filter((t) => on[t.id]);
  return {on, toggle, pinned};
}

function SourceLogo() {
  return (
    <div className={styles.logoWrap}>
      <div className={styles.logoShadow} aria-hidden="true" />
      <div className={styles.logoRing} aria-hidden="true" />
      <span className={styles.sparkle} aria-hidden="true">*</span>
      <span className={styles.sparkle} aria-hidden="true">+</span>
      <span className={styles.sparkle} aria-hidden="true">*</span>
      <img className={styles.logoImg} src="/img/logo.png" alt="skillshare: the source of truth" />
      <span className={`${styles.pin} ${styles.pinBlue}`} style={{left: 102, top: -26}} />
      <span className={`${styles.pin} ${styles.pinBlue}`} style={{left: 102, bottom: -26}} />
    </div>
  );
}

function StringBoard({on, toggle, pinned}: ReturnType<typeof usePinned>) {
  return (
    <ScaledBoard width={1168} height={620} className={styles.heroBoard}>
      <svg width="1168" height="620" viewBox="0 0 1168 620" className={styles.strings} aria-hidden="true">
        {TOOLS.map((t, i) => (
          <path
            key={t.id}
            className={`${styles.str} ${on[t.id] ? styles.strOn : styles.strOff}`}
            d={t.string}
            style={{animationDelay: `${0.05 + i * 0.1}s`}}
          />
        ))}
      </svg>

      <SourceLogo />
      <span className={styles.sourceCaption}>source · ~/.config/skillshare/skills/</span>

      {TOOLS.map((t) => (
        <button
          key={t.id}
          type="button"
          className={`${styles.tool} ${on[t.id] ? '' : styles.toolOff}`}
          style={{left: t.left, top: t.top, transform: `rotate(${t.rotate}deg)`}}
          onClick={() => toggle(t.id)}
          aria-pressed={on[t.id]}
          aria-label={`${on[t.id] ? 'Unpin' : 'Pin'} ${t.name}`}
        >
          <span className={styles.pin} />
          <span className={styles.toolName}>{t.name}</span>
          <span className={styles.toolPath}>{t.path}</span>
        </button>
      ))}

      <span className={styles.note} style={{left: 272, top: 120, width: 130, textAlign: 'center'}}>
        per-skill symlinks<br />(nothing copied)
      </span>
      <span className={styles.note} style={{left: 760, top: 330, width: 190}}>
        tools you don't use<br />just hang there, unlinked
      </span>
      <Link to="/docs/reference/targets/supported-targets" className={styles.boardLink}>
        +{TARGET_COUNT - TOOLS.length} more tools on the full board →
      </Link>

      <div className={styles.counter}>
        <div className={styles.tape} style={{width: 70, height: 20, top: -10, left: 24, transform: 'rotate(-3deg)'}} aria-hidden="true" />
        <span className={styles.counterNum}>{pinned.length}</span>
        <span className={styles.counterLabel}>pinned</span>
      </div>
    </ScaledBoard>
  );
}

// Small screens: same state, no strings.
function PinList({on, toggle}: ReturnType<typeof usePinned>) {
  return (
    <div className={styles.pinList}>
      {TOOLS.map((t) => (
        <button
          key={t.id}
          type="button"
          className={`${styles.pinChip} ${on[t.id] ? styles.pinChipOn : ''}`}
          onClick={() => toggle(t.id)}
          aria-pressed={on[t.id]}
        >
          <span className={styles.pin} />
          {t.name}
        </button>
      ))}
    </div>
  );
}

function SyncTerminal({pinned}: {pinned: Tool[]}) {
  return (
    <div className={styles.termWrap}>
      <div className={styles.tape} style={{top: -14, left: 40, transform: 'rotate(-5deg)'}} aria-hidden="true" />
      <div className={styles.term}>
        <div className={styles.termBar}>
          <span style={{background: '#ff4d4d'}} />
          <span style={{background: '#fff3a0'}} />
          <span style={{background: '#7bd88f'}} />
          <span className={styles.termBarText}>this output follows the board</span>
        </div>
        <pre className={styles.termBody}>
          <span className={styles.ok}>$</span> skillshare sync{'\n\n'}
          <span className={styles.termHead}>Syncing skills</span>{'\n'}
          {pinned.map((t) => (
            <span key={t.id}>
              <span className={styles.ok}>✓</span> {t.id}: merged ({SKILL_COUNT} linked, {t.local} local, 0 updated, 0 pruned){'\n'}
            </span>
          ))}
          {pinned.length === 0 && <span className={styles.dim}>no targets pinned — click a tool on the board{'\n'}</span>}
          {'\n'}
          <span className={styles.dim}>{pinned.length} targets · {SKILL_COUNT} skills</span>
        </pre>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Install
// ---------------------------------------------------------------------------

const DESKTOP_INSTALL = 'brew tap runkids/tap\nbrew install --cask skillshare-app';

function CopyButton({text}: {text: string}) {
  const [copied, setCopied] = useState(false);
  const handleCopy = async () => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };
  return (
    <button className={styles.copyButton} onClick={handleCopy} aria-label="Copy to clipboard">
      {copied ? <Check size={16} /> : <Copy size={16} />}
    </button>
  );
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

function HeroSection() {
  const board = usePinned();
  return (
    <header className={styles.hero}>
      <div className="container">
        <div className={styles.heroHead}>
          <div>
            <Heading as="h1" className={styles.heroTitle}>
              One folder of skills,<br />strung to <span className={styles.mark}>every AI tool.</span>
            </Heading>
            <p className={styles.heroSubtitle}>
              skillshare keeps your skills in one place and symlinks them into Claude Code, Cursor, Codex and{' '}
              {TARGET_COUNT - 3} more. Pin the tools you use. Watch the strings.
            </p>
            <div className={`${styles.buttons} ${styles.heroActions}`}>
              <Link className="button button--primary button--lg" to="/docs/getting-started/desktop-app">
                <Monitor size={18} aria-hidden="true" />{' '}
                <Translate id="home.desktop.get">Get Desktop App</Translate>
              </Link>
              <Link className="button button--secondary button--lg" to="/docs/getting-started/first-sync">
                <Translate id="home.desktop.cliGuide">CLI quick start</Translate>
              </Link>
            </div>
            <p className={styles.desktopPlatforms}>
              <Translate id="home.desktop.platforms">macOS (Apple Silicon) · Windows · Linux</Translate>
            </p>
          </div>
          <span className={`${styles.hand} ${styles.heroHint}`}>click a tool to pin<br />or unpin it ↓</span>
        </div>

        <div className={styles.onlyWide}>
          <StringBoard {...board} />
        </div>
        <div className={styles.onlyNarrow}>
          <PinList {...board} />
        </div>

        <div className={styles.heroBottom}>
          <SyncTerminal pinned={board.pinned} />
          <div className={styles.installCol}>
            <Heading as="h2" className={styles.h2}>
              <Translate id="home.desktop.title">Start with the desktop app.</Translate>
            </Heading>
            <p className={styles.lead}>
              <Translate id="home.desktop.summary">Manage skills, agents, MCP and hooks in one desktop window. First launch helps you set up the CLI, choose your AI tools and run your first sync.</Translate>
            </p>
            <p className={styles.desktopPlatforms}>
              <Translate id="home.desktop.homebrew">Install on macOS with Homebrew:</Translate>
            </p>
            <div className={`${styles.installCommand} ${styles.desktopCommand}`}>
              <code>{DESKTOP_INSTALL}</code>
              <CopyButton text={DESKTOP_INSTALL} />
            </div>
            <div className={styles.buttons}>
              <Link className="button button--primary button--lg" href="https://github.com/runkids/skillshare-app/releases/latest">
                <Translate id="home.desktop.download">Download installers</Translate>
              </Link>
              <Link className="button button--secondary button--lg" to="/docs/getting-started/desktop-app">
                <Translate id="home.desktop.installGuide">Installation guide</Translate>
              </Link>
            </div>
            <p className={styles.cliAlternative}>
              <Link to="/docs/getting-started/first-sync">
                <Translate id="home.desktop.cliAlternative">Prefer the terminal? Install the CLI.</Translate>
              </Link>
            </p>
          </div>
        </div>
      </div>
    </header>
  );
}

const MOVES = [
  {cmd: 'skillshare install', color: 'var(--color-success)', text: 'Repo or local path → audit gate → source. Critical findings block it.'},
  {cmd: 'skillshare sync', color: 'var(--color-blue)', text: 'Source → every target, as per-skill symlinks.'},
  {cmd: 'skillshare collect', color: 'var(--color-danger)', text: 'A skill born inside a tool → back to the source.'},
  {cmd: 'skillshare push · pull', color: 'var(--color-pencil)', text: 'Source ↔ any git remote, so every machine has the same folder.'},
];

function Card({left, top, width, height, rotate, pinLeft, bg, children}: {
  left: number; top: number; width: number; height: number; rotate: number; pinLeft: number; bg?: string; children: ReactNode;
}) {
  return (
    <div className={styles.card} style={{left, top, width, height, transform: `rotate(${rotate}deg)`, background: bg}}>
      <span className={`${styles.pin} ${bg ? styles.pinBlue : ''}`} style={{left: pinLeft, top: -8}} />
      {children}
    </div>
  );
}

function FourMovesSection() {
  return (
    <section className={styles.section}>
      <div className="container">
        <div className={styles.sectionHead}>
          <Heading as="h2" className={styles.h2Big}>Four moves. One board.</Heading>
          <span className={`${styles.hand} ${styles.aside}`}>everything else is a flag</span>
        </div>

        <div className={styles.onlyWide}>
          <ScaledBoard width={1168} height={560}>
            <svg width="1168" height="560" viewBox="0 0 1168 560" className={styles.strings} aria-hidden="true">
              <path className={`${styles.str} ${styles.strOn}`} d="M714 300 Q820 250 930 152" style={{animationDelay: '0.1s'}} />
              <path className={`${styles.str} ${styles.strOn}`} d="M714 300 Q820 300 930 300" style={{animationDelay: '0.2s'}} />
              <path className={`${styles.str} ${styles.strOn}`} d="M714 300 Q820 350 930 448" style={{animationDelay: '0.3s'}} />
              <path d="M930 330 Q810 380 714 330" className={styles.arrowCollect} strokeDasharray="7 6" />
              <path d="M728 318 L714 330 L730 340" className={styles.arrowCollect} />
              <path d="M570 220 Q560 140 560 92" className={styles.arrowGit} />
              <path d="M552 108 L560 90 L570 108" className={styles.arrowGit} />
              <path d="M604 92 Q604 140 604 220" className={styles.arrowGit} strokeDasharray="7 6" />
              <path d="M594 204 L604 222 L614 204" className={styles.arrowGit} />
              <path d="M200 300 Q250 300 296 300" className={styles.arrowInstall} />
              <path d="M392 300 Q420 300 454 300" className={styles.arrowInstall} />
              <path d="M440 288 L456 300 L440 312" className={styles.arrowInstall} />
            </svg>

            <Card left={476} top={28} width={216} height={62} rotate={0.8} pinLeft={100}>
              <span className={styles.cardTitle}>Any git remote</span>
              <span className={styles.cardSub}>GitHub · GitLab · self-hosted</span>
            </Card>
            <span className={styles.note} style={{left: 630, top: 130}}>
              push ↑ pull ↓<br /><span className={styles.noteCmd}>skillshare push · pull</span>
            </span>

            <Card left={40} top={268} width={160} height={64} rotate={-1.2} pinLeft={72}>
              <span className={styles.cardTitle}>owner/repo</span>
              <span className={styles.cardSub}>or a local path</span>
            </Card>
            <div className={styles.stamp}>
              <ShieldCheck size={26} strokeWidth={2} />
              <span>AUDITED</span>
            </div>
            <span className={styles.note} style={{left: 236, top: 352, width: 220}}>
              every install is scanned first.<br />critical findings block it.<br />
              <span className={styles.noteCmd}>skillshare install · audit</span>
            </span>

            <Card left={454} top={220} width={260} height={160} rotate={-0.6} pinLeft={122} bg="var(--color-postit)">
              <span className={styles.cardKicker}>Source</span>
              <span className={styles.cardCode}>~/.config/skillshare/skills/</span>
              <span className={styles.cardText}>One folder. Yours, your team's tracked repos, and project skills, side by side.</span>
            </Card>

            <Card left={930} top={120} width={190} height={60} rotate={1} pinLeft={86}>
              <span className={styles.cardTitle}>Claude Code</span>
              <span className={styles.cardSub}>pr-review → source</span>
            </Card>
            <Card left={930} top={270} width={190} height={60} rotate={-0.8} pinLeft={86}>
              <span className={styles.cardTitle}>Cursor</span>
              <span className={styles.cardSub}>+ 1 local skill kept</span>
            </Card>
            <Card left={930} top={418} width={190} height={60} rotate={1.3} pinLeft={86}>
              <span className={styles.cardTitle}>Codex</span>
              <span className={styles.cardSub}>… {TARGET_COUNT - 3} more</span>
            </Card>
            <span className={styles.note} style={{left: 790, top: 196, color: 'var(--color-blue)'}}>
              sync →<br /><span className={styles.noteCmd}>skillshare sync</span>
            </span>
            <span className={styles.note} style={{left: 760, top: 372, color: 'var(--color-danger)'}}>
              ← collect<br /><span className={styles.noteCmd}>skillshare collect cursor</span>
            </span>
            <span className={styles.note} style={{left: 40, top: 470, width: 300}}>
              merge mode never overwrites a skill the tool already had. that's the whole trick.
            </span>
          </ScaledBoard>
        </div>

        <ul className={`${styles.onlyNarrow} ${styles.moveList}`}>
          {MOVES.map((m) => (
            <li key={m.cmd} className={styles.moveItem}>
              <code className={styles.cmd} style={{borderColor: m.color}}>{m.cmd}</code>
              <span>{m.text}</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

const IDX_POSITIONS = [
  {left: 40, top: 40, rotate: -2},
  {left: 250, top: 70, rotate: 1.5},
  {left: 460, top: 36, rotate: -1},
  {left: 90, top: 220, rotate: 1.2},
  {left: 300, top: 250, rotate: -1.6},
  {left: 510, top: 226, rotate: 0.8},
];

function FeatureMapTeaser() {
  return (
    <section className={styles.section}>
      <div className="container">
        <div className={styles.teaser}>
          <div className={styles.teaserText}>
            <span className={`${styles.hand} ${styles.bigNum}`}>{TARGET_COUNT}</span>
            <span className={styles.teaserTitle}>tools on the board.<br />{COMMAND_COUNT} commands to move them.</span>
            <p className={styles.lead}>
              Nobody remembers {COMMAND_COUNT} commands. The map pins them by the job you're doing, with a live filter.
            </p>
            <Link className="button button--primary button--lg" to="/features">
              Open the feature map
            </Link>
          </div>
          <Link to="/features" className={`${styles.board} ${styles.teaserBoard}`}>
            {FEATURE_GROUPS.map((g, i) => (
              <span
                key={g.id}
                className={styles.idx}
                style={{left: IDX_POSITIONS[i].left, top: IDX_POSITIONS[i].top, transform: `rotate(${IDX_POSITIONS[i].rotate}deg)`}}
              >
                <span className={styles.pin} style={{left: 66, top: -8}} />
                <span className={styles.idxTitle}>{g.title}</span>
                <span className={styles.idxCmds}>{g.teaser}</span>
              </span>
            ))}
            <span className={`${styles.note} ${styles.teaserNote}`}>open the full board →</span>
          </Link>
        </div>
      </div>
    </section>
  );
}

function CtaSection() {
  return (
    <section className={styles.ctaSection}>
      <div className="container">
        <div className={styles.cta}>
          <div className={styles.tape} style={{top: -14, left: 60, transform: 'rotate(-4deg)'}} aria-hidden="true" />
          <div>
            <Heading as="h2" className={`${styles.hand} ${styles.ctaTitle}`}>Pin it once. Never copy a skill again.</Heading>
            <p className={styles.lead}>Free, MIT licensed. Uninstall leaves every tool exactly as it was.</p>
          </div>
          <div className={styles.buttons}>
            <Link className="button button--primary button--lg" to="/docs/getting-started/desktop-app">
              <Translate id="home.desktop.get">Get Desktop App</Translate>
            </Link>
            <Link className="button button--secondary button--lg" href="https://github.com/runkids/skillshare">
              Star on GitHub
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout title="AI Skills Manager — Desktop App & CLI" description={siteConfig.tagline}>
      <div className={styles.homePage}>
        <HeroSection />
        <main>
          <FourMovesSection />
          <FeatureMapTeaser />
          <CtaSection />
        </main>
      </div>
    </Layout>
  );
}
