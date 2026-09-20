// Test the delivered files and interactions without a development server or network.
import { readFile, writeFile, stat, mkdir } from 'node:fs/promises';
import { dirname, resolve, join } from 'node:path';
import { pathToFileURL, fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { chromium } from 'playwright';
import sharp from 'sharp';
import { guides } from './guides.mjs';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const temp = join(root, '.build');
await mkdir(temp, { recursive: true });
const channel = process.env.VISUAL_BROWSER_CHANNEL || (process.platform === 'win32' ? 'msedge' : 'bundled');
const browser = await chromium.launch({ headless: true, ...(channel === 'bundled' ? {} : { channel }) });
const results = [];
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1100 } });
  const external = [];
  await context.route(/^https?:/, route => { external.push(route.request().url()); return route.abort(); });
  for (const slug of ['index', ...guides.map(g => g.slug)]) {
    const html = await readFile(join(root, `${slug}.html`), 'utf8');
    for (const match of html.matchAll(/(?:href|src|data-src)="([^"]+)"/g)) {
      const link = match[1];
      if (link.startsWith('#') || /^(data:|https?:)/.test(link)) continue;
      await stat(resolve(root, decodeURIComponent(link.split('#')[0])));
    }
    const page = await context.newPage(); const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(pathToFileURL(join(root, `${slug}.html`)).href);
    await page.evaluate(() => document.fonts.ready);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${slug}: desktop overflow`);
    if (slug !== 'index') {
      const shapeCheck = await page.evaluate(() => {
        const outer = document.querySelector('#vector>svg');
        const graph = outer.querySelector('svg');
        const box = graph.viewBox.baseVal;
        const violations = [];
        for (const node of graph.querySelectorAll('g.node')) {
          const bounds = node.getBBox(); const matrix = node.getScreenCTM(); const base = graph.getScreenCTM().inverse();
          for (const [x,y] of [[bounds.x,bounds.y],[bounds.x+bounds.width,bounds.y+bounds.height]]) {
            const pt = new DOMPoint(x,y).matrixTransform(matrix).matrixTransform(base);
            if (pt.x < box.x-2 || pt.y < box.y-2 || pt.x > box.x+box.width+2 || pt.y > box.y+box.height+2) violations.push(node.id);
          }
        }
        return violations;
      });
      assert.deepEqual(shapeCheck, [], `${slug}: clipped nodes`);
      await page.getByRole('button', { name: 'Pause', exact: true }).click();
      const paused = await page.locator('#vector>svg').evaluate(svg => svg.getCurrentTime());
      await page.waitForTimeout(180);
      assert.equal(await page.locator('#vector>svg').evaluate(svg => svg.getCurrentTime()), paused, `${slug}: pause failed`);
      await page.getByRole('button', { name: 'GIF', exact: true }).click();
      await page.locator('#gif img').evaluate(img => img.decode());
      assert.equal(await page.locator('#gif').isVisible(), true);
      await page.getByRole('button', { name: 'Pause', exact: true }).click();
      assert.equal(await page.locator('#still').isVisible(), true, `${slug}: GIF pause fallback`);
      await page.getByRole('button', { name: 'Still', exact: true }).click();
      assert.equal(await page.locator('#play-toggle').isDisabled(), true);
      await page.getByRole('button', { name: 'Motion SVG', exact: true }).click();
      await page.locator('#speed').selectOption('1.5');
      await page.getByRole('button', { name: 'Expand', exact: true }).click();
      assert.equal(await page.locator('body').evaluate(body => body.classList.contains('theater')), true);
      await page.getByRole('button', { name: 'Compact', exact: true }).click();
      await page.getByRole('button', { name: 'Still', exact: true }).click();
      const imagePath = join(root, 'assets', `${slug}.gif`);
      const metadata = await sharp(imagePath, { animated: true }).metadata();
      assert.equal(metadata.pages, 80); assert.equal(metadata.width, 864); assert.equal(metadata.pageHeight, 1080);
      assert.equal(metadata.loop, 0); assert.equal(metadata.delay.reduce((a,b)=>a+b,0), 8000);
      const first = await sharp(imagePath, { page: 0 }).raw().toBuffer();
      const later = await sharp(imagePath, { page: 20 }).raw().toBuffer();
      assert.notEqual(createHash('sha256').update(first).digest('hex'), createHash('sha256').update(later).digest('hex'), `${slug}: GIF has no motion`);
      results.push({ slug, frames: metadata.pages, width: metadata.width, height: metadata.pageHeight, durationMs: 8000, bytes: (await stat(imagePath)).size });
    }
    await page.screenshot({ path: join(temp, `${slug}-desktop.png`), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${slug}: mobile overflow`);
    await page.screenshot({ path: join(temp, `${slug}-mobile.png`), fullPage: true });
    assert.deepEqual(errors, [], `${slug}: browser errors`);
    await page.close();
    console.log(`PASS ${slug}: local links, offline load, layout${slug === 'index' ? '' : ', playback, GIF motion'}`);
  }
  const reduced = await browser.newPage({ reducedMotion: 'reduce' });
  await reduced.goto(pathToFileURL(join(root, 'architecture.html')).href);
  assert.equal(await reduced.locator('#still').isVisible(), true, 'Reduced-motion default');
  assert.equal(await reduced.locator('#gif img').getAttribute('src'), null, 'Reduced-motion page should not load GIF');
  assert.deepEqual(external, [], 'Pages attempted external requests');
  await writeFile(join(temp, 'validation.json'), JSON.stringify({ checkedAt: new Date().toISOString(), pages: 6, offline: true, reducedMotion: true, results }, null, 2));
  console.log('PASS reduced-motion defaults and zero external requests');
} finally { await browser.close(); }
