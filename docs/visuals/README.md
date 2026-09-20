# Replica Control visual field guide

Open [index.html](index.html) directly in a browser. No server, Go process, Docker daemon,
network connection or package installation is needed to view the guides.
Keep this directory and its assets together when copying it elsewhere.

[Repository README](../../README.md) | [Visual validation results](VALIDATION.md) | [Shared level guides](../../levels/README.md)

To open the gallery from PowerShell at the repository root:

```powershell
$repoRoot = (Get-Location).Path
Start-Process (Join-Path $repoRoot 'docs/visuals/index.html')
```

## Guides

| Page | What it explains |
|---|---|
| [Architecture](architecture.html) | Authenticated requests, shared services, caches and Kubernetes |
| [Levels 1–5](levels.html) | Cumulative capabilities within the same shared server |
| [Reconciliation](reconciliation.html) | Durable intent, leader workers, ownership checks and convergence |
| [Rolling updates](rollout.html) | New capacity, readiness, routing and request draining |
| [Development pipeline](delivery.html) | Branch source checks, optional PR checks, full main/tag validation and artifacts |

Each page includes motion SVG, GIF and still modes; playback controls; an expanded
view; a prose walkthrough; implementation links; and Mermaid source. Reduced-motion
preferences select the static view automatically. GIF pause shows the static poster
because a browser image element cannot seek or freeze a GIF at an exact frame.
The SVG mode provides true pause, restart and speed control.

The diagrams describe the current development reference. Moving markers show
directions of work, not real traffic, measured latency or proof of availability.
The level-5 acknowledgement means persisted intent, not ready Pods. The rollout
guide shows monitoring through confirmed upgrade completion and ten seconds
afterward. See [integration validation](../INTEGRATION-VALIDATION.md),
[the historical audit](../AUDIT.md) and [dependency validation](../DEPENDENCY-VALIDATION.md)
for evidence boundaries.

## Export formats

For each guide, `assets/` contains:

- `<name>-mermaid.svg`: the direct Mermaid rendering.
- `<name>.svg`: the styled, static 1080 × 1350 portrait poster.
- `<name>-animated.svg`: the same poster with motion along the actual Mermaid paths.
- `<name>.gif`: an 864 × 1080 export, 80 frames at 10 fps, eight seconds, looping.

The 4:5 portrait layout is intended for clear local presentations and social-style
graphics. No social account is connected and nothing is published. SVG retains
sharp text at any size; GIF has a limited color palette and a fixed playback rate.
The HTML includes the animated SVG inline and embeds the GIF using a relative
image path. There are no CDN scripts, remote fonts, analytics or external requests.

## Rebuild on Windows

Requirements are Node.js 22.12 or later and Microsoft Edge. The application’s Go
toolchain does not need these tools; the visual build is separate and optional.
The builder derives every path from its own script location.

From the repository root:

```powershell
$repoRoot = (Get-Location).Path
$toolsRoot = Join-Path $repoRoot 'docs/visuals/tools'
$cacheRoot = Join-Path $repoRoot '.local/npm-cache'
npm --cache "$cacheRoot" --prefix "$toolsRoot" ci --no-fund
node (Join-Path $toolsRoot 'build.mjs')
node (Join-Path $toolsRoot 'check.mjs')
Start-Process (Join-Path $repoRoot 'docs/visuals/index.html')
```

The pinned development packages and their dependency lock are under `tools/`:
Mermaid 12.0.0, Playwright 1.63.0, sharp 0.35.4 and gifenc 1.0.3. Installation
requires internet access; subsequent viewing requires none. Edge runs headless
while exporting and checking the graphics. Set `VISUAL_BROWSER_CHANNEL=chrome`
to use installed Chrome, or `bundled` to use Playwright Chromium.

On Linux/macOS with Node and the required Chromium system libraries:

```bash
repo_root="$(pwd)"
tools_root="$repo_root/docs/visuals/tools"
npm --prefix "$tools_root" ci --no-fund
node "$tools_root/node_modules/playwright/cli.js" install chromium
VISUAL_BROWSER_CHANNEL=bundled node "$tools_root/build.mjs"
VISUAL_BROWSER_CHANNEL=bundled node "$tools_root/check.mjs"
```

The Windows/Edge workflow is the exercised path; Linux/macOS commands are provided
for portability and were not run in this change.

## Edit and validate

Edit diagrams in `sources/*.mmd` and explanatory content in `tools/guides.mjs`.
The builder uses Mermaid to render SVG, decorates the resulting edges with SVG
motion, captures deterministic animation times, and encodes those frames as GIF.
It then embeds the SVG and GIF assets into the six HTML pages. Do not hand-edit
generated pages or exports: rebuild them after changing sources.

Use `node build.mjs --only=architecture` from `tools/` to rebuild one graphic and
all HTML pages, or `node build.mjs --html-only` after changing only HTML page copy.
Changes to diagram sources, poster titles, captions or fact cards require a full
graphic rebuild. Shared CSS and viewer JavaScript are loaded directly by the pages.
Both shortcuts require the other generated assets to exist already.

`check.mjs` checks file links, offline page loads, browser errors, desktop/mobile
overflow, diagram bounds, pause/mode controls, GIF dimensions/frame count/duration,
actual frame differences and reduced-motion behavior. Screenshots and a JSON
receipt go to ignored `.build/`. Node dependencies are ignored; these visual files
are excluded from the Go application’s Docker build context.

The [recorded validation](VALIDATION.md) covers all six HTML pages and all five
animated GIFs. Rebuilding graphics does not run the Go application tests or prove
live cluster behavior; those checks are documented in the repository README.

References: [Mermaid usage](https://mermaid.js.org/config/usage.html),
[SVG animation](https://developer.mozilla.org/en-US/docs/Web/SVG/Element/animateMotion),
[Playwright](https://playwright.dev/docs/intro),
[gifenc](https://github.com/mattdesl/gifenc).
