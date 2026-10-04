// The startup chime: three low, soft notes rising, synthesized with Web
// Audio so there is no file to load. Browsers may refuse to play sound
// before the first click; then it stays silent.
export function startupChime() {
  const Ctx = window.AudioContext || window.webkitAudioContext;
  if (!Ctx) return;
  let ctx;
  try { ctx = new Ctx(); } catch { return; }
  const play = () => {
    const t0 = ctx.currentTime + 0.05;
    const out = ctx.createGain();
    out.gain.value = 0.24;
    out.connect(ctx.destination);
    // E3, B3, E4: an open fifth and octave, low and warm. The overtones
    // carry it on small laptop speakers that barely play the fundamental.
    [164.81, 246.94, 329.63].forEach((f, i) => {
      const t = t0 + i * 0.13;
      for (const [mult, level] of [[1, 1], [2, 0.35], [3, 0.12]]) {
        const o = ctx.createOscillator();
        const g = ctx.createGain();
        o.type = 'sine';
        o.frequency.value = f * mult;
        g.gain.setValueAtTime(0, t);
        g.gain.linearRampToValueAtTime(level, t + 0.02);
        g.gain.exponentialRampToValueAtTime(0.0001, t + 1.4 / mult + 0.3);
        o.connect(g).connect(out);
        o.start(t);
        o.stop(t + 1.8);
      }
    });
    setTimeout(() => ctx.close().catch(() => {}), 2600);
  };
  if (ctx.state === 'suspended') {
    // Autoplay was blocked; give up quietly rather than chime on a later click.
    ctx.resume().then(() => { if (ctx.state === 'running') play(); else ctx.close().catch(() => {}); }, () => {});
    setTimeout(() => { if (ctx.state === 'suspended') ctx.close().catch(() => {}); }, 300);
  } else {
    play();
  }
}
