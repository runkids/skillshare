import type {ReactNode} from 'react';
import {useState} from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import {RefreshCw, Users, Bot, Copy, Check} from 'lucide-react';

import styles from './index.module.css';

const INSTALL_COMMAND = 'curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh';

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

function HeroSection() {
  return (
    <header className={styles.hero}>
      <div className="container">
        <img
          src="/img/logo.png"
          alt="skillshare"
          className={styles.heroLogo}
        />
        <Heading as="h1" className={styles.heroTitle}>
          One source of truth for AI CLI skills
        </Heading>
        <p className={styles.heroSubtitle}>
          Sync everywhere with one command. Claude Code, OpenCode, Cursor & 40+ more.
        </p>

        <div className={styles.installCommand}>
          <code>
            <span className={styles.prompt}>$</span> {INSTALL_COMMAND}
          </code>
          <CopyButton text={INSTALL_COMMAND} />
        </div>

        <div className={styles.heroButtons}>
          <Link className="button button--primary button--lg" to="/docs/intro">
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

const features = [
  {
    title: 'Sync Everywhere',
    Icon: RefreshCw,
    description: 'Edit once, sync to all 40+ AI CLIs. Symlinks keep everything in sync automatically.',
  },
  {
    title: 'Team Edition',
    Icon: Users,
    description: 'Share skills across your team with tracked repos. Install once, update everywhere.',
  },
  {
    title: 'AI-Native',
    Icon: Bot,
    description: 'Built-in skill teaches AI how to manage skills. Your AI can run skillshare directly.',
  },
];

function FeaturesSection() {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className={styles.featureGrid}>
          {features.map((feature, idx) => (
            <div key={idx} className={styles.featureCard}>
              <div className={styles.featureIconWrapper}>
                <feature.Icon size={28} strokeWidth={1.5} />
              </div>
              <Heading as="h3" className={styles.featureTitle}>
                {feature.title}
              </Heading>
              <p className={styles.featureDescription}>{feature.description}</p>
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
          <img src="/img/demo.gif" alt="skillshare demo" />
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
      <main>
        <FeaturesSection />
        <DemoSection />
        <SupportedSection />
      </main>
    </Layout>
  );
}
