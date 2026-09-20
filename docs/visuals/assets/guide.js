// All media and controls work from file:// with no network or server dependency.
const player = document.querySelector('.player');
if (player) {
  const svg = document.querySelector('#vector svg');
  const motionPreference = matchMedia('(prefers-reduced-motion: reduce)');
  const views = ['vector', 'gif', 'still'];
  const toggle = document.querySelector('#play-toggle');
  const speed = document.querySelector('#speed');
  const status = document.querySelector('#play-status');
  const progress = document.querySelector('#progress');
  const gif = document.querySelector('#gif img');
  let mode = motionPreference.matches ? 'still' : 'vector';
  let playing = !motionPreference.matches;
  let elapsed = 0;
  let previous = performance.now();
  svg.pauseAnimations();
  svg.setCurrentTime(0);
  function update() {
    for (const view of views) document.getElementById(view).hidden = view !== mode || (view === 'gif' && !playing);
    if (mode === 'gif' && !playing) document.getElementById('still').hidden = false;
    document.querySelectorAll('[data-mode]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.mode === mode)));
    toggle.textContent = playing && mode !== 'still' ? 'Pause' : 'Play';
    toggle.disabled = mode === 'still';
    speed.disabled = mode !== 'vector';
    status.textContent = mode === 'still' ? 'Static view' : playing ? (mode === 'gif' ? 'GIF loop · fixed speed' : 'SVG motion · playing') : (mode === 'gif' ? 'Paused · static poster shown' : 'SVG motion · paused');
    if (mode === 'gif' && playing && !gif.src) gif.src = gif.dataset.src;
    if (mode !== 'vector') progress.style.width = '0%';
  }
  for (const button of document.querySelectorAll('[data-mode]')) button.addEventListener('click', () => {
    mode = button.dataset.mode;
    playing = mode !== 'still';
    update();
  });
  toggle.addEventListener('click', () => { playing = !playing; update(); });
  document.querySelector('#restart').addEventListener('click', () => {
    elapsed = 0; svg.setCurrentTime(0);
    if (mode === 'gif') { gif.removeAttribute('src'); gif.src = gif.dataset.src; }
  });
  document.querySelector('#theater').addEventListener('click', event => {
    const active = document.body.classList.toggle('theater');
    event.currentTarget.setAttribute('aria-pressed', String(active));
    event.currentTarget.textContent = active ? 'Compact' : 'Expand';
  });
  motionPreference.addEventListener('change', event => { if (event.matches) { mode = 'still'; playing = false; update(); } });
  function frame(now) {
    const delta = Math.min((now - previous) / 1000, .1); previous = now;
    if (mode === 'vector' && playing && !document.hidden) {
      elapsed = (elapsed + delta * Number(speed.value)) % 8;
      svg.setCurrentTime(elapsed);
      progress.style.width = `${elapsed / 8 * 100}%`;
    }
    requestAnimationFrame(frame);
  }
  update();
  requestAnimationFrame(frame);
}
