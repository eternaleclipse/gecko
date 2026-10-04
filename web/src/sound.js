// The startup chime: three soft bell notes rising, synthesized with Web
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
    out.gain.value = 0.16;
    out.connect(ctx.destination);
    // E5, B5, E6: an open fifth and octave, bright without being a jingle.
    [659.25, 987.77, 1318.51].forEach((f, i) => {
      const t = t0 + i * 0.11;
      for (const [mult, level] of [[1, 1], [2, 0.18], [3, 0.06]]) {
        const o = ctx.createOscillator();
        const g = ctx.createGain();
        o.type = 'sine';
        o.frequency.value = f * mult;
        g.gain.setValueAtTime(0, t);
        g.gain.linearRampToValueAtTime(level, t + 0.008);
        g.gain.exponentialRampToValueAtTime(0.0001, t + 1.1 / mult + 0.25);
        o.connect(g).connect(out);
        o.start(t);
        o.stop(t + 1.5);
      }
    });
    setTimeout(() => ctx.close().catch(() => {}), 2200);
  };
  if (ctx.state === 'suspended') {
    // Autoplay was blocked; give up quietly rather than chime on a later click.
    ctx.resume().then(() => { if (ctx.state === 'running') play(); else ctx.close().catch(() => {}); }, () => {});
    setTimeout(() => { if (ctx.state === 'suspended') ctx.close().catch(() => {}); }, 300);
  } else {
    play();
  }
}
