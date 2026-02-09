import type {ReactNode} from 'react';
import {useState} from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import {Copy, Check, Apple, Terminal, GitBranch, FolderSync, ArrowLeftRight, Globe, Layers, LayoutDashboard} from 'lucide-react';

import styles from './index.module.css';

type InstallMethod = 'curl' | 'powershell' | 'homebrew';

const INSTALL_COMMANDS: Record<InstallMethod, {command: string; label: string; icon: ReactNode}> = {
  curl: {
    command: 'curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh',
    label: 'macOS / Linux',
    icon: <Terminal size={14} />,
  },
  powershell: {
    command: 'irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex',
    label: 'Windows',
    icon: <span style={{fontSize: '12px'}}>PS</span>,
  },
  homebrew: {
    command: 'brew install runkids/tap/skillshare',
    label: 'Homebrew',
    icon: <Apple size={14} />,
  },
};

// Slight random rotations for hand-drawn card feel
const CARD_ROTATIONS = [
  'rotate(-0.8deg)',
  'rotate(0.5deg)',
  'rotate(-0.3deg)',
  'rotate(0.7deg)',
  'rotate(-0.5deg)',
  'rotate(0.4deg)',
];

function WavyDivider({color = 'var(--color-muted-dark)'}: {color?: string}) {
  return (
    <div className={styles.wavyDivider} aria-hidden="true">
      <svg viewBox="0 0 1200 40" preserveAspectRatio="none">
        <path
          d="M0,20 Q150,0 300,20 T600,20 T900,20 T1200,20"
          fill="none"
          stroke={color}
          strokeWidth="2"
          strokeDasharray="8,6"
        />
      </svg>
    </div>
  );
}

function CopyButton({text}: {text: string}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button
      className={styles.copyButton}
      onClick={handleCopy}
      aria-label="Copy to clipboard"
    >
      {copied ? <Check size={16} /> : <Copy size={16} />}
    </button>
  );
}

function InstallTabs() {
  const [method, setMethod] = useState<InstallMethod>('curl');
  const current = INSTALL_COMMANDS[method];

  return (
    <div className={styles.installSection}>
      <div className={styles.installTabs}>
        {(Object.keys(INSTALL_COMMANDS) as InstallMethod[]).map((key) => (
          <button
            key={key}
            className={`${styles.installTab} ${method === key ? styles.installTabActive : ''}`}
            onClick={() => setMethod(key)}
          >
            {INSTALL_COMMANDS[key].icon}
            <span>{INSTALL_COMMANDS[key].label}</span>
          </button>
        ))}
      </div>
      <div className={styles.installCommand}>
        <code>
          <span className={styles.prompt}>$</span> {current.command}
        </code>
        <CopyButton text={current.command} />
      </div>
    </div>
  );
}

function HeroSection() {
  return (
    <header className={styles.hero}>
      <div className="container">
        {/* Logo + title group: acts as a cohesive visual unit */}
        <div className={styles.heroGroup}>
          <div className={styles.heroLogoWrapper}>
            <div className={styles.heroLogoRing} aria-hidden="true" />
            <div className={styles.heroLogoShadow} aria-hidden="true" />
            <span className={styles.heroLogoSparkle} aria-hidden="true">*</span>
            <span className={styles.heroLogoSparkle} aria-hidden="true">+</span>
            <span className={styles.heroLogoSparkle} aria-hidden="true">*</span>
            <img
              src="/img/logo.png"
              alt="skillshare"
              className={styles.heroLogo}
            />
          </div>

          {/* Hand-drawn connector line between logo and title */}
          <svg
            className={styles.heroConnector}
            viewBox="0 0 120 24"
            aria-hidden="true"
          >
            <path
              d="M60,2 C58,8 54,12 52,16 C50,20 56,20 60,22 C64,20 70,20 68,16 C66,12 62,8 60,2"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeDasharray="4,3"
              strokeLinecap="round"
            />
          </svg>

          <Heading as="h1" className={styles.heroTitle}>
            One source of truth for{' '}
            <span className={styles.heroTitleAccent}>AI CLI skills</span>
          </Heading>

          {/* Hand-drawn underline beneath the title */}
          <svg
            className={styles.heroUnderline}
            viewBox="0 0 400 12"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            <path
              d="M8,8 C50,3 100,6 150,5 C200,4 250,7 300,5 C350,3 380,6 392,7"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
            />
          </svg>
        </div>

        <p className={styles.heroSubtitle}>
          Sync everywhere with one command. Claude Code, OpenCode, Cursor & 40+ more.
        </p>

        <InstallTabs />

        <div className={styles.heroButtons}>
          <Link className="button button--primary button--lg" to="/docs/">
            Get Started
          </Link>
          <Link
            className="button button--secondary button--lg"
            href="https://github.com/runkids/skillshare"
          >
            View on GitHub
          </Link>
        </div>
      </div>
    </header>
  );
}

const whyFeatures = [
  {
    Icon: FolderSync,
    title: 'Non-destructive Merge',
    description: 'Sync shared skills while preserving CLI-specific ones. Per-skill symlinks keep local skills untouched.',
  },
  {
    Icon: ArrowLeftRight,
    title: 'Bidirectional Sync',
    description: 'Created a skill in Claude? Collect it back to source and share with OpenClaw, OpenCode, and others.',
  },
  {
    Icon: Globe,
    title: 'Cross-machine Sync',
    description: 'One git push/pull syncs skills across all your machines. No re-running install commands.',
  },
  {
    Icon: Layers,
    title: 'Unified Source',
    description: 'Local skills and installed skills live together in one directory. No separate management.',
  },
  {
    Icon: GitBranch,
    title: 'Dual-Level Skills',
    description: 'Organization-wide standards via tracked repos, plus project-scoped skills shared via git. Both auto-detected.',
  },
  {
    Icon: LayoutDashboard,
    title: 'Web Dashboard',
    description: 'Visual skill browsing, sync status, and management. Run skillshare ui — single binary, no setup.',
  },
];

const uiHighlights = [
  {
    title: 'One-Click Install',
    description: 'Install a selected skill directly from the web flow without switching to terminal.',
    image: '/img/web-install-demo.png',
    alt: 'Web install flow for a selected skill',
  },
  {
    title: 'Dashboard Overview',
    description: 'Quickly check total skills, targets, and current sync mode.',
    image: '/img/web-dashboard-demo.png',
    alt: 'Web dashboard overview page',
  },
  {
    title: 'Skills Explorer',
    description: 'Browse installed skills and search your local catalog instantly.',
    image: '/img/web-skills-demo.png',
    alt: 'Web skills page listing installed skills',
  },
  {
    title: 'Skill Detail',
    description: 'Open SKILL.md content with metadata and repository source in one view.',
    image: '/img/web-skill-detail-demo.png',
    alt: 'Web skill detail page showing SKILL.md content',
  },
  {
    title: 'Sync Control',
    description: 'Run sync with dry-run and force options from a single control panel.',
    image: '/img/web-sync-demo.png',
    alt: 'Web sync page with dry-run and force controls',
  },
  {
    title: 'GitHub Search',
    description: 'Search remote skills and install to your source directory in one flow.',
    image: '/img/web-search-skills-demo.png',
    alt: 'Web search page for GitHub skills',
  },
];

function WhySection() {
  return (
    <section className={styles.why}>
      <div className="container">
        <Heading as="h2" className={styles.sectionTitle}>
          Why skillshare?
        </Heading>
        <p className={styles.sectionSubtitle}>
          Install tools get skills onto agents. <strong>Skillshare keeps them in sync.</strong>
        </p>
        <div className={styles.whyGrid}>
          {whyFeatures.map((item, idx) => (
            <div
              key={idx}
              className={styles.whyCard}
              style={{transform: CARD_ROTATIONS[idx]}}
            >
              <div className={styles.whyIconWrapper}>
                <item.Icon size={22} strokeWidth={1.5} />
              </div>
              <Heading as="h3" className={styles.whyCardTitle}>
                {item.title}
              </Heading>
              <p className={styles.whyCardDescription}>{item.description}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function DemoSection() {
  return (
    <section className={styles.demo}>
      <div className="container">
        <div className={styles.demoContainer}>
          <div className={styles.tapeDecoration} aria-hidden="true" />
          <img src="/img/demo.gif" alt="skillshare demo" />
        </div>
      </div>
    </section>
  );
}

function UIHighlightsSection() {
  return (
    <section className={styles.uiHighlights}>
      <div className="container">
        <Heading as="h2" className={styles.sectionTitle}>
          Web UI Highlights
        </Heading>
        <p className={styles.sectionSubtitle}>
          v0.10.0 delivers a complete visual workflow across dashboard, skills, install, targets, sync, collect, backup, git sync, and search.
        </p>
        <div className={styles.uiHighlightsGrid}>
          {uiHighlights.map((item) => (
            <article key={item.title} className={styles.uiHighlightCard}>
              <img src={item.image} alt={item.alt} loading="lazy" />
              <div className={styles.uiHighlightContent}>
                <Heading as="h3" className={styles.uiHighlightTitle}>
                  {item.title}
                </Heading>
                <p className={styles.uiHighlightDescription}>{item.description}</p>
              </div>
            </article>
          ))}
        </div>
        <div className={styles.uiHighlightsAction}>
          <Link className="button button--primary button--lg" to="/docs/commands/ui">
            Explore Web UI Docs
          </Link>
        </div>
      </div>
    </section>
  );
}

const supportedCLIs = [
  'Claude Code',
  'OpenCode',
  'Cursor',
  'Gemini CLI',
  'Codex',
  '40+ more',
];

function SupportedSection() {
  return (
    <section className={styles.supported}>
      <div className="container">
        <p className={styles.supportedTitle}>Works with</p>
        <div className={styles.cliLogos}>
          {supportedCLIs.map((cli, idx) => (
            <span key={idx}>{cli}</span>
          ))}
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="AI CLI Skills Sync Tool"
      description={siteConfig.tagline}
    >
      <HeroSection />
      <WavyDivider />
      <main>
        <WhySection />
        <WavyDivider />
        <DemoSection />
        <WavyDivider />
        <UIHighlightsSection />
        <WavyDivider />
        <SupportedSection />
      </main>
    </Layout>
  );
}
