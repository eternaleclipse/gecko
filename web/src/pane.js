// A Pane is one xterm.js terminal bound to one session stream.
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { SearchAddon } from '@xterm/addon-search';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { ClipboardAddon, Base64 } from '@xterm/addon-clipboard';
import { copyText, isMac } from './util.js';

export const FONT = '"JetBrains Mono Variable", "JetBrains Mono", ui-monospace, "SF Mono", Menlo, Consolas, "DejaVu Sans Mono", monospace';

// OSC 52 writes go to the system clipboard (also through tmux and ssh);
// reads are refused so programs can't snoop on the clipboard.
const clipboardProvider = {
  readText: async () => '',
  writeText: async (_sel, text) => { await copyText(text); },
};

const TERMINAL_REPLY = /^(?:\x1b\[[?>=]?[\d;]*[cRn]|\x1b\[\?[\d;]*\$y|\x1b\[[IO]|\x1b\][\d;]*[^\x07\x1b]*(?:\x07|\x1b\\)|\x1bP[\s\S]*?\x1b\\)+$/;

export class Pane {
  constructor(app, id) {
    this.app = app;
    this.id = id;
    this.offset = -1; // next byte expected; -1 = replay everything
    this.muted = 0; // >0 while replaying history
    this.attached = false;
    this.exited = false;
    this.lastUsed = Date.now();
    this.el = document.createElement('div');
    this.el.className = 'pane';
    this.el.dataset.id = id;
    const s = app.settings;
    this.term = new Terminal({
      allowProposedApi: true,
      allowTransparency: true, // see-through desktop app and picture themes
      fontFamily: app.termFont || FONT,
      fontSize: app.termFontSize(),
      lineHeight: 1.15,
      scrollback: 20000,
      cursorBlink: true,
      cursorInactiveStyle: 'outline',
      macOptionIsMeta: s.optionIsMeta,
      macOptionClickForcesSelection: true,
      rightClickSelectsWord: !isMac,
      drawBoldTextInBrightColors: false,
      minimumContrastRatio: 1.1,
      smoothScrollDuration: 0,
      theme: app.termTheme,
    });
    this.fit = new FitAddon();
    this.search = new SearchAddon();
    this.term.loadAddon(this.fit);
    this.term.loadAddon(this.search);
    this.term.loadAddon(new WebLinksAddon((ev, uri) => {
      if (ev.metaKey || ev.ctrlKey || app.touch) window.open(uri, '_blank', 'noopener');
    }));
    const u = new Unicode11Addon();
    this.term.loadAddon(u);
    this.term.unicode.activeVersion = '11';
    this.term.loadAddon(new ClipboardAddon(new Base64(), clipboardProvider));
    this.term.open(this.el);
    this.loadRenderer();

    this.term.onData((d) => {
      // Replaying history makes xterm answer old queries (device attributes,
      // cursor reports) again; a pane nobody is looking at shouldn't answer
      // either, or two attached devices would both reply.
      if (TERMINAL_REPLY.test(d) && (this.muted > 0 || !this.visible || document.hidden)) return;
      app.sendInput(this.id, d);
    });
    this.term.onBinary((d) => {
      const b = new Uint8Array(d.length);
      for (let i = 0; i < d.length; i++) b[i] = d.charCodeAt(i) & 255;
      app.conn.input(this.id, b);
    });
    this.term.onSelectionChange(() => {
      if (app.settings.copyOnSelect && this.term.hasSelection()) copyText(this.term.getSelection());
    });
    this.term.attachCustomKeyEventHandler((ev) => app.keys.terminalKey(ev, this));
    this.term.onTitleChange(() => {});
    this.ro = new ResizeObserver(() => this.scheduleLayout());
    this.ro.observe(this.el);
  }

  loadRenderer() {
    // WebGL is fastest on desktops. Phones get the DOM renderer: lighter on
    // battery, and mobile browsers allow only a few WebGL contexts.
    const want = this.app.settings.renderer;
    if (want === 'dom' || (want === 'auto' && this.app.touch)) return;
    try {
      const gl = new WebglAddon();
      gl.onContextLoss(() => gl.dispose()); // fall back to the DOM renderer
      this.term.loadAddon(gl);
    } catch {
      // DOM renderer is fine
    }
  }

  get visible() {
    return this.el.classList.contains('active');
  }

  // Resizing is kept cheap while a window is being dragged:
  // - at most one fit per animation frame;
  // - row changes apply immediately, but column changes (which re-wrap the
  //   whole scrollback) wait until the size has settled;
  // - the program is only told the final size, so it redraws once instead
  //   of on every intermediate size.
  scheduleLayout() {
    if (this.layoutFrame) return;
    this.layoutFrame = requestAnimationFrame(() => {
      this.layoutFrame = 0;
      this.layout();
    });
  }

  layout(now = false) {
    if (!this.visible || !this.el.clientWidth || !this.el.clientHeight) return;
    const dims = this.fit.proposeDimensions();
    if (!dims || !Number.isFinite(dims.cols) || !Number.isFinite(dims.rows)) return;
    const { cols, rows } = dims;
    clearTimeout(this.colsTimer);
    if (cols !== this.term.cols && !now) {
      if (rows !== this.term.rows) this.term.resize(this.term.cols, rows);
      this.colsTimer = setTimeout(() => this.layout(true), 120);
      return;
    }
    if (cols !== this.term.cols || rows !== this.term.rows) this.term.resize(cols, rows);
    clearTimeout(this.sizeTimer);
    if (now) this.sendSize();
    else this.sizeTimer = setTimeout(() => this.sendSize(), 120);
  }

  // Called when the pane becomes visible: size it right away and repaint.
  show() {
    this.layout(true);
    this.term.refresh(0, this.term.rows - 1);
    this.sendSize(true); // the device you're looking at decides the size
  }

  sendSize(force) {
    const { cols, rows } = this.term;
    const info = this.app.sessions.get(this.id);
    if (!force && info && info.cols === cols && info.rows === rows) return;
    this.app.conn.send({ t: 'resize', id: this.id, cols, rows });
  }

  attach() {
    this.attached = true;
    this.replayUntil = this.app.sessions.get(this.id)?.offset ?? 0;
    this.app.conn.send({ t: 'attach', id: this.id, sub: this.id, offset: this.offset });
  }

  detach() {
    if (!this.attached) return;
    this.attached = false;
    this.app.conn.send({ t: 'detach', sub: this.id });
  }

  write(off, data) {
    if (this.offset >= 0) {
      const end = off + data.length;
      if (end <= this.offset) return; // already have it
      if (off < this.offset) data = data.subarray(this.offset - off);
    }
    this.offset = off + data.length;
    if (off < this.replayUntil) {
      this.muted++;
      this.term.write(data, () => this.muted--);
    } else {
      this.term.write(data);
    }
  }

  reset(off, prefix) {
    this.term.reset();
    if (prefix) this.term.write(prefix);
    this.offset = off;
  }

  markExited(code) {
    if (this.exited) return;
    this.exited = true;
    this.term.write(`\r\n\x1b[2m[exited with code ${code}; press any key to close]\x1b[0m`);
  }

  setTheme(t) { this.term.options.theme = t; }

  setFontFamily(f) {
    if (this.term.options.fontFamily === f) return;
    this.term.options.fontFamily = f;
    this.layout(true);
  }

  setFontSize(n) {
    this.term.options.fontSize = n;
    this.layout(true);
  }

  focus() { this.term.focus(); }

  dispose() {
    this.detach();
    this.ro.disconnect();
    cancelAnimationFrame(this.layoutFrame);
    clearTimeout(this.colsTimer);
    clearTimeout(this.sizeTimer);
    this.term.dispose();
    this.el.remove();
  }
}
