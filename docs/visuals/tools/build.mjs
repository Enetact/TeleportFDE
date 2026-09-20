import { readFile, writeFile, mkdir, stat } from 'node:fs/promises';
import { createServer } from 'node:http';
import { dirname, resolve, join, extname, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
import sharp from 'sharp';
import gifenc from 'gifenc';
import { guides } from './guides.mjs';

const { GIFEncoder, quantize, applyPalette } = gifenc;
const toolsRoot = dirname(fileURLToPath(import.meta.url));
const root = resolve(toolsRoot, '..');
const repoRoot = resolve(root, '../..');
const output = join(root, 'assets');
const temporary = join(root, '.build');
const htmlOnly = process.argv.includes('--html-only');
const selected = process.argv.find(arg => arg.startsWith('--only='))?.split('=')[1];
const esc = value => String(value).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');
await mkdir(output, { recursive: true });
await mkdir(temporary, { recursive: true });
for (const guide of guides) for (const ref of guide.refs) await stat(join(repoRoot, ref));

const head = (title, description) => `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="description" content="${esc(description)}"><title>${esc(title)} · Replica Control</title><link rel="icon" href="data:,"><link rel="stylesheet" href="assets/guide.css"></head><body>`;
const header = `<header><div class="bar"><a class="brand" href="index.html"><span class="mark" aria-hidden="true">↗</span><span>Replica Control<small>THE VISUAL FIELD GUIDE</small></span></a><span class="header-note">Development reference · works offline</span></div></header>`;
const footer = `<footer class="footer"><span>TeleportFDE · independent educational reference</span><span>Illustrated architecture, not live telemetry · <a href="README.md">Guide & rebuild instructions</a></span></footer>`;

function poster(guide, diagram) {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="1080" height="1350" viewBox="0 0 1080 1350" role="img" aria-labelledby="poster-title poster-description">
  <title id="poster-title">${esc(guide.title)}</title><desc id="poster-description">${esc(guide.summary)} Illustrative flow, not live telemetry.</desc>
  <defs><radialGradient id="ambient" cx="90%" cy="0%" r="90%"><stop stop-color="#183849"/><stop offset=".7" stop-color="#0b1423"/></radialGradient><pattern id="dots" width="28" height="28" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r=".8" fill="#7aa4b5" opacity=".12"/></pattern></defs>
  <rect width="1080" height="1350" fill="#0b1423"/><rect width="1080" height="1100" fill="url(#ambient)"/>
  <rect width="1080" height="1350" fill="url(#dots)"/>
  <g font-family="Segoe UI,Arial,sans-serif">
  <rect x="48" y="45" width="28" height="4" rx="2" fill="#7ce8cc"/>
  <text x="91" y="52" fill="#9bc5c1" font-size="12" letter-spacing="2.4">REPLICA CONTROL / VISUAL FIELD GUIDE</text>
  <text x="1028" y="58" text-anchor="end" font-size="24" fill="#7ce8cc" font-family="Consolas,monospace">${guide.number}</text>
  <text x="48" y="101" fill="#80e3c7" font-size="12" letter-spacing="2.5">${esc(guide.category)}</text>
  <text x="46" y="163" fill="#f3f9f7" font-size="53" font-weight="650" letter-spacing="-2">${esc(guide.title)}</text>
  <text x="48" y="203" fill="#a8beca" font-size="22">${esc(guide.subtitle)}</text>
  <rect x="40" y="232" width="1000" height="836" rx="24" fill="#0f1d30" stroke="#294254"/>
  <text x="67" y="267" fill="#819bac" font-size="10" letter-spacing="2">${guide.slug === 'levels' ? 'CAPABILITIES ACCUMULATE' : 'ILLUSTRATED FLOW'}</text>
  <circle cx="983" cy="262" r="4" fill="#79e2c7"/><text x="966" y="266" text-anchor="end" fill="#8baaaf" font-size="10" letter-spacing="1">${guide.slug === 'levels' ? 'L1 → L5' : 'DIRECTION OF WORK'}</text>
  </g>
  ${diagram}
  <g font-family="Segoe UI,Arial,sans-serif">
  <text x="48" y="1114" fill="#e0f7ee" font-size="23" font-weight="600">${esc(guide.principle)}</text>
  ${guide.facts.map(([label, value], i) => `<rect x="${48 + i * 332}" y="1140" width="320" height="106" rx="13" fill="#13253a" stroke="#2b4354"/><text x="${68 + i * 332}" y="1174" fill="#8ca9b9" font-size="14">${esc(label)}</text><text x="${68 + i * 332}" y="1213" fill="#e6f6ee" font-size="20" font-weight="600">${esc(value)}</text>`).join('')}
  <text x="48" y="1283" fill="#9eb5bf" font-size="15">${esc(guide.caption)}</text>
  <path d="M48 1304 H1032" stroke="#243b4d"/><text x="48" y="1330" fill="#6d8c9c" font-size="11" letter-spacing="1">TELEPORTFDE · DEVELOPMENT REFERENCE</text><text x="1032" y="1330" text-anchor="end" fill="#6d8c9c" font-size="11">MERMAID → SVG → GIF</text>
  </g></svg>`;
}

function guidePage(guide, animated, source) {
  const next = guides[(guides.indexOf(guide) + 1) % guides.length];
  return `${head(guide.title, guide.summary)}${header}<main>
  <div class="breadcrumb"><a href="index.html">Visual field guide</a> / ${guide.number} / ${esc(guide.category.toLowerCase())}</div>
  <section class="guide-heading"><p class="eyebrow">Guide ${guide.number} · ${esc(guide.category)}</p><h1>${esc(guide.title)}</h1><p class="lede">${esc(guide.summary)}</p></section>
  <nav class="guide-nav" aria-label="Visual guides">${guides.map(g => `<a href="${g.slug}.html" ${g.slug === guide.slug ? 'aria-current="page"' : ''}>${g.number} ${esc(g.category.split(' ')[0].toLowerCase())}</a>`).join('')}</nav>
  <div class="guide-layout"><section aria-label="Diagram and downloads">
  <div class="player"><div class="player-toolbar"><div class="mode-buttons" role="group" aria-label="Display format"><button data-mode="vector" aria-pressed="true">Motion SVG</button><button data-mode="gif" aria-pressed="false">GIF</button><button data-mode="still" aria-pressed="false">Still</button></div><div class="playback-controls"><button id="play-toggle">Pause</button><button id="restart" aria-label="Restart animation">↺</button><select id="speed" aria-label="SVG playback speed"><option value="0.5">0.5×</option><option value="1" selected>1×</option><option value="1.5">1.5×</option></select><button id="theater" aria-pressed="false">Expand</button></div></div>
  <div class="stage"><div id="vector">${animated}</div><div id="gif" hidden><img data-src="assets/${guide.slug}.gif" alt="Animated ${esc(guide.subtitle.toLowerCase())}" width="864" height="1080"></div><div id="still" hidden><img src="assets/${guide.slug}.svg" alt="${esc(guide.summary)}" width="1080" height="1350"></div></div>
  <div class="progress-track" aria-hidden="true"><span id="progress"></span></div><div class="player-status"><span id="play-status" role="status">SVG motion · playing</span><span>Illustration · not live telemetry</span></div></div>
  <div class="downloads" aria-label="Download diagram assets"><a href="assets/${guide.slug}.gif" download>↓ Animated GIF</a><a href="assets/${guide.slug}-animated.svg" download>↓ Motion SVG</a><a href="assets/${guide.slug}.svg" download>↓ Static SVG</a><a href="assets/${guide.slug}-mermaid.svg" download>↓ Mermaid SVG</a><a href="sources/${guide.slug}.mmd" download>↓ Mermaid source</a></div>
  <details><summary>Read the editable Mermaid source</summary><pre>${esc(source)}</pre></details>
  </section><aside aria-label="Architecture walkthrough">
  ${guide.notes.map(([title, body], i) => `<article class="explain-card"><span class="step-number">${String(i + 1).padStart(2, '0')} / UNDERSTAND THE FLOW</span><h3>${esc(title)}</h3><p>${esc(body)}</p></article>`).join('')}
  <p class="caveat"><strong>What this diagram does not prove</strong>${esc(guide.caveat)}</p>
  <section class="sources"><h3>Follow the implementation</h3>${guide.refs.map(ref => `<a href="../../${ref}">${esc(ref)}</a>`).join('')}</section>
  </aside></div><div class="next-guide"><a href="index.html">← All five guides</a><a href="${next.slug}.html">Next: ${esc(next.title)} →</a></div></main>${footer}<script src="assets/guide.js"></script></body></html>`;
}

async function generatePages() {
  for (const guide of guides) {
    const source = await readFile(join(root, 'sources', `${guide.slug}.mmd`), 'utf8');
    const svg = await readFile(join(output, `${guide.slug}-animated.svg`), 'utf8');
    await writeFile(join(root, `${guide.slug}.html`), guidePage(guide, svg, source));
  }
  const index = `${head('The visual field guide', 'Five animated architecture guides for the TeleportFDE development reference.')}${header}<main>
  <section class="hero"><div><p class="eyebrow">Architecture you can follow</p><h1>See the system.<br>Follow the flow.</h1><p class="lede">A visual field guide to Replica Control. Explore the architecture, trace the controller, and understand how five challenge levels fit together.</p><div class="tags"><span class="tag">Editable Mermaid</span><span class="tag">Animated SVG + GIF</span><span class="tag">100% local viewing</span></div></div><div class="hero-aside"><div class="hero-stat"><b>05</b><span>guided stories<br>from request to reconciliation</span></div></div></section>
  <div class="section-head"><h2>Choose a story</h2><p>Open a guide to play, pause, explore and download.</p></div>
  <section class="gallery" aria-label="Architecture guides">${guides.map(g => `<a class="card" href="${g.slug}.html"><div class="card-art"><img src="assets/${g.slug}.svg" alt="" loading="lazy" width="1080" height="1350"><span class="card-number">${g.number} / 05</span></div><div class="card-body"><p class="eyebrow">${esc(g.category)}</p><h3>${esc(g.title)}</h3><p>${esc(g.summary)}</p><span class="card-link">Explore this guide <span aria-hidden="true">↗</span></span></div></a>`).join('')}</section>
  <div class="note-strip"><strong>Built for understanding.</strong><p>The motion illustrates code paths and design intent. It does not show live traffic or prove availability. Each guide links to the shared implementation and explains its limits.</p></div>
  </main>${footer}</body></html>`;
  await writeFile(join(root, 'index.html'), index);
}

let browser, server;
if (!htmlOnly) {
  server = createServer(async (request, response) => {
    try {
      const url = new URL(request.url, 'http://localhost');
      if (url.pathname === '/__render') { response.setHeader('Content-Type', 'text/html'); response.end('<!doctype html><html><head><link rel="icon" href="data:,"></head><body></body></html>'); return; }
      const path = resolve(root, `.${decodeURIComponent(url.pathname)}`);
      if (!path.startsWith(root + sep)) { response.writeHead(403).end(); return; }
      const mime = { '.mjs': 'text/javascript', '.js': 'text/javascript', '.svg': 'image/svg+xml', '.html': 'text/html', '.css': 'text/css' };
      response.setHeader('Content-Type', mime[extname(path)] || 'application/octet-stream');
      response.end(await readFile(path));
    } catch { response.writeHead(404).end(); }
  });
  await new Promise(done => server.listen(0, '127.0.0.1', done));
  const base = `http://127.0.0.1:${server.address().port}`;
  const channel = process.env.VISUAL_BROWSER_CHANNEL || (process.platform === 'win32' ? 'msedge' : 'bundled');
  try {
    browser = await chromium.launch({ headless: true, ...(channel === 'bundled' ? {} : { channel }) });
    const page = await browser.newPage({ viewport: { width: 1080, height: 1350 }, deviceScaleFactor: 1 });
    await page.goto(`${base}/__render`);
    await page.evaluate(async () => {
      const { default: mermaid } = await import('/tools/node_modules/mermaid/dist/mermaid.esm.min.mjs');
      window.mermaid = mermaid;
      mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'base', layout: 'dagre',
        themeVariables: { fontFamily: 'Segoe UI,Arial,sans-serif', fontSize: '23px', primaryColor: '#182c47', primaryTextColor: '#f1f7ff', primaryBorderColor: '#76baff', lineColor: '#72b6c2', secondaryColor: '#173d44', tertiaryColor: '#352b47', edgeLabelBackground: '#0f1d30', textColor: '#c6dce3', background: '#0f1d30' },
        flowchart: { htmlLabels: false, curve: 'basis', padding: 13, wrappingWidth: 320, nodeSpacing: 32, rankSpacing: 26, useMaxWidth: false } });
    });
    for (const guide of guides.filter(g => !selected || g.slug === selected)) {
      console.log(`Rendering ${guide.slug}: Mermaid → SVG`);
      const source = await readFile(join(root, 'sources', `${guide.slug}.mmd`), 'utf8');
      const raw = await page.evaluate(async ({ source, slug }) => (await window.mermaid.render(`diagram-${slug}`, source)).svg, { source, slug: guide.slug });
      await writeFile(join(output, `${guide.slug}-mermaid.svg`), raw);
      const variants = await page.evaluate(raw => {
        const host = document.createElement('div'); host.innerHTML = raw; document.body.append(host);
        const svg = host.querySelector('svg');
        svg.setAttribute('x', '62'); svg.setAttribute('y', '286'); svg.setAttribute('width', '956'); svg.setAttribute('height', '754');
        svg.removeAttribute('style'); svg.setAttribute('preserveAspectRatio', 'xMidYMid meet');
        svg.querySelectorAll('.node rect').forEach(rect => { rect.setAttribute('rx', '10'); rect.setAttribute('ry', '10'); });
        const paths = [...svg.querySelectorAll('path.flowchart-link')];
        const staticSVG = svg.outerHTML;
        const ns = 'http://www.w3.org/2000/svg';
        paths.forEach((path, index) => {
          path.style.strokeWidth = '2.3px';
          for (let particle = 0; particle < 3; particle++) {
            const circle = document.createElementNS(ns, 'circle');
            circle.setAttribute('r', String(4.5 - particle * .9));
            circle.setAttribute('fill', particle === 0 ? '#d4fff0' : '#5eead4');
            circle.setAttribute('opacity', String(1 - particle * .27));
            circle.setAttribute('class', 'flow-particle');
            circle.setAttribute('aria-hidden', 'true');
            const motion = document.createElementNS(ns, 'animateMotion');
            motion.setAttribute('path', path.getAttribute('d'));
            motion.setAttribute('dur', '4s');
            motion.setAttribute('begin', `${-((index * .57 + particle * .1) % 4)}s`);
            motion.setAttribute('repeatCount', 'indefinite'); motion.setAttribute('calcMode', 'paced');
            circle.append(motion); path.parentNode.append(circle);
          }
        });
        const animatedSVG = svg.outerHTML; host.remove(); return { staticSVG, animatedSVG, edges: paths.length };
      }, raw);
      if (!variants.edges) throw new Error(`No edges found in ${guide.slug}`);
      const still = poster(guide, variants.staticSVG);
      const animated = poster(guide, variants.animatedSVG);
      await writeFile(join(output, `${guide.slug}.svg`), still);
      await writeFile(join(output, `${guide.slug}-animated.svg`), animated);
      const capture = await browser.newPage({ viewport: { width: 864, height: 1080 }, deviceScaleFactor: 1 });
      await capture.setContent(`<style>html,body{margin:0;background:#0b1423}body>svg{display:block;width:864px;height:1080px}</style>${animated}`);
      await capture.evaluate(async () => { await document.fonts.ready; document.querySelector('body>svg').pauseAnimations(); });
      console.log(`Encoding ${guide.slug}: 80 SVG frames → GIF (8 seconds)`);
      const encoder = GIFEncoder(); let palette;
      for (let frame = 0; frame < 80; frame++) {
        await capture.evaluate(time => document.querySelector('body>svg').setCurrentTime(time), frame / 10);
        const png = await capture.screenshot({ type: 'png', animations: 'allow' });
        const pixels = await sharp(png).ensureAlpha().raw().toBuffer();
        if (!palette) palette = quantize(pixels, 256);
        const indexed = applyPalette(pixels, palette);
        encoder.writeFrame(indexed, 864, 1080, { palette, delay: 100, repeat: 0 });
        if (frame === 20) await writeFile(join(temporary, `${guide.slug}-preview.png`), png);
      }
      encoder.finish();
      await writeFile(join(output, `${guide.slug}.gif`), encoder.bytes());
      await capture.close();
    }
  } finally {
    if (browser) await browser.close();
    await new Promise(done => server.close(done));
  }
}
await generatePages();
console.log(`Wrote six offline HTML pages to ${root}`);
