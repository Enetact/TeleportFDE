# Visual guide validation

Validated on 2026-09-20 using Node.js 24.16.0 and headless Microsoft Edge on Windows.
The checks load the actual delivered HTML through `file://`, with HTTP/HTTPS
requests blocked and recorded. No development server is needed to view the files.

| Check | Result |
|---|---|
| Mermaid parsing/rendering | All five source diagrams rendered successfully |
| Delivered pages | Index plus five walkthroughs loaded successfully |
| Relative asset and source links | All checked local targets exist |
| Offline viewing | Zero external requests |
| Desktop/mobile layout | No horizontal document overflow at 1440px and 390px |
| Diagram geometry | Nodes remain inside the diagram view boxes |
| Playback | SVG pause, format switching, GIF loading/pause fallback, speed selection and expanded view passed |
| Reduced motion | Static view selected; GIF not loaded automatically |
| GIF exports | Five files, each 864 × 1080, 80 frames, eight seconds, infinite loop |
| Actual motion | Decoded frames at different times have different pixel hashes |
| Visual review | All five poster previews, desktop gallery, desktop guide and mobile guide inspected |

Each GIF is approximately 3.7–4.0 MiB. Static and animated SVGs use the same
Mermaid layout as the GIF frames. The rendered diagrams were reviewed against
server startup, controller behavior, the Helm Deployment and the development
workflow. In particular, readiness permits Service routing before the separate
minReadySeconds stability period completes.

Reproduce with `node docs/visuals/tools/check.mjs` after installing the pinned
visual-build dependencies. The generated screenshots and detailed JSON receipt
are in ignored `docs/visuals/.build/`. Validation is specific to the exercised
Windows/Edge environment; it is not a live Kubernetes availability result.
