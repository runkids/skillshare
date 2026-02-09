import { chromium } from 'playwright';
import { fileURLToPath } from 'url';
import path from 'path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const PORT = process.env.PORT || 3002;

async function main() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({
    viewport: { width: 1280, height: 900 },
    deviceScaleFactor: 3,
  });

  await page.goto(`http://localhost:${PORT}`, { waitUntil: 'networkidle' });
  await page.waitForTimeout(3000);

  // Kill animations, force visibility — keep original backgrounds
  await page.addStyleTag({
    content: `
      *, *::before, *::after {
        animation: none !important;
        transition: none !important;
      }
    `
  });

  await page.evaluate(() => {
    const patterns = ['heroLogoSparkle', 'heroConnector', 'heroUnderline', 'heroTitle', 'heroSubtitle'];
    document.querySelectorAll('*').forEach(el => {
      const cls = el.className;
      if (typeof cls === 'string') {
        for (const p of patterns) {
          if (cls.includes(p)) {
            el.style.opacity = '1';
            el.style.clipPath = 'none';
            break;
          }
        }
      }
    });
  });

  const logoImg = page.locator('img[class*="heroLogo"]');
  await logoImg.waitFor({ state: 'visible', timeout: 10000 });
  const heroWrapper = logoImg.locator('..');

  // Hide title/connector/underline so only logo + ring remain
  await page.evaluate(() => {
    const hidePatterns = ['heroConnector', 'heroTitle', 'heroUnderline', 'heroSubtitle'];
    document.querySelectorAll('*').forEach(el => {
      const cls = el.className;
      if (typeof cls === 'string') {
        for (const p of hidePatterns) {
          if (cls.includes(p)) {
            el.style.display = 'none';
            break;
          }
        }
      }
    });
  });

  const wrapperBox = await heroWrapper.boundingBox();
  // Ring: inset -18px. Shadow: translate(4px,4px) + inset -18px. Sparkles: ~14px outside.
  const expand = 28;
  const logoPath = path.join(__dirname, 'static', 'img', 'hero-logo.png');
  await page.screenshot({
    path: logoPath,
    // Keep original paper-warm background (no omitBackground)
    clip: {
      x: Math.max(0, wrapperBox.x - expand),
      y: Math.max(0, wrapperBox.y - expand),
      width: wrapperBox.width + expand * 2,
      height: wrapperBox.height + expand * 2,
    },
  });
  console.log(`Saved: ${logoPath}`);

  await browser.close();
}

main().catch(console.error);
