import '@xterm/xterm/css/xterm.css';
import '@fontsource-variable/jetbrains-mono';
import '@fontsource-variable/instrument-sans';
import '@fontsource/barlow/400.css'; // Mission Control skin (DIN-style); only downloaded when used
import '@fontsource/barlow/500.css';
import '@fontsource/barlow/600.css';
// Theme fonts (Latin only; a font downloads only when its theme is in use).
import '@fontsource/zen-maru-gothic/latin-400.css';
import '@fontsource/zen-maru-gothic/latin-700.css';
import '@fontsource/rajdhani/latin-500.css';
import '@fontsource/rajdhani/latin-700.css';
import '@fontsource/orbitron/latin-600.css';
import '@fontsource/share-tech-mono/latin-400.css';
import '@fontsource/zen-kaku-gothic-new/latin-400.css';
import '@fontsource/zen-kaku-gothic-new/latin-700.css';
import '@fontsource/dela-gothic-one/latin-400.css';
import '@fontsource/bebas-neue/latin-400.css';
import '@fontsource/barlow-semi-condensed/latin-500.css';
import '@fontsource/barlow-semi-condensed/latin-700.css';
import '@fontsource/josefin-sans/latin-400.css';
import '@fontsource/josefin-sans/latin-600.css';
import '@fontsource/zilla-slab/latin-500.css';
import '@fontsource/zilla-slab/latin-700.css';
import '@fontsource/ibm-plex-mono/latin-400.css';
import '@fontsource/ibm-plex-mono/latin-600.css';
import '@fontsource/m-plus-rounded-1c/latin-500.css';
import '@fontsource/m-plus-rounded-1c/latin-700.css';
import '@fontsource/mochiy-pop-one/latin-400.css';
import './style.css';
import qrcode from 'qrcode-generator';

const splash = window.__splash || { set() {}, done() {} };
splash.set(0.4); // the app's code is here
import { Conn } from './conn.js';
import { Terminal } from '@xterm/xterm';
import { WebglAddon } from '@xterm/addon-webgl';
import { Pane, FONT } from './pane.js';
import { THEMES, AUTO, resolveTheme, paintTheme, swatch } from './themes.js';
import { Keys, prettyCombo, comboOf, ACTIONS, DEFAULTS, DEFAULT_TAB_MODIFIER, TAB_MODIFIERS } from './keys.js';
import { $, esc, ago, shortPath, copyText, readClipboard, fuzzy, store, save, isTouch, isMac } from './util.js';

const MAX_LIVE_PANES = 12;

// The theme this browser last used. "theme" once meant system/dark/light
// and the theme id was stored as "scheme"; both still load.
function initialTheme() {
  const old = { system: AUTO, dark: 'gecko-dark', light: 'gecko-light' };
  const t = store('theme', null);
  if (t && !old[t]) return t;
  return store('scheme', old[t] || AUTO);
}

// Settings saved in ~/.gecko-terminal/settings.json (the rest of what the
// browser stores is per-window state, like which tab is open).
const SYNCED = ['theme', 'opacity', 'fontSize', 'mobileFontSize', 'copyOnSelect', 'smartCopy', 'optionIsMeta', 'renderer', 'sidebarHidden', 'keys'];

// Protocol version this client speaks; see internal/proto.
const PROTOCOL = 6;
const OUTDATED = 'The Gecko background service is older than this window, so some features are missing. Click to restart it.';

const STATUS_TEXT = { working: 'Working', 'needs-input': 'Needs you', idle: 'Idle', running: 'Running' };

function basename(p) {
  if (!p) return '';
  const parts = p.replace(/\/+$/, '').split('/');
  return parts[parts.length - 1] || '/';
}

export function tabLabel(s) {
  // tmux tabs are titled by the active tmux window, unless named by hand
  // (older Gecko named tmux tabs after their session automatically).
  if (s.kind === 'tmux' && s.tmuxWindow && (!s.name || s.name === s.tmuxSession || /^tmux /.test(s.name))) return s.tmuxWindow;
  if (s.name) return s.name;
  if (s.agent) return s.agent.name;
  if ((s.kind === 'ssh' || s.kind === 'mosh') && s.remote) return s.remote;
  if (s.kind === 'tmux' && s.tmuxSession) return s.tmuxSession; // the status line says it's tmux
  if (s.kind !== 'shell' && s.proc) return s.proc;
  return s.repo || basename(s.cwd) || s.shell || 'shell';
}

// Git info drawn by Gecko itself, so prompts don't need powerline or a
// patched font: branch icon, branch, and working-tree state.
const BRANCH_ICON = '<svg class="gi" viewBox="0 0 16 16" aria-hidden="true"><circle cx="4.5" cy="3.5" r="1.7"/><circle cx="4.5" cy="12.5" r="1.7"/><circle cx="11.5" cy="5" r="1.7"/><path d="M4.5 5.2v5.6M11.5 6.7c0 3-6.2 2.2-6.8 4.1"/></svg>';

function gitStateText(g) {
  if (!g) return '';
  const bits = [];
  if (g.changed) bits.push(`${g.changed} changed`);
  if (g.untracked) bits.push(`${g.untracked} untracked`);
  if (g.ahead) bits.push(`${g.ahead} ahead`);
  if (g.behind) bits.push(`${g.behind} behind`);
  return bits.length ? bits.join(', ') : 'clean';
}

// full: also show counts and ahead/behind (status line). Otherwise just a
// dot when there are uncommitted changes (tabs).
function gitChip(s, full = false) {
  if (!s.repo) return '';
  const g = s.git;
  const prefix = tabLabel(s) === s.repo && !full ? '' : `<span class="repo-name">${esc(s.repo)}</span>`;
  let state = '';
  if (g) {
    if (full) {
      if (g.changed) state += `<b class="dirty">●${g.changed}</b>`;
      if (g.untracked) state += `<b class="untracked">?${g.untracked}</b>`;
      if (g.ahead) state += `<b class="ab">↑${g.ahead}</b>`;
      if (g.behind) state += `<b class="ab">↓${g.behind}</b>`;
    } else if (g.changed || g.untracked) {
      state = '<b class="dirty">●</b>';
    }
  }
  const tip = `${s.repo} on ${s.branch || '?'}${g ? ' · ' + gitStateText(g) : ''}`;
  return `<span class="git" title="${esc(tip)}">${prefix}${s.branch ? BRANCH_ICON + `<span class="br">${esc(s.branch)}</span>` : ''}${state}</span>`;
}

// Where in the repo the tab is, e.g. "src/auth" (empty at the root).
function repoPath(s) {
  if (!s.repoRoot || !s.cwd || !s.cwd.startsWith(s.repoRoot)) return '';
  return s.cwd.slice(s.repoRoot.length).replace(/^\/+/, '');
}

// Text with `code` spans, as in Markdown: hostnames, commands and names in
// messages stand out from the sentence around them.
function md(text) {
  return esc(text).replace(/`([^`]+)`/g, '<code>$1</code>');
}

// Compact condensed tags (SSH, MOSH, TMUX) for the status line, in the
// theme's accent. The tab title stays just the name.
const tag = (text) => `<span class="ctag" aria-hidden="true">${text}</span>`;
const TMUX_ICON = tag('TMUX');

// Tabs show their number (the Alt/⌘+N shortcut) in a square whose color
// carries the state: pink needs you, ochre working, sage idle agent.
function numGlyph(s, n) {
  const st = s.agent ? s.agent.status : s.attention ? 'bell' : s.running || (s.kind && s.kind !== 'shell') ? 'busy' : '';
  const label = s.agent ? STATUS_TEXT[s.agent.status] || s.agent.status : st === 'bell' ? 'Bell' : '';
  return `<i class="num ${st ? 's-' + st : ''}"${label ? ` title="${esc(label)}"` : ''}>${n}</i>`;
}

function glyph(s) {
  if (s.agent) return `<i class="dot s-${s.agent.status}" title="${esc(STATUS_TEXT[s.agent.status] || s.agent.status)}"></i>`;
  if (s.attention) return '<i class="dot s-bell" title="Bell"></i>';
  if (s.running || (s.kind !== 'shell' && s.kind)) return '<i class="dot s-busy"></i>';
  return '<i class="dot"></i>';
}

class App {
  constructor() {
    this.sessions = new Map();
    this.hosts = [];
    this.tmux = [];
    this.templates = [];
    this.hostName = '';
    this.panes = new Map();
    this.exited = new Map(); // id -> info of sessions that failed while shown
    this.closing = new Set(); // ids the user closed
    this.touch = isTouch;
    // Running inside the Gecko desktop app (Electron) rather than a browser.
    this.desktop = window.geckoDesktop || null;
    this.settings = {
      fontSize: store('fontSize', 13), // desktop
      mobileFontSize: store('mobileFontSize', 12), // phones and tablets
      copyOnSelect: store('copyOnSelect', false),
      smartCopy: store('smartCopy', true),
      optionIsMeta: store('optionIsMeta', true),
      theme: initialTheme(),
      renderer: store('renderer', 'auto'), // auto | webgl | dom
      opacity: store('opacity', 0.9), // desktop app background opacity
      sidebarHidden: store('sidebarHidden', false),
      keys: store('keys', {}), // key binding overrides
    };
    // `gecko open` asks for a window that fits a terminal of this size,
    // centered; done once, then dropped from the URL.
    const url = new URL(location.href);
    const fit = /^(\d+)x(\d+)$/.exec(url.searchParams.get('fit') || '');
    if (fit) {
      this.fitRequest = { cols: +fit[1], rows: +fit[2] };
      url.searchParams.delete('fit');
      history.replaceState(null, '', url);
    }
    this.ws = store('workspace', 'default');
    this.activeByWs = store('activeByWs', {});
    this.collapsed = store('collapsed', {});
    this.tmuxOpen = store('tmuxOpen', {}); // tmux sessions expanded in the sidebar
    this.mods = { ctrl: false, alt: false };
    this.agentSeen = new Map();
    this.pendingFocus = null;
    this.applyTheme();

    this.conn = new Conn({
      onOpen: () => {
        splash.set(0.85);
        for (const p of this.panes.values()) if (p.attached) p.attach();
        const a = this.activePane();
        if (a) a.sendSize(true);
      },
      onStatus: () => this.renderSoon(),
      onMessage: (m) => this.onMessage(m),
      onData: (sub, off, data) => this.panes.get(sub)?.write(off, data),
    });
    this.keys = new Keys(this);
    this.bindUI();
    this.conn.connect();
    setInterval(() => this.renderSoon(), 5000); // relative times
  }

  // ---------------------------------------------------------------- state

  onMessage(m) {
    switch (m.t) {
      case 'hello':
        this.hostName = m.host;
        // A different start id means the service restarted; finish a
        // "Restart Gecko" by reloading this window into the new version.
        if (this.restarting && m.text && m.text !== this.startId) location.reload();
        this.startId = m.text;
        this.outdated = (m.version || 1) < PROTOCOL;
        if (this.outdated) this.toast(OUTDATED);
        break;
      case 'sessions': {
        const fresh = new Map((m.sessions || []).map((s) => [s.id, s]));
        for (const [id, s] of this.sessions) {
          if (m.host && s.host !== m.host) fresh.set(id, s);
        }
        for (const id of this.sessions.keys()) if (!fresh.has(id)) this.sessionGone(id);
        this.sessions = fresh;
        this.firstSync = true;
        splash.done(); // tabs are in: ready
        this.ensureWorkspace();
        break;
      }
      case 'session': {
        const s = m.session;
        if (this.closing.has(s.id)) break;
        this.sessions.set(s.id, s);
        this.exited.delete(s.id);
        this.watchAgent(s);
        if (this.pendingFocus === s.id) {
          this.pendingFocus = null;
          this.focusSession(s.id);
        }
        break;
      }
      case 'closed':
        this.sessionGone(m.id);
        this.closing.delete(m.id);
        break;
      case 'state':
        this.hosts = m.hosts || [];
        this.tmux = m.tmux || [];
        this.templates = m.templates || [];
        break;
      case 'settings':
        this.onSettings(m.text);
        return;
      case 'hostlog':
        this.hostSetup?.log(m.sub, m.text);
        return;
      case 'reset':
        this.panes.get(m.sub)?.reset(m.offset, m.prefix);
        break;
      case 'exit': {
        const p = this.panes.get(m.sub);
        if (p && m.error && !this.sessions.has(m.sub)) this.sessionGone(m.sub);
        break;
      }
    }
    this.renderSoon();
  }

  // A session ended. Its tab closes, unless the process failed while you
  // were looking at it: then it stays so you can read why.
  sessionGone(id) {
    const info = this.sessions.get(id);
    const p = this.panes.get(id);
    const failed = info?.exited && (info.exitCode ?? 0) !== 0 && !this.closing.has(id);
    if (p && p.visible && failed) {
      this.sessions.delete(id);
      this.exited.set(id, info);
      p.markExited(info.exitCode);
      this.renderSoon();
      return;
    }
    this.selectNeighbor(id);
    this.sessions.delete(id);
    if (p) {
      p.dispose();
      this.panes.delete(id);
    }
    this.renderSoon();
  }

  // Before a tab disappears, make the tab next to it active.
  selectNeighbor(id) {
    const s = this.sessions.get(id) || this.exited.get(id);
    if (!s || this.activeByWs[s.workspace] !== id) return;
    const tabs = this.tabsOf(s.workspace);
    const i = tabs.findIndex((t) => t.id === id);
    const next = tabs[i + 1] || tabs[i - 1];
    if (next) this.activeByWs[s.workspace] = next.id;
    else delete this.activeByWs[s.workspace];
    save('activeByWs', this.activeByWs);
  }

  // Every tab in a workspace, including ones whose process just exited.
  tabsOf(ws) {
    const out = [];
    for (const s of this.sessions.values()) if (s.workspace === ws) out.push(s);
    for (const s of this.exited.values()) if (s.workspace === ws) out.push({ ...s, exited: true });
    return out.sort((a, b) => (a.created || 0) - (b.created || 0));
  }

  workspaces() {
    const names = new Map();
    for (const s of this.sessions.values()) names.set(s.workspace, true);
    for (const s of this.exited.values()) names.set(s.workspace, true);
    names.set(this.ws, true);
    return [...names.keys()].sort((a, b) => (a === 'default' ? -1 : b === 'default' ? 1 : a.localeCompare(b)));
  }

  ensureWorkspace() {
    if (this.tabsOf(this.ws).length) return;
    const first = [...this.sessions.values()][0];
    if (first) this.ws = first.workspace;
  }

  activeId() {
    const tabs = this.tabsOf(this.ws);
    const want = this.activeByWs[this.ws];
    if (tabs.some((t) => t.id === want)) return want;
    return tabs.length ? tabs[tabs.length - 1].id : null;
  }

  activeSession() {
    const id = this.activeId();
    return id ? this.sessions.get(id) || this.exited.get(id) : null;
  }

  activePane() {
    const id = this.activeId();
    return id ? this.panes.get(id) : null;
  }

  // ---------------------------------------------------------------- agents

  allAgents() {
    const out = [];
    for (const s of this.sessions.values()) if (s.agent) out.push({ key: s.id, session: s, state: s.agent, host: s.host });
    const rank = { 'needs-input': 0, working: 1, idle: 2, running: 3 };
    return out.sort((a, b) => (rank[a.state.status] ?? 9) - (rank[b.state.status] ?? 9) || (b.state.since || 0) - (a.state.since || 0));
  }

  watchAgent(s) {
    const prev = this.agentSeen.get(s.id);
    const now = s.agent?.status;
    this.agentSeen.set(s.id, now);
    if (!prev || !now || prev === now) return;
    const finished = prev === 'working' && now === 'idle';
    if (now !== 'needs-input' && !finished) return;
    const visible = !document.hidden && this.activeId() === s.id;
    if (visible) return;
    const title = `${s.agent.name} ${finished ? 'finished' : 'needs you'} · ${tabLabel(s)}`;
    this.notify(title, s.agent.detail || '', s.id);
  }

  notify(title, body, id) {
    if ('Notification' in window && Notification.permission === 'granted' && document.hidden) {
      const n = new Notification(title, { body, tag: id || title, icon: '/icon.svg' });
      n.onclick = () => { window.focus(); if (id) this.focusSession(id); n.close(); };
    } else {
      this.toast(title, id ? () => this.focusSession(id) : null);
    }
  }

  // ---------------------------------------------------------------- panes

  ensurePane(id) {
    let p = this.panes.get(id);
    if (!p) {
      p = new Pane(this, id);
      this.panes.set(id, p);
      $('#panes').appendChild(p.el);
      this.setupTouch(p);
      if (this.sessions.has(id)) p.attach();
      this.trimPanes();
    }
    p.lastUsed = Date.now();
    return p;
  }

  // Keep memory bounded: far-away tabs drop their terminal and re-attach
  // (replaying the server-side buffer) when visited again.
  trimPanes() {
    if (this.panes.size <= MAX_LIVE_PANES) return;
    const victims = [...this.panes.values()].filter((p) => !p.visible).sort((a, b) => a.lastUsed - b.lastUsed);
    while (this.panes.size > MAX_LIVE_PANES && victims.length) {
      const v = victims.shift();
      v.dispose();
      this.panes.delete(v.id);
    }
  }

  showActive() {
    const id = this.activeId();
    for (const p of this.panes.values()) {
      const on = p.id === id;
      if (on !== p.visible) p.el.classList.toggle('active', on);
    }
    $('#empty').hidden = !!id;
    if (!id) {
      this.shownId = null;
      return;
    }
    const p = this.ensurePane(id);
    if (!p.visible) p.el.classList.add('active');
    if (this.shownId === id) return;
    this.shownId = id;
    requestAnimationFrame(() => {
      p.show();
      if (this.fitRequest) {
        const { cols, rows } = this.fitRequest;
        this.fitRequest = null;
        this.fitWindow(cols, rows);
      }
      if (!this.paletteOpen && !this.textViewOpen && !(this.touch && document.activeElement?.closest('#compose'))) p.focus();
    });
    const s = this.sessions.get(id);
    if (s?.attention) this.conn.send({ t: 'patch', id, patch: { seen: true } });
  }

  focusSession(id) {
    const s = this.sessions.get(id) || this.exited.get(id);
    if (!s) return;
    this.ws = s.workspace;
    this.activeByWs[this.ws] = id;
    save('workspace', this.ws);
    save('activeByWs', this.activeByWs);
    this.closeDrawer();
    this.render();
  }

  // All tabs in sidebar order. Tab numbers (and Alt/⌘+N) run across
  // workspaces, so every tab has one stable number.
  allTabs() {
    return this.workspaces().flatMap((ws) => this.tabsOf(ws));
  }

  tabNumbers() {
    return new Map(this.allTabs().map((t, i) => [t.id, i + 1]));
  }

  selectTabIndex(i) {
    const t = this.allTabs()[i];
    if (t) this.focusSession(t.id);
  }

  cycleTab(d) {
    const tabs = this.tabsOf(this.ws);
    if (!tabs.length) return;
    const i = tabs.findIndex((t) => t.id === this.activeId());
    this.focusSession(tabs[(i + d + tabs.length) % tabs.length].id);
  }

  switchWorkspace(ws) {
    this.ws = ws;
    save('workspace', ws);
    this.closeDrawer();
    this.render();
  }

  cycleWorkspace(d) {
    const all = this.workspaces();
    const i = all.indexOf(this.ws);
    this.switchWorkspace(all[(i + d + all.length) % all.length]);
  }

  sendInput(id, data) {
    if (this.exited.has(id)) {
      this.closeTab(id, true);
      return;
    }
    if (this.mods.ctrl || this.mods.alt) {
      if (this.mods.ctrl && data.length === 1) {
        const c = data.toUpperCase().charCodeAt(0);
        if (c >= 64 && c <= 95) data = String.fromCharCode(c - 64);
        else if (data === ' ') data = '\x00';
      }
      if (this.mods.alt) data = '\x1b' + data;
      if (!this.mods.locked) this.mods = { ctrl: false, alt: false };
      this.renderKeybar();
    }
    this.conn.input(id, data);
  }

  // ---------------------------------------------------------------- actions

  async newTab(opts = {}) {
    const cur = this.activeSession();
    const p = this.activePane();
    const req = {
      host: opts.host ?? (cur && !cur.exited ? cur.host : this.hostName),
      workspace: opts.workspace ?? this.ws,
      cwd: opts.cwd ?? (cur && (opts.host ?? cur.host) === cur.host ? cur.cwd : ''),
      name: opts.name || '',
      command: opts.command || '',
      argv: opts.argv,
      cols: p?.term.cols || 80,
      rows: p?.term.rows || 24,
    };
    try {
      const r = await this.conn.request({ t: 'create', create: req });
      this.sessions.set(r.session.id, r.session);
      this.focusSession(r.session.id);
    } catch (e) {
      this.toast(`Couldn't open a tab on ${req.host}: ${e.message}`);
    }
  }

  async closeTab(id = this.activeId(), force = false) {
    if (!id) return;
    if (this.exited.has(id)) {
      this.selectNeighbor(id);
      this.exited.delete(id);
      this.panes.get(id)?.dispose();
      this.panes.delete(id);
      this.render();
      return;
    }
    const s = this.sessions.get(id);
    if (!force && s && (s.agent || (s.kind && s.kind !== 'shell') || s.running)) {
      const what = s.agent ? `${s.agent.name} (agent)` : s.command || s.proc || s.kind;
      const ok = await this.confirm(`Close \`${tabLabel(s)}\`? This ends \`${what}\`.`, 'Close tab');
      if (!ok) return;
    }
    // Close right away; don't wait for the process to finish exiting.
    this.closing.add(id);
    this.conn.send({ t: 'kill', id });
    this.sessionGone(id);
  }

  // The Gecko tab already showing tmux session `name` on `host`, if any.
  tmuxTab(host, name) {
    return [...this.sessions.values()].find((s) => s.host === host && s.tmuxSession === name) || null;
  }

  // Show tmux session `name` (optionally window `index`): go to the tab that
  // already shows it, or open one. Never a second tab for the same session.
  async openTmux(host, name, index = null) {
    const tab = this.tmuxTab(host, name);
    if (tab) {
      this.focusSession(tab.id);
      if (index != null) await this.tmuxDo(host, 'select-window', `${name}:${index}`);
      return;
    }
    await this.attachTmux(host, index == null ? name : `${name}:${index}`);
  }

  // Single click acts after a short wait; a second click on the same thing
  // within it is a double click instead (so renaming never opens a tab).
  clickOrDouble(key, single, double) {
    if (this.pendingClick?.key === key) {
      clearTimeout(this.pendingClick.timer);
      this.pendingClick = null;
      return double();
    }
    clearTimeout(this.pendingClick?.timer);
    this.pendingClick = { key, timer: setTimeout(() => { this.pendingClick = null; single(); }, 250) };
  }

  async renameTmuxSession(host, name) {
    const n = (await this.prompt('Rename tmux session', name, ''))?.trim();
    if (!n || n === name) return;
    await this.tmuxDo(host, 'rename-session', name, n);
    const k = host + '/' + name;
    if (k in this.tmuxOpen) {
      this.tmuxOpen[host + '/' + n] = this.tmuxOpen[k];
      delete this.tmuxOpen[k];
      save('tmuxOpen', this.tmuxOpen);
    }
  }

  async renameTmuxWindow(host, session, index) {
    const w = this.tmux.find((t) => t.host === host && t.name === session)?.wins?.find((x) => x.index === index);
    const n = (await this.prompt('Rename tmux window', w?.name || '', `${session}:${index}`))?.trim();
    if (n && n !== w?.name) await this.tmuxDo(host, 'rename-window', `${session}:${index}`, n);
  }

  // Run a tmux action on the tmux server of `host`.
  async tmuxDo(host, action, target, arg = '', id = '') {
    try {
      await this.conn.request({ t: 'tmux', host, name: action, target, data: arg, id });
    } catch (e) {
      if (/too old|bad request|doesn.t know/.test(e.message)) this.needsNewerGecko(host, 'control tmux');
      else this.toast(e.message);
    }
  }

  async attachTmux(host, target) {
    await this.newTab({ host, cwd: '', argv: ['tmux', 'attach', '-t', target] }); // titled by the tmux window
  }

  openAgent(a) {
    this.focusSession(a.session.id);
  }

  nextAgentNeedingYou() {
    const list = this.allAgents().filter((a) => a.state.status === 'needs-input');
    const target = list.find((a) => a.session.id !== this.activeId()) || list[0];
    if (target) this.focusSession(target.session.id);
    else this.toast('No agent is waiting for you.');
  }

  copySelection(p = this.activePane()) {
    if (!p || !p.term.hasSelection()) return false;
    copyText(p.term.getSelection()).then((ok) => this.toast(ok ? 'Copied' : 'Copy failed: the browser blocked clipboard access'));
    p.term.clearSelection();
    return true;
  }

  async paste() {
    const p = this.activePane();
    if (!p) return;
    const text = await readClipboard();
    if (text == null) {
      this.toast(isTouch ? 'Long-press the compose box to paste, then send.' : `Use ${isMac ? '⌘V' : 'Ctrl+Shift+V'} to paste.`);
      return;
    }
    p.term.paste(text);
    p.focus();
  }

  setFont(n) {
    this.setSetting(this.touch ? 'mobileFontSize' : 'fontSize', Math.max(8, Math.min(32, n)));
  }

  // Settings live in ~/.gecko-terminal/settings.json on the machine running
  // Gecko and sync to every window; localStorage only caches them so the
  // first paint already has the right colors.
  setSetting(k, v) {
    this.applySetting(k, v);
    this.pushSettings();
  }

  applySetting(k, v) {
    if (JSON.stringify(this.settings[k]) === JSON.stringify(v)) return;
    this.settings[k] = v;
    save(k, v);
    if (k === 'theme' || k === 'opacity') this.applyTheme();
    if (k === 'optionIsMeta') for (const p of this.panes.values()) p.term.options.macOptionIsMeta = v;
    if (k === 'fontSize' || k === 'mobileFontSize') for (const p of this.panes.values()) p.setFontSize(this.fontSize());
    if (k === 'sidebarHidden') {
      document.body.classList.toggle('side-hidden', !!v);
      this.activePane()?.layout(true);
    }
    if (k === 'keys') this.keys?.rebuild(v);
  }

  pushSettings() {
    const out = { ...(this.serverSettings || {}) };
    for (const k of SYNCED) out[k] = this.settings[k];
    this.serverSettings = out;
    this.conn.send({ t: 'setsettings', text: JSON.stringify(out) });
  }

  // Settings arrived from the service (on connect, or changed elsewhere).
  onSettings(text) {
    let obj;
    try { obj = JSON.parse(text || '{}'); } catch { return; }
    if (!Object.keys(obj).length && !this.serverSettings) {
      // First run with settings.json: keep what this browser had.
      this.serverSettings = {};
      return this.pushSettings();
    }
    if ('scheme' in obj) {
      // settings.json from before themes were called themes
      if (!('theme' in obj)) obj.theme = obj.scheme;
      delete obj.scheme;
      this.serverSettings = obj;
      this.pushSettings();
    }
    this.serverSettings = obj;
    for (const k of SYNCED) if (k in obj) this.applySetting(k, obj[k]);
    this.renderSoon();
  }

  fontSize() {
    return this.touch ? this.settings.mobileFontSize : this.settings.fontSize;
  }

  applyTheme(id = this.settings.theme, opacity = this.opacity()) {
    const theme = resolveTheme(id);
    this.termTheme = paintTheme(theme, opacity);
    this.setTermFont(theme.termFont || FONT);
    for (const p of this.panes?.values() || []) p.setTheme(this.termTheme);
  }

  // Keyboard shortcuts editor: every action with its keys; add, remove,
  // reset. Saved to settings.json as overrides of the defaults.
  openShortcuts() {
    const m = $('#modal');
    const overrides = () => ({ ...(this.settings.keys || {}) });
    const save2 = (o) => {
      // Keep only real changes, so new defaults still reach people later.
      for (const a of Object.keys(DEFAULTS)) {
        if (o[a] && JSON.stringify(o[a]) === JSON.stringify(DEFAULTS[a])) delete o[a];
      }
      if (o.tabNumbers === DEFAULT_TAB_MODIFIER) delete o.tabNumbers;
      this.setSetting('keys', o);
      render();
    };
    let capture = null; // { action } while waiting for keys
    let conflict = null; // { action, combo, owner }
    let filter = '';
    const chip = (action, c) => `<span class="keychip"><kbd>${esc(prettyCombo(c))}</kbd><button data-rm="${esc(action)}" data-combo="${esc(c)}" aria-label="Remove ${esc(prettyCombo(c))}" title="Remove">×</button></span>`;
    const render = () => {
      const b = this.keys.bindings;
      const o = this.settings.keys || {};
      const rows = Object.entries(ACTIONS)
        .filter(([, label]) => !filter || label.toLowerCase().includes(filter))
        .map(([a, label]) => {
          const changed = !!o[a];
          const keysHtml = capture?.action === a
            ? '<span class="capture">Press the new keys… (Esc cancels)</span>'
            : (b[a].length ? b[a].map((c) => chip(a, c)).join('') : '<span class="none">No shortcut</span>');
          const conf = conflict?.action === a
            ? `<div class="conflict">${esc(prettyCombo(conflict.combo))} already does “${esc(ACTIONS[conflict.owner] || conflict.owner)}”. <button data-take>Use it here instead</button> <button data-cancel-conflict>Cancel</button></div>`
            : '';
          return `<li class="${changed ? 'changed' : ''}"><span class="act">${esc(label)}</span><span class="keys">${keysHtml}</span>
            <span class="tools"><button data-add="${esc(a)}" title="Add a shortcut" aria-label="Add a shortcut for ${esc(label)}">+</button>${changed ? `<button data-reset="${esc(a)}" title="Back to the default">reset</button>` : ''}</span>${conf}</li>`;
        }).join('');
      const tm = this.keys.tabModifier;
      const tabRow = !filter || 'go to tab'.includes(filter)
        ? `<li class="${tm !== DEFAULT_TAB_MODIFIER ? 'changed' : ''}"><span class="act">Go to tab 1-9</span><span class="keys">${TAB_MODIFIERS.map((mod) => `<button class="mod ${mod === tm ? 'on' : ''}" data-tabmod="${mod}">${esc(prettyCombo(mod + '+1'))}…9</button>`).join('')}</span><span class="tools"></span></li>`
        : '';
      m.querySelector('ul').innerHTML = tabRow + rows || '<li class="none">No action matches.</li>';
    };
    m.innerHTML = `<div class="card shortcuts" role="dialog" aria-modal="true" aria-labelledby="sc-title">
      <h2 id="sc-title">Keyboard shortcuts</h2>
      <input type="search" placeholder="Filter actions" aria-label="Filter actions">
      <ul></ul>
      <div class="row"><span class="where">Saved in ~/.gecko-terminal/settings.json</span><button data-reset-all>Reset all</button><button class="primary" data-done>Done</button></div>
    </div>`;
    m.hidden = false;
    render();
    // Keys are caught on the window while the editor is open: re-rendering
    // the list moves focus out of the dialog.
    const onKey = (ev) => keydown(ev);
    window.addEventListener('keydown', onKey, true);
    const close = () => {
      window.removeEventListener('keydown', onKey, true);
      this.capturingKeys = false;
      m.hidden = true;
      m.innerHTML = '';
      this.activePane()?.focus();
    };
    const search = m.querySelector('input');
    search.addEventListener('input', () => { filter = search.value.trim().toLowerCase(); render(); });
    search.focus();
    m.onclick = (ev) => {
      const t = ev.target.closest('button');
      if (!t) return;
      const o = overrides();
      const d = t.dataset;
      if ('done' in d) return close();
      if ('resetAll' in d) return save2({});
      if (d.add) { capture = { action: d.add }; conflict = null; this.capturingKeys = true; render(); return; }
      if (d.reset) { delete o[d.reset]; return save2(o); }
      if (d.rm) { o[d.rm] = this.keys.bindings[d.rm].filter((c) => c !== d.combo); return save2(o); }
      if (d.tabmod) { o.tabNumbers = d.tabmod; return save2(o); }
      if ('take' in d && conflict) {
        o[conflict.owner] = this.keys.bindings[conflict.owner].filter((c) => c !== conflict.combo);
        o[conflict.action] = [...this.keys.bindings[conflict.action], conflict.combo];
        conflict = null;
        return save2(o);
      }
      if ('cancelConflict' in d) { conflict = null; render(); }
    };
    const keydown = (ev) => {
      if (!capture) {
        if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); close(); }
        return;
      }
      ev.preventDefault();
      ev.stopPropagation();
      if (ev.key === 'Escape') { capture = null; this.capturingKeys = false; render(); return; }
      if (['Control', 'Shift', 'Alt', 'Meta', 'AltGraph'].includes(ev.key)) return; // wait for the real key
      const combo = comboOf(ev);
      const action = capture.action;
      capture = null;
      this.capturingKeys = false;
      const owner = Object.keys(this.keys.bindings).find((a) => a !== action && this.keys.bindings[a].includes(combo));
      if (owner) { conflict = { action, combo, owner }; render(); return; }
      const o = overrides();
      if (!this.keys.bindings[action].includes(combo)) o[action] = [...this.keys.bindings[action], combo];
      save2(o);
    };
  }

  // Some themes bring their own terminal font. Terminals measure glyphs, so
  // switch once the font has loaded, then refit.
  async setTermFont(f) {
    if (this.termFontWanted === f) return;
    this.termFontWanted = f;
    const first = f.split(',')[0].trim();
    try { await document.fonts.load(`${this.fontSize()}px ${first}`); } catch {}
    if (this.termFontWanted !== f) return; // changed again meanwhile
    this.termFont = f; // new terminals use it from now on
    for (const p of this.panes?.values() || []) p.setFontFamily(f);
  }

  // Background opacity applies in the desktop app only.
  opacity() {
    return this.desktop?.transparent ? this.settings.opacity : 1;
  }

  // Opacity of the whole window over the desktop (desktop app only): the
  // background, picture themes' photo included. Text stays solid.
  openOpacityPicker() {
    const before = this.settings.opacity;
    const items = [1, 0.95, 0.9, 0.85, 0.8, 0.75, 0.7, 0.6, 0.5].map((o) => ({
      kind: Math.abs(o - before) < 0.001 ? 'Current' : 'Opacity',
      label: o === 1 ? 'Solid' : `${Math.round(o * 100)}%`,
      hint: o === 1 ? 'No transparency' : o >= 0.85 ? 'A hint of the desktop behind the window' : o >= 0.7 ? 'The desktop clearly shows through' : 'Very see-through',
      value: o,
      run: () => this.setSetting('opacity', o),
    }));
    this.openPalette('', {
      placeholder: 'Window opacity over the desktop',
      items,
      select: items.findIndex((i) => Math.abs(i.value - before) < 0.001),
      onSelect: (item) => item && this.applyTheme(this.settings.theme, item.value),
      onCancel: () => this.applyTheme(),
    });
  }

  // Pick a theme; the app previews each one as you move through
  // the list. Enter keeps it, Esc goes back.
  openThemePicker() {
    const before = this.settings.theme;
    const items = [{ id: AUTO, name: 'Gecko', hint: 'Dark or light, following your system', s: resolveTheme(AUTO) }]
      .concat(THEMES.map((s) => ({ id: s.id, name: s.name, hint: (s.dark ? 'Dark' : 'Light') + (s.image ? ', with a picture' : ''), s })))
      .map((x) => ({
        kind: x.id === before ? 'Current' : 'Theme',
        label: x.name,
        hint: x.hint,
        glyph: swatch(x.s),
        theme: x.id,
        run: () => this.setSetting('theme', x.id),
      }));
    this.openPalette('', {
      placeholder: 'Theme',
      items,
      select: items.findIndex((i) => i.theme === before),
      onSelect: (item) => item && this.applyTheme(item.theme),
      onCancel: () => this.applyTheme(before),
    });
  }

  action(name) {
    switch (name) {
      case 'palette': return this.openPalette();
      case 'toggleSidebar': return this.toggleSidebar();
      case 'fullscreen': return this.toggleFullscreen();
      case 'newTab': return this.newTab();
      case 'closeTab': return this.closeTab();
      case 'nextTab': return this.cycleTab(1);
      case 'prevTab': return this.cycleTab(-1);
      case 'nextWs': return this.cycleWorkspace(1);
      case 'prevWs': return this.cycleWorkspace(-1);
      case 'copy': return this.copySelection() ? undefined : false;
      case 'find': return this.openFind();
      case 'selectText': return this.openTextView();
      case 'nextAgent': return this.nextAgentNeedingYou();
      case 'fontUp': return this.setFont(this.fontSize() + 1);
      case 'fontDown': return this.setFont(this.fontSize() - 1);
      case 'fontReset': return this.setFont(this.touch ? 12 : 13);
      case 'rename': return this.renameTab();
      case 'help': return this.openShortcuts();
    }
    return false;
  }

  async renameTab(id = this.activeId()) {
    const s = this.sessions.get(id);
    if (!s) return;
    const name = await this.prompt('Rename tab', tabLabel(s), 'Leave empty to name it after what runs in it');
    if (name == null) return;
    this.conn.send({ t: 'patch', id, patch: { name } });
  }

  async moveTab(id = this.activeId()) {
    const s = this.sessions.get(id);
    if (!s) return;
    const ws = await this.prompt('Move tab to workspace', '', `Existing: ${this.workspaces().join(', ')}`);
    if (!ws) return;
    this.conn.send({ t: 'patch', id, patch: { workspace: ws } });
    this.activeByWs[ws] = id;
    this.switchWorkspace(ws);
  }

  async newWorkspace() {
    const ws = await this.prompt('New workspace', '', 'A workspace groups tabs for one project, across machines');
    if (!ws) return;
    await this.newTab({ workspace: ws, cwd: '' });
  }

  // Browse folders on a tab's machine and make one its working folder.
  async openFolderPicker(id, dir = '') {
    const s = this.sessions.get(id);
    if (!s) return;
    let r;
    try {
      r = await this.conn.request({ t: 'listdir', id, dir });
    } catch (e) {
      if (/too old|bad request|doesn.t know/.test(e.message)) return this.needsNewerGecko(s.host, 'browse folders');
      return this.toast(e.message);
    }
    const here = r.dir, home = r.text || '';
    const pretty = (p) => (home && (p === home || p.startsWith(home + '/')) ? '~' + p.slice(home.length) : p);
    const join = (a, b) => (a.endsWith('/') ? a + b : a + '/' + b);
    const parent = here === '/' ? null : here.replace(/\/[^/]+\/?$/, '') || '/';
    const go = (p) => this.openFolderPicker(id, p);
    const use = (p) => this.useFolder(id, p);
    const top = [{ kind: 'Use', label: 'Use this folder', hint: pretty(here), run: () => use(here) }];
    if (parent) top.push({ kind: 'Up', label: '..', hint: pretty(parent), keepOpen: true, run: () => go(parent), commit: () => use(parent) });
    const folders = (r.entries || []).map((e) => {
      const p = join(here, e.name);
      return {
        kind: e.git ? 'Repo' : 'Folder', label: e.name + '/', hint: e.git ? 'git repository' : '',
        glyph: e.git ? BRANCH_ICON : '', keepOpen: true, run: () => go(p), commit: () => use(p),
      };
    });
    if (!folders.length) top.push({ kind: '', label: 'No folders here', hint: '', run: () => {} });
    this.openPalette('', {
      placeholder: `${pretty(here)} on ${s.host}: Enter opens a folder, Ctrl+Enter uses it, or type a path`,
      items: (q) => {
        if (/^[~/]/.test(q)) {
          // A typed path: open it, or use it right away with Ctrl+Enter.
          const resolveThen = async (f) => {
            try {
              const t = await this.conn.request({ t: 'listdir', id, dir: q });
              f(t.dir);
            } catch (e) {
              this.toast(e.message);
            }
          };
          return [{ kind: 'Go', label: `Go to ${q}`, hint: 'Enter opens it, Ctrl+Enter uses it', keepOpen: true, run: () => go(q), commit: () => resolveThen(use) }];
        }
        if (!q) return [...top, ...folders];
        return folders
          .map((i) => ({ i, s: fuzzy(q, i.label) }))
          .filter((x) => x.s >= 0)
          .sort((a, b) => b.s - a.s)
          .map((x) => x.i);
      },
    });
  }

  // A feature needs a newer Gecko on some machine: say which, and offer
  // the fix (update that machine, or restart this one).
  needsNewerGecko(host, what) {
    if (host && host !== this.hostName) {
      this.toast(`\`${host}\` runs an older Gecko that can't ${what}. Click to update it there (its tabs keep running).`, () => this.updateHost(host));
    } else {
      this.toast(`This Gecko is too old to ${what}. Click to restart it with the installed version.`, () => this.restartService());
    }
  }

  // Copy this Gecko to a machine and restart it there in place.
  async updateHost(name) {
    const m = $('#modal');
    m.innerHTML = `<div class="card setup" role="dialog" aria-modal="true" aria-labelledby="upd-title">
      <h2 id="upd-title">Update Gecko on ${esc(name)}</h2>
      <p class="sub">Copies this version of Gecko there and restarts it in place. Tabs on ${esc(name)} keep running.</p>
      <div class="setup-status" role="status">Working…</div>
      <pre class="setup-log"></pre>
      <div class="row"><button class="primary" data-close disabled>Close</button></div>
    </div>`;
    m.hidden = false;
    const job = 'update' + Date.now();
    const log = m.querySelector('.setup-log');
    const status = m.querySelector('.setup-status');
    const close = m.querySelector('[data-close]');
    const done = () => { this.hostSetup = null; m.hidden = true; m.innerHTML = ''; this.activePane()?.focus(); };
    close.onclick = done;
    m.onkeydown = (ev) => { if (ev.key === 'Escape' && !close.disabled) done(); };
    this.hostSetup = { log: (sub, text) => { if (sub === job) { log.textContent += text + '\n'; log.scrollTop = log.scrollHeight; } } };
    try {
      await this.conn.request({ t: 'updatehost', name, sub: job });
      status.textContent = `${name} is up to date.`;
      status.className = 'setup-status good';
    } catch (e) {
      status.textContent = /too old|bad request|doesn.t know/.test(e.message) ? 'This Gecko is too old to update other machines. Restart Gecko (Ctrl+K) first.' : e.message;
      status.className = 'setup-status bad';
    }
    close.disabled = false;
    close.focus();
  }

  // Make path the tab's working folder: `cd` at a shell prompt, otherwise
  // (something is running) a new tab in that folder.
  useFolder(id, path) {
    const s = this.sessions.get(id);
    if (!s) return;
    if (s.kind === 'shell' && !s.running) {
      const q = "'" + path.replace(/'/g, "'\\''") + "'";
      this.focusSession(id);
      this.conn.input(id, '\x05\x15cd ' + q + '\r'); // Ctrl+E Ctrl+U: clear anything half-typed first
    } else {
      this.newTab({ host: s.host, cwd: path, workspace: s.workspace });
      this.toast(`Something is running in \`${tabLabel(s)}\`, so \`${path}\` opened in a new tab.`);
    }
  }

  // The Gecko machine an ssh destination refers to, if one is set up.
  hostForTarget(target) {
    if (!target) return null;
    const norm = (t) => (t || '').replace(/^ssh:\/\//, '').replace(/\/$/, '').replace(/:22$/, '').toLowerCase();
    const want = norm(target);
    const bare = (t) => norm(t).replace(/^[^@]*@/, '');
    return this.hosts.find((h) => !h.local && h.target && (norm(h.target) === want || (bare(h.target) === bare(want) && !want.includes('@'))))
      || this.hosts.find((h) => !h.local && h.name === bare(want).split(/[.:]/)[0]) || null;
  }

  // Add (or fix) a machine: check the SSH connection with live output,
  // offer to install gecko there, and only then add it.
  addHost(prefill = {}) {
    const guessName = (t) => t.replace(/^ssh:\/\//, '').replace(/^.*@/, '').replace(/^\[|\]?(:\d+)?\/?$/g, '').split('.')[0].toLowerCase();
    const m = $('#modal');
    m.innerHTML = `<div class="card setup" role="dialog" aria-modal="true" aria-labelledby="setup-title">
      <h2 id="setup-title">${prefill.name ? `Fix ${esc(prefill.name)}` : 'Add a machine'}</h2>
      <p class="sub">Gecko connects with your SSH keys and runs its own background service there, so sessions keep running when you disconnect.</p>
      <label>SSH destination<input name="target" placeholder="me@devbox or me@devbox:2222" autocomplete="off" autocapitalize="off" spellcheck="false"></label>
      <label>Name in Gecko<input name="name" placeholder="devbox" autocomplete="off" autocapitalize="off" spellcheck="false"></label>
      <details><summary>Advanced</summary>
        <label>Path to gecko on that machine<input name="gecko" placeholder="found on PATH or in ~/.local/bin" autocomplete="off" spellcheck="false"></label>
      </details>
      <div class="setup-status" role="status"></div>
      <pre class="setup-log" hidden></pre>
      <div class="row">
        <button data-cancel>Cancel</button>
        <button data-recheck hidden>Check again</button>
        <button class="primary" data-main>Check connection</button>
      </div>
    </div>`;
    m.hidden = false;
    const $m = (sel) => m.querySelector(sel);
    const f = { target: $m('[name=target]'), name: $m('[name=name]'), gecko: $m('[name=gecko]') };
    f.target.value = prefill.target || '';
    f.name.value = prefill.name || (prefill.target ? guessName(prefill.target) : '');
    f.gecko.value = prefill.gecko || '';
    let nameEdited = !!prefill.name;
    let state = 'idle'; // idle | busy | ok | missing | failed
    let found = null;
    const job = 'setup' + Date.now();
    const log = $m('.setup-log');
    const status = $m('.setup-status');
    const main = $m('[data-main]');
    const recheck = $m('[data-recheck]');

    const setState = (st, msg = '', kind = '') => {
      state = st;
      status.textContent = msg;
      status.className = 'setup-status ' + kind;
      const busy = st === 'busy';
      for (const el of [main, recheck, f.target, f.name, f.gecko]) el.disabled = busy;
      main.textContent = { idle: 'Check connection', busy: 'Working…', ok: 'Add machine', missing: 'Install Gecko there', failed: 'Retry' }[st];
      recheck.hidden = !(st === 'ok' || st === 'missing');
    };
    const close = () => {
      this.hostSetup = null;
      m.hidden = true;
      m.innerHTML = '';
      this.activePane()?.focus();
    };
    this.hostSetup = {
      log: (sub, text) => {
        if (sub !== job) return;
        log.hidden = false;
        log.textContent += text + '\n';
        log.scrollTop = log.scrollHeight;
      },
    };
    const check = async (keepLog = false) => {
      const target = f.target.value.trim();
      if (!target) return setState('failed', 'Enter the SSH destination, for example me@devbox.', 'bad');
      if (keepLog !== true) log.textContent = '';
      setState('busy', 'Connecting…');
      try {
        const r = await this.conn.request({ t: 'probehost', sub: job, target, data: f.gecko.value.trim() });
        if (!this.hostSetup) return;
        found = r;
        if (r.missing) setState('missing', `Reachable (${r.os}), but Gecko isn't installed there. Gecko can copy itself over to ~/.local/bin.`, 'warn');
        else setState('ok', `Connected: ${r.os}, ${r.text}.`, 'good');
      } catch (e) {
        if (this.hostSetup) setState('failed', e.message, 'bad');
      }
    };
    const install = async () => {
      setState('busy', 'Installing Gecko…');
      try {
        await this.conn.request({ t: 'installhost', sub: job, target: f.target.value.trim(), os: found.os });
        if (this.hostSetup) await check(true);
      } catch (e) {
        if (this.hostSetup) setState('missing', e.message, 'bad');
      }
    };
    const add = async () => {
      const name = f.name.value.trim();
      if (!name || /[\s/]/.test(name)) return setState('ok', 'Give the machine a name without spaces or slashes.', 'bad');
      setState('busy', 'Adding…');
      try {
        await this.conn.request({ t: 'addhost', name, target: f.target.value.trim(), data: f.gecko.value.trim() });
        close();
        this.toast(`Added \`${name}\`. Connecting…`);
      } catch (e) {
        setState('ok', e.message, 'bad');
      }
    };
    const primary = () => {
      if (state === 'busy') return;
      if (state === 'ok') return add();
      if (state === 'missing') return install();
      return check();
    };
    main.onclick = primary;
    recheck.onclick = () => check();
    $m('[data-cancel]').onclick = close;
    f.target.addEventListener('input', () => {
      if (!nameEdited) f.name.value = guessName(f.target.value);
      if (state !== 'busy') setState('idle');
    });
    f.gecko.addEventListener('input', () => state !== 'busy' && setState('idle'));
    f.name.addEventListener('input', () => { nameEdited = true; });
    m.onkeydown = (ev) => {
      if (ev.key === 'Escape') { ev.preventDefault(); close(); }
      if (ev.key === 'Enter' && ev.target.tagName === 'INPUT') { ev.preventDefault(); primary(); }
    };
    setState('idle');
    if (prefill.error) setState('failed', prefill.error, 'bad');
    (f.target.value ? main : f.target).focus();
    if (prefill.check) check();
  }

  async removeHost(name) {
    if (!(await this.confirm(`Remove \`${name}\` from Gecko? Sessions running there keep running, and you can add it back any time.`, 'Remove'))) return;
    try { await this.conn.request({ t: 'rmhost', name }); } catch (e) { this.toast(e.message); }
  }

  async openTemplate(name) {
    const p = this.activePane();
    try {
      const r = await this.conn.request({ t: 'openws', name, cols: p?.term.cols || 80, rows: p?.term.rows || 24 });
      if (r.sessions?.length) {
        for (const s of r.sessions) this.sessions.set(s.id, s);
        this.focusSession(r.sessions[0].id);
      }
    } catch (e) {
      this.toast(e.message);
    }
  }

  async enableNotifications() {
    if (!('Notification' in window)) return this.toast('This browser has no notifications.');
    const r = await Notification.requestPermission();
    this.toast(r === 'granted' ? 'Gecko will notify you when an agent needs you.' : 'Notifications are blocked for this site.');
  }

  // ---------------------------------------------------------------- UI wiring

  bindUI() {
    $('#new-tab').onclick = () => this.newTab();
    $('#menu-btn').onclick = () => this.toggleSidebar();
    $('#menu-btn').title = `Show or hide workspaces (${prettyCombo(this.keys.first('toggleSidebar'))})`;
    document.body.classList.toggle('side-hidden', !!this.settings.sidebarHidden);
    $('#scrim').onclick = () => this.closeDrawer();
    $('#palette-btn').onclick = () => this.openPalette();
    $('#conn').onclick = () => { if (this.outdated) this.restartService(); };
    $('#palette-btn kbd').textContent = prettyCombo(this.keys.first('palette'));
    $('#tabs').addEventListener('click', (ev) => {
      const close = ev.target.closest('[data-close]');
      if (close) return this.closeTab(close.dataset.close);
      const t = ev.target.closest('[data-tab]');
      if (!t) return;
      if (this.isDouble('tab:' + t.dataset.tab)) return this.renameTab(t.dataset.tab);
      this.focusSession(t.dataset.tab);
    });
    $('#tabs').addEventListener('auxclick', (ev) => {
      const t = ev.target.closest('[data-tab]');
      if (t && ev.button === 1) this.closeTab(t.dataset.tab);
    });
    $('#side').addEventListener('click', (ev) => {
      const el = ev.target.closest('[data-act]');
      if (!el) return;
      const d = el.dataset;
      switch (d.act) {
        case 'ws':
          // Click folds/unfolds; double-click renames (and undoes the fold).
          if (this.isDouble('ws:' + d.ws)) {
            this.toggleFold(d.ws); // undo the first click's fold
            return this.renameWorkspace(d.ws);
          }
          return this.toggleFold(d.ws);
        case 'tab':
          if (this.isDouble('tab:' + d.id)) return this.renameTab(d.id);
          return this.focusSession(d.id);
        case 'tab-close':
          return this.closeTab(d.id);
        case 'ws-close':
          return this.closeWorkspace(d.ws);
        case 'template': return this.openTemplate(d.name);
        case 'host-new': return this.newTab({ host: d.host, cwd: '' });
        case 'adopt-ssh': return this.addHost({ target: d.target, check: true });
        case 'host-rm': return this.removeHost(d.host);
        case 'host-update': return this.updateHost(d.host);
        case 'host-fix': {
          const h = this.hosts.find((x) => x.name === d.host);
          return this.addHost({ name: d.host, target: h?.target, error: h?.error });
        }
        case 'tmux':
          return this.clickOrDouble(`tmux:${d.host}/${d.name}`, () => this.openTmux(d.host, d.name), () => this.renameTmuxSession(d.host, d.name));
        case 'tmux-win':
          return this.clickOrDouble(`tmuxw:${d.host}/${d.name}:${d.idx}`, () => this.openTmux(d.host, d.name, +d.idx), () => this.renameTmuxWindow(d.host, d.name, +d.idx));
        case 'tmux-fold': {
          const k = d.host + '/' + d.name;
          this.tmuxOpen[k] = !this.tmuxOpen[k];
          save('tmuxOpen', this.tmuxOpen);
          return this.renderSidebar();
        }
        case 'add-host': return this.addHost();
        case 'new-ws': return this.newWorkspace();
      }
    });
    $('#side').addEventListener('auxclick', (ev) => {
      if (ev.button !== 1) return;
      const row = ev.target.closest('.tab-row');
      if (row) return this.closeTab(row.dataset.id);
      const ws = ev.target.closest('.ws-row [data-ws]');
      if (ws) this.closeWorkspace(ws.dataset.ws);
    });
    $('#statusline').addEventListener('click', (ev) => {
      const el = ev.target.closest('[data-act]');
      if (!el) return;
      if (el.dataset.act === 'adopt-ssh') this.addHost({ target: el.dataset.target, check: true });
      if (el.dataset.act === 'pick-cwd') this.openFolderPicker(this.activeId());
      const cur = this.activeSession();
      if (el.dataset.act === 'tmux-sel' && cur?.tmuxSession) {
        const idx = +el.dataset.idx;
        this.clickOrDouble(`chip:${cur.host}/${cur.tmuxSession}:${idx}`,
          () => this.tmuxDo(cur.host, 'select-window', `${cur.tmuxSession}:${idx}`),
          () => this.renameTmuxWindow(cur.host, cur.tmuxSession, idx));
      }
      if (el.dataset.act === 'tmux-rename' && cur?.tmuxSession) {
        this.clickOrDouble(`sess:${cur.host}/${cur.tmuxSession}`, () => {}, () => this.renameTmuxSession(cur.host, cur.tmuxSession));
      }
      if (el.dataset.act === 'tmux-new' && cur?.tmuxSession) this.tmuxDo(cur.host, 'new-window', cur.tmuxSession);
      if (el.dataset.act === 'host-new') this.newTab({ host: el.dataset.host, cwd: '' });
    });
    $('#dock').addEventListener('click', (ev) => {
      const reply = ev.target.closest('[data-reply]');
      if (reply) {
        const id = reply.dataset.id;
        const keys = { enter: '\r', esc: '\x1b', up: '\x1b[A', down: '\x1b[B' };
        this.conn.input(id, keys[reply.dataset.reply] ?? reply.dataset.reply);
        return;
      }
      const chip = ev.target.closest('[data-agent]');
      if (chip) {
        const a = this.allAgents().find((x) => x.key === chip.dataset.agent);
        if (a) this.openAgent(a);
      }
    });
    $('#empty').addEventListener('click', (ev) => {
      const el = ev.target.closest('[data-act]');
      if (!el) return;
      if (el.dataset.act === 'host-new') this.newTab({ host: el.dataset.host, cwd: '' });
      if (el.dataset.act === 'template') this.openTemplate(el.dataset.name);
    });
    this.keepTerminalFocus();
    this.bindKeybar();
    this.bindCompose();
    this.bindPalette();
    this.bindTextView();
    this.bindFind();
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (this.settings.theme === AUTO) this.applyTheme();
    });
    window.addEventListener('focus', () => {
      const p = this.activePane();
      if (p) p.sendSize(true);
    });
    document.addEventListener('visibilitychange', () => this.updateTitle());
  }

  // Double-click detection that survives the sidebar re-rendering between
  // the two clicks (which swallows native dblclick events).
  isDouble(key) {
    const now = Date.now();
    const dbl = this.lastClick && this.lastClick.key === key && now - this.lastClick.at < 400;
    this.lastClick = dbl ? null : { key, at: now };
    return dbl;
  }

  toggleFold(ws) {
    this.collapsed[ws] = !this.collapsed[ws];
    save('collapsed', this.collapsed);
    this.renderSidebar();
  }

  // Close a workspace: all its tabs, after one confirmation.
  async closeWorkspace(ws) {
    const tabs = this.tabsOf(ws);
    if (tabs.length) {
      const busy = tabs.filter((t) => !t.exited && (t.agent || (t.kind && t.kind !== 'shell') || t.running))
        .map((t) => '`' + (t.agent ? t.agent.name : t.command || t.proc || t.kind) + '`' + (t.agent ? ' (agent)' : ''));
      const what = busy.length ? ` This ends ${busy.join(', ')}.` : '';
      const n = tabs.length === 1 ? '1 tab' : `${tabs.length} tabs`;
      if (!(await this.confirm(`Close \`${ws}\` and its ${n}?${what}`, 'Close workspace'))) return;
    }
    for (const t of tabs) this.closeTab(t.id, true);
    delete this.activeByWs[ws];
    delete this.collapsed[ws];
    save('activeByWs', this.activeByWs);
    save('collapsed', this.collapsed);
    if (this.ws === ws) {
      const next = this.workspaces().find((w) => w !== ws && this.tabsOf(w).length);
      this.switchWorkspace(next || 'default');
    } else {
      this.render();
    }
  }

  async renameWorkspace(ws) {
    const name = (await this.prompt('Rename workspace', ws, 'Using the name of another workspace merges the two'))?.trim();
    if (!name || name === ws) return;
    for (const s of this.sessions.values()) {
      if (s.workspace === ws) this.conn.send({ t: 'patch', id: s.id, patch: { workspace: name } });
    }
    for (const s of this.exited.values()) if (s.workspace === ws) s.workspace = name;
    if (this.activeByWs[ws]) this.activeByWs[name] = this.activeByWs[ws];
    delete this.activeByWs[ws];
    if (this.collapsed[ws] !== undefined) this.collapsed[name] = this.collapsed[ws];
    delete this.collapsed[ws];
    save('activeByWs', this.activeByWs);
    save('collapsed', this.collapsed);
    if (this.ws === ws) this.switchWorkspace(name);
  }

  // The shell is what has the keyboard. Clicking chrome (sidebar, tabs,
  // dock, margins) must not take focus away from it; only text fields and
  // overlays (palette, prompts, find, compose, text view) get focus.
  keepTerminalFocus() {
    const ownsFocus = 'input, textarea, select, [contenteditable], #palette, #modal, #textview, #find, #compose';
    document.addEventListener('mousedown', (ev) => {
      if (this.touch || ev.button !== 0) return;
      const t = ev.target;
      if (t.closest?.('.xterm') || t.closest?.(ownsFocus)) return;
      // Leave scrollbar drags alone.
      if (t.clientWidth && ev.offsetX > t.clientWidth) return;
      ev.preventDefault(); // keeps focus where it is
      if (!document.activeElement?.closest?.('.xterm') && !this.paletteOpen && !this.textViewOpen) {
        requestAnimationFrame(() => this.activePane()?.focus());
      }
    }, true);
    // Coming back to the window lands in the shell too.
    window.addEventListener('focus', () => {
      if (!this.touch && !this.paletteOpen && !this.textViewOpen && !document.activeElement?.closest?.(ownsFocus)) {
        this.activePane()?.focus();
      }
    });
  }

  // Space the terminal gets and its cell size, computed exactly like
  // xterm's fit addon (parent size minus padding and a 14px scrollbar).
  terminalArea() {
    const p = this.activePane();
    const dims = p?.term._core?._renderService?.dimensions?.css?.cell;
    if (!p || !dims?.width) return this.estimatedArea();
    const parent = getComputedStyle(p.el);
    const own = getComputedStyle(p.term.element);
    const padX = parseInt(own.paddingLeft) + parseInt(own.paddingRight);
    const padY = parseInt(own.paddingTop) + parseInt(own.paddingBottom);
    return {
      cw: dims.width, ch: dims.height,
      w: parseInt(parent.width) - padX - 14,
      h: parseInt(parent.height) - padY,
    };
  }

  // No terminal open yet: measure with a hidden terminal laid out exactly
  // like a real tab, then throw it away.
  estimatedArea() {
    const el = document.createElement('div');
    el.className = 'pane';
    el.style.visibility = 'hidden';
    $('#panes').appendChild(el);
    const t = new Terminal({ fontFamily: this.termFont || FONT, fontSize: this.fontSize(), lineHeight: 1.15, scrollback: 1 });
    try {
      t.open(el);
      // Same renderer as real tabs: WebGL snaps cells to whole pixels.
      if (this.settings.renderer === 'webgl' || (this.settings.renderer === 'auto' && !this.touch)) {
        try { t.loadAddon(new WebglAddon()); } catch {}
      }
      const cell = t._core?._renderService?.dimensions?.css?.cell;
      if (!cell?.width) return null;
      const parent = getComputedStyle(el);
      // The status line is hidden while there are no tabs and takes its
      // place once the first one opens.
      const statusline = $('#statusline').offsetHeight ? 0 : 24;
      return { cw: cell.width, ch: cell.height, w: parseInt(parent.width) - 14, h: parseInt(parent.height) - statusline };
    } finally {
      t.dispose();
      el.remove();
    }
  }

  // Resize this window so the terminal is exactly cols x rows, then center
  // it. Works in app windows (gecko open, installed app); browsers ignore
  // it for normal tabs, which is fine.
  async fitWindow(cols, rows) {
    const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
    for (let i = 0; i < 6; i++) {
      const a = this.terminalArea();
      if (!a) break;
      // Aim for the middle of a cell so rounding lands on exactly cols x rows.
      const dw = Math.round((cols + 0.5) * a.cw - a.w);
      const dh = Math.round((rows + 0.5) * a.ch - a.h);
      if (Math.abs(dw) < a.cw / 2 && Math.abs(dh) < a.ch / 2) break;
      const before = [window.outerWidth, window.outerHeight];
      if (this.desktop) await this.desktop.setBounds({ width: window.outerWidth + dw, height: window.outerHeight + dh });
      else window.resizeTo(window.outerWidth + dw, window.outerHeight + dh);
      await sleep(150);
      if (window.outerWidth === before[0] && window.outerHeight === before[1]) break; // not allowed here
    }
    const x = Math.round((screen.availLeft || 0) + Math.max(0, (screen.availWidth - window.outerWidth) / 2));
    const y = Math.round((screen.availTop || 0) + Math.max(0, (screen.availHeight - window.outerHeight) / 2));
    if (Math.abs(window.screenX - x) > 1 || Math.abs(window.screenY - y) > 1) {
      if (this.desktop) await this.desktop.setBounds({ width: window.outerWidth, height: window.outerHeight, x, y });
      else window.moveTo(x, y);
    }
    this.activePane()?.layout(true);
    // Tell gecko where this window belongs, so the next `gecko open`
    // starts there instead of moving into place.
    const a = this.terminalArea();
    if (a && Math.floor(a.w / a.cw) === cols && Math.floor(a.h / a.ch) === rows) {
      this.conn.send({ t: 'window', text: JSON.stringify({
        w: window.outerWidth, h: window.outerHeight,
        sx: screen.availLeft || 0, sy: screen.availTop || 0, sw: screen.availWidth, sh: screen.availHeight,
      }) });
    }
  }

  // Restart Gecko: the background service restarts in place with the
  // installed binary (tabs keep running), then this window reloads into the
  // new version once the service is back.
  async restartService() {
    const reloadSoon = (ms) => setTimeout(() => location.reload(), ms);
    try {
      this.restarting = true;
      await this.conn.request({ t: 'upgrade' });
      this.toast('Restarting Gecko. Your tabs keep running.');
      reloadSoon(15000); // in case the reconnect is never noticed
    } catch (e) {
      this.restarting = false;
      const old = /too old|bad request|doesn.t know|can.t restart/.test(e.message);
      this.toast(old
        ? 'Reloading this window. The background service can\'t restart itself here; run gecko stop && gecko to update it (closes its tabs).'
        : e.message);
      if (old) reloadSoon(2500);
    }
  }

  // Desktop: hide or show the sidebar (remembered). Phones: open the drawer.
  toggleSidebar() {
    if (matchMedia('(max-width: 760px)').matches) {
      document.body.classList.toggle('drawer');
      return;
    }
    const hidden = document.body.classList.toggle('side-hidden');
    this.setSetting('sidebarHidden', hidden);
    this.activePane()?.layout(true);
  }

  // Fullscreen. Where the browser supports it, the keyboard is captured too,
  // so Esc (vim!) and shortcuts like Ctrl+W reach the terminal; holding Esc
  // leaves fullscreen.
  async toggleFullscreen() {
    if (document.fullscreenElement) {
      navigator.keyboard?.unlock?.();
      return document.exitFullscreen();
    }
    try {
      await document.documentElement.requestFullscreen({ navigationUI: 'hide' });
    } catch {
      return this.toast('This browser does not allow fullscreen here.');
    }
    let locked = false;
    try {
      await navigator.keyboard.lock();
      locked = true;
    } catch {}
    const leave = prettyCombo(this.keys.first('fullscreen'));
    this.toast(locked ? `Fullscreen. Hold Esc or press ${leave} to leave.` : `Fullscreen. Press ${leave} to leave.`);
  }

  closeDrawer() {
    document.body.classList.remove('drawer');
  }

  setupTouch(p) {
    if (!this.touch) return;
    // Long-press anywhere on the terminal opens the selectable text view.
    let timer = null, x = 0, y = 0;
    p.el.addEventListener('touchstart', (ev) => {
      const t = ev.touches[0];
      x = t.clientX; y = t.clientY;
      clearTimeout(timer);
      timer = setTimeout(() => this.openTextView(), 550);
    }, { passive: true });
    p.el.addEventListener('touchmove', (ev) => {
      const t = ev.touches[0];
      if (Math.abs(t.clientX - x) + Math.abs(t.clientY - y) > 10) clearTimeout(timer);
    }, { passive: true });
    p.el.addEventListener('touchend', () => clearTimeout(timer));
  }

  bindKeybar() {
    const seq = {
      esc: '\x1b', tab: '\t', up: '\x1b[A', down: '\x1b[B', left: '\x1b[D', right: '\x1b[C',
      pgup: '\x1b[5~', pgdn: '\x1b[6~', home: '\x1b[H', end: '\x1b[F', 'ctrl-c': '\x03', 'ctrl-d': '\x04',
    };
    const bar = $('#keybar');
    bar.addEventListener('pointerdown', (ev) => {
      if (ev.target.closest('button')) ev.preventDefault(); // keep the keyboard up
    });
    bar.addEventListener('click', (ev) => {
      const b = ev.target.closest('button');
      if (!b) return;
      const k = b.dataset.k;
      const id = this.activeId();
      if (k === 'ctrl' || k === 'alt') {
        this.mods[k] = !this.mods[k];
        this.renderKeybar();
        this.activePane()?.focus();
        return;
      }
      if (k === 'paste') return this.paste();
      if (k === 'select') return this.openTextView();
      if (k === 'compose') return this.toggleCompose();
      if (!id) return;
      const data = seq[k] ?? b.dataset.text ?? b.textContent;
      this.sendInput(id, data);
    });
  }

  renderKeybar() {
    for (const b of document.querySelectorAll('#keybar [data-k=ctrl], #keybar [data-k=alt]')) {
      b.classList.toggle('on', !!this.mods[b.dataset.k]);
    }
  }

  bindCompose() {
    const ta = $('#compose textarea');
    const send = () => {
      const id = this.activeId();
      if (!id || !ta.value) return;
      let text = ta.value;
      // Multi-line prompts go in as one bracketed paste so agents don't
      // submit each line separately.
      if (text.includes('\n')) text = `\x1b[200~${text}\x1b[201~`;
      this.conn.input(id, text);
      setTimeout(() => this.conn.input(id, '\r'), 30);
      ta.value = '';
      ta.style.height = '';
    };
    $('#compose [data-send]').onclick = send;
    ta.addEventListener('keydown', (ev) => {
      if (ev.key === 'Enter' && !ev.shiftKey && !ev.isComposing) {
        ev.preventDefault();
        send();
      }
      if (ev.key === 'Escape') this.toggleCompose(false);
    });
    ta.addEventListener('input', () => {
      ta.style.height = '';
      ta.style.height = Math.min(ta.scrollHeight, 160) + 'px';
    });
  }

  toggleCompose(on) {
    const c = $('#compose');
    c.hidden = on === undefined ? !c.hidden : !on;
    if (!c.hidden) $('#compose textarea').focus();
    else this.activePane()?.focus();
    requestAnimationFrame(() => this.activePane()?.layout(true));
  }

  // ---------------------------------------------------------------- palette

  bindPalette() {
    const input = $('#palette input');
    input.addEventListener('input', () => this.renderPalette());
    input.addEventListener('keydown', (ev) => {
      const items = this.paletteItems || [];
      if (ev.key === 'ArrowDown' || (ev.key === 'n' && ev.ctrlKey)) {
        this.paletteSel = Math.min(this.paletteSel + 1, items.length - 1);
        this.renderPaletteSel();
        ev.preventDefault();
      } else if (ev.key === 'ArrowUp' || (ev.key === 'p' && ev.ctrlKey)) {
        this.paletteSel = Math.max(this.paletteSel - 1, 0);
        this.renderPaletteSel();
        ev.preventDefault();
      } else if ((ev.key === 'PageDown' || ev.key === 'PageUp') && items.length) {
        // Move by one visible page of results.
        const ul = $('#palette ul');
        const li = ul.querySelector('li[data-i]');
        const page = Math.max(1, Math.floor(ul.clientHeight / (li?.offsetHeight || 34)) - 1);
        const d = ev.key === 'PageDown' ? page : -page;
        this.paletteSel = Math.max(0, Math.min(items.length - 1, this.paletteSel + d));
        this.renderPaletteSel();
        ev.preventDefault();
      } else if (ev.key === 'Enter') {
        ev.preventDefault();
        if (this.promptState) return this.finishPrompt(input.value);
        const item = items[this.paletteSel];
        // Ctrl/Cmd+Enter: an item's alternate action (folder browser: use it).
        if ((ev.ctrlKey || ev.metaKey) && item?.commit) return this.choose({ run: item.commit });
        this.choose(item);
      } else if (ev.key === 'Escape') {
        ev.preventDefault();
        this.closePalette();
      }
    });
    $('#palette').addEventListener('mousedown', (ev) => {
      if (ev.target.id === 'palette') this.closePalette();
    });
    $('#palette ul').addEventListener('click', (ev) => {
      const li = ev.target.closest('li[data-i]');
      if (li) this.choose(this.paletteItems[+li.dataset.i]);
    });
  }

  commands() {
    const cur = this.activeSession();
    const items = [];
    const add = (label, hint, run, kind = 'Command', combo, keywords = '') => items.push({ label, hint, run, kind, combo, keywords });
    const nums = this.tabNumbers();
    for (const s of this.allTabs().filter((t) => !t.exited)) {
      const n = nums.get(s.id);
      const where = s.repo ? `${s.repo} on ${s.branch || '?'}${s.git && (s.git.changed || s.git.untracked) ? ' (uncommitted changes)' : ''}` : shortPath(s.cwd);
      const what = s.agent ? `${s.agent.name} · ${STATUS_TEXT[s.agent.status] || s.agent.status} · ${where}` : s.kind === 'shell' ? where : s.command || s.kind;
      items.push({ kind: 'Tab', num: n, label: tabLabel(s), hint: `${s.workspace} · ${s.host} · ${what}`, run: () => this.focusSession(s.id), glyph: numGlyph(s, n) });
    }
    for (const ws of this.workspaces()) if (ws !== this.ws) add(`Switch to ${ws}`, 'Workspace', () => this.switchWorkspace(ws), 'Workspace');
    for (const t of this.templates) add(`Open ${t.name}`, `Workspace template on ${t.host || this.hostName}`, () => this.openTemplate(t.name), 'Workspace');
    for (const h of this.hosts) {
      add(`New tab on ${h.name}`, h.local ? 'This machine' : `${h.target || ''} · ${h.status}`, () => this.newTab({ host: h.name, cwd: '' }), 'Host');
    }
    for (const t of this.tmux) {
      const tab = this.tmuxTab(t.host, t.name);
      const where = tab ? `in tab ${nums.get(tab.id)}` : 'opens a new tab';
      add(`tmux ${t.name}`, `${t.host} · ${t.windows} ${t.windows === 1 ? 'window' : 'windows'} · ${where}`, () => this.openTmux(t.host, t.name), 'tmux', undefined, 'tmux session');
      for (const w of t.wins || []) {
        add(`${t.name} › ${w.index}:${w.name}`, `${t.host}${w.command && w.command !== w.name ? ' · ' + w.command : ''} · ${where}`, () => this.openTmux(t.host, t.name, w.index), 'tmux', undefined, 'tmux window');
      }
    }
    if (cur?.tmuxSession) {
      const ts = this.tmux.find((t) => t.host === cur.host && t.name === cur.tmuxSession);
      const active = ts?.wins?.find((w) => w.active);
      add('Rename tmux session…', cur.tmuxSession, () => this.renameTmuxSession(cur.host, cur.tmuxSession), 'tmux', undefined, 'tmux session rename');
      add('New tmux window', `In ${cur.tmuxSession}`, () => this.tmuxDo(cur.host, 'new-window', cur.tmuxSession), 'tmux', undefined, 'tmux window create');
      if (active) {
        add('Rename tmux window…', `${active.index}:${active.name}`, () => this.renameTmuxWindow(cur.host, cur.tmuxSession, active.index), 'tmux', undefined, 'tmux window rename');
        add('Close tmux window', `${active.index}:${active.name}`, async () => {
          if (await this.confirm(`Close tmux window \`${active.index}:${active.name}\` in \`${cur.tmuxSession}\`? This ends what runs in it.`, 'Close window')) {
            this.tmuxDo(cur.host, 'kill-window', `${cur.tmuxSession}:${active.index}`);
          }
        }, 'tmux', undefined, 'tmux window kill');
      }
      for (const t of this.tmux) {
        if (t.host === cur.host && t.name !== cur.tmuxSession) {
          add(`Switch this tab to tmux ${t.name}`, `${t.windows} ${t.windows === 1 ? 'window' : 'windows'}`, () => this.tmuxDo(cur.host, 'switch-client', t.name, '', cur.id), 'tmux', undefined, 'tmux session switch');
        }
      }
    }
    for (const a of this.allAgents()) {
      const where = tabLabel(a.session);
      add(`${a.state.name}: ${STATUS_TEXT[a.state.status] || a.state.status}`, `${a.host} · ${where} · ${a.state.detail || ''}`, () => this.openAgent(a), 'Agent');
    }
    const b = (a) => this.keys.first(a);
    add('New tab', 'Same machine and folder as this tab', () => this.newTab(), 'Command', b('newTab'));
    add('New workspace…', '', () => this.newWorkspace(), 'Command');
    if (cur && !cur.exited) {
      add('Rename tab…', '', () => this.renameTab(), 'Command', b('rename'));
      add('Move tab to workspace…', '', () => this.moveTab(), 'Command');
      add('Close tab', '', () => this.closeTab(), 'Command', b('closeTab'));
      add('Select text', 'Full scrollback as selectable text (uses tmux history inside tmux)', () => this.openTextView(), 'Command', b('selectText'));
      add('Find in terminal', '', () => this.openFind(), 'Command', b('find'));
      add('Change folder…', `Now: ${shortPath(cur.cwd)} on ${cur.host}`, () => this.openFolderPicker(cur.id), 'Command', undefined, 'cd cwd directory folder path browse');
      add('Copy all text', '', () => this.copyAll(), 'Command');
    }
    add(document.body.classList.contains('side-hidden') ? 'Show workspaces sidebar' : 'Hide workspaces sidebar', '', () => this.toggleSidebar(), 'Command', b('toggleSidebar'), 'panel sidebar workspaces hide show collapse expand');
    add(document.fullscreenElement ? 'Leave fullscreen' : 'Fullscreen', '', () => this.toggleFullscreen(), 'Command', b('fullscreen'), 'full screen maximize zen focus');
    add('Keyboard shortcuts…', 'See and change every shortcut', () => this.openShortcuts(), 'Setting', b('help'), 'keys keybindings hotkeys remap bindings shortcuts keyboard');
    add('Next tab', '', () => this.cycleTab(1), 'Command', b('nextTab'));
    add('Previous tab', '', () => this.cycleTab(-1), 'Command', b('prevTab'));
    add('Next workspace', '', () => this.cycleWorkspace(1), 'Command', b('nextWs'));
    add('Previous workspace', '', () => this.cycleWorkspace(-1), 'Command', b('prevWs'));
    add('Go to the agent that needs you', '', () => this.nextAgentNeedingYou(), 'Command', b('nextAgent'));
    if (cur && (cur.kind === 'ssh' || cur.kind === 'mosh') && cur.remote) {
      const known = this.hostForTarget(cur.remote);
      if (known) add(`New Gecko tab on ${known.name}`, 'Tabs there keep running when the connection drops', () => this.newTab({ host: known.name, cwd: '' }), 'Command', undefined, 'ssh remote');
      else add(`Make ${cur.remote} a Gecko machine…`, 'Install Gecko there; its tabs survive disconnects and show up here', () => this.addHost({ target: cur.remote, check: true }), 'Command', undefined, 'ssh install remote host server adopt');
    }
    add('Add a machine…', 'Run sessions on another computer over SSH', () => this.addHost(), 'Command', undefined, 'ssh host server remote connect computer');
    for (const h of this.hosts) {
      if (!h.local && h.status === 'connected') add(`Update Gecko on ${h.name}`, (h.version || 0) < PROTOCOL ? 'It runs an older version; its tabs keep running' : 'Reinstall this version there; its tabs keep running', () => this.updateHost(h.name), 'Command', undefined, 'upgrade install remote machine host');
    }
    for (const h of this.hosts) if (!h.local) add(`Remove machine ${h.name}`, 'Its sessions keep running there', () => this.removeHost(h.name), 'Command');
    add('Restart Gecko', 'Restarts the background service with the installed version; your tabs and what runs in them keep going', () => this.restartService(), 'Command', undefined, 'restart upgrade update reload daemon service');
    add('Open on another device', 'Phone or tablet: shows a link and QR code', () => this.openShare(), 'Command', undefined, 'phone mobile tablet qr share link');
    add('Notify me when agents need input', '', () => this.enableNotifications(), 'Setting', undefined, 'notifications alerts');
    add(`Copy on select: ${this.settings.copyOnSelect ? 'on' : 'off'}`, 'Toggle', () => this.setSetting('copyOnSelect', !this.settings.copyOnSelect), 'Setting', undefined, 'clipboard selection');
    if (!isMac) add(`Ctrl+C copies selected text: ${this.settings.smartCopy ? 'on' : 'off'}`, 'Without a selection it still interrupts', () => this.setSetting('smartCopy', !this.settings.smartCopy), 'Setting');
    if (isMac) add(`Option key as Meta: ${this.settings.optionIsMeta ? 'on' : 'off'}`, 'Toggle', () => this.setSetting('optionIsMeta', !this.settings.optionIsMeta), 'Setting');
    const cur0 = this.settings.theme === AUTO ? 'Gecko (follows system)' : resolveTheme(this.settings.theme).name;
    add('Change Theme', `Now: ${cur0}`, () => this.openThemePicker(), 'Setting', undefined, 'switch change theme themes colors colours color scheme palette appearance dark light mode background wallpaper skin');
    if (this.desktop?.transparent) {
      add('Background opacity…', `Now: ${this.settings.opacity >= 1 ? 'solid' : Math.round(this.settings.opacity * 100) + '%'} over the desktop`, () => this.openOpacityPicker(), 'Setting', undefined, 'transparency transparent see-through blur glass alpha window opacity');
    }
    const gl = this.settings.renderer === 'webgl' || (this.settings.renderer === 'auto' && !this.touch);
    add(`Renderer: ${gl ? 'WebGL' : 'DOM'}`, `Switch to ${gl ? 'DOM (most compatible)' : 'WebGL (fastest)'}; reloads the page`, () => {
      this.setSetting('renderer', gl ? 'dom' : 'webgl');
      location.reload();
    }, 'Setting', undefined, 'gpu webgl dom rendering performance');
    add('Bigger text', '', () => this.setFont(this.fontSize() + 1), 'Setting', b('fontUp'), 'font size zoom in larger');
    add('Smaller text', '', () => this.setFont(this.fontSize() - 1), 'Setting', b('fontDown'), 'font size zoom out');
    add('Toggle compose box', 'Type prompts with your keyboard\'s autocorrect, send with Enter', () => this.toggleCompose(), 'Command', undefined, 'prompt message write input');
    return items;
  }

  // picker: { placeholder, items, select, onSelect(item), onCancel() } turns
  // the palette into a chooser over its own items.
  openPalette(initial = '', picker = null) {
    this.promptState = null;
    this.content = null;
    this.paletteOpen = true;
    this.picker = picker;
    const el = $('#palette');
    el.hidden = false;
    el.classList.remove('prompt');
    const input = $('#palette input');
    input.placeholder = picker?.placeholder || 'Jump to a tab, agent, workspace or machine. Or run a command.';
    input.value = initial;
    $('#palette .note').textContent = '';
    this.allCommands = picker ? picker.items : this.commands();
    this.renderPalette();
    if (picker?.select > 0) {
      this.paletteSel = picker.select;
      this.renderPaletteSel();
    }
    input.focus();
  }

  // Run a palette item. The palette closes first, so items that open a
  // prompt or another picker can reuse it.
  choose(item) {
    if (!item) return;
    if (item.keepOpen) return item.run(); // e.g. going into a folder
    this.picker = null; // chosen, not cancelled
    this.closePalette();
    item.run();
  }

  closePalette(result) {
    $('#palette').hidden = true;
    this.paletteOpen = false;
    const pk = this.picker;
    this.picker = null;
    if (pk) pk.onCancel?.();
    const ps = this.promptState;
    this.promptState = null;
    if (ps) ps.resolve(result ?? null);
    if (!this.textViewOpen) this.activePane()?.focus();
  }

  renderPalette(keepSel = false) {
    const q = $('#palette input').value.trim();
    if (this.promptState) return;
    let items = this.allCommands;
    if (typeof items === 'function') {
      items = items(q); // the picker filters its own items
    } else if (q === '?') {
      items = this.allCommands.filter((i) => i.combo || i.label === 'Keyboard shortcuts…');
    } else if (/^\d+$/.test(q)) {
      // A number jumps to that tab; other matches follow.
      const n = +q;
      items = [...items.filter((i) => i.num === n), ...items.filter((i) => i.num !== n && `${i.label} ${i.hint}`.includes(q))];
    } else if (q) {
      items = items
        .map((i) => {
          // Exact keyword ("colors" -> Change Theme) beats a loose fuzzy match.
          const kw = i.keywords && ` ${i.keywords} `.includes(` ${q.toLowerCase()}`) ? 60 : -1;
          return { i, s: Math.max(kw, fuzzy(q, i.label) * 2, fuzzy(q, `${i.num ?? ''} ${i.label} ${i.hint} ${i.kind} ${i.keywords || ''}`)) };
        })
        .filter((x) => x.s >= 0)
        .sort((a, b) => b.s - a.s)
        .map((x) => x.i);
    }
    items = items.slice(0, 60);
    if (!this.picker && q.length >= 2 && !/^\d+$/.test(q)) {
      items = items.concat(this.contentItems(q));
      this.searchContent(q);
    }
    this.paletteItems = items;
    this.paletteSel = keepSel ? Math.min(this.paletteSel, Math.max(items.length - 1, 0)) : 0;
    $('#palette ul').innerHTML = this.paletteItems.map((i, n) => `
      <li data-i="${n}">
        <span class="k">${esc(i.kind)}</span>
        <span class="l${i.mono ? ' mono' : ''}">${i.glyph || ''}<span class="t">${esc(i.label)}</span></span>
        <span class="h">${esc(i.hint || '')}</span>
        ${i.combo ? `<kbd>${esc(prettyCombo(i.combo))}</kbd>` : ''}
      </li>`).join('') || `<li class="none">Nothing matches “${esc(q)}”.</li>`;
    this.renderPaletteSel();
  }

  // Ask every host to search its tabs' text (debounced); results arrive
  // later and are merged into the open palette.
  searchContent(q) {
    if (this.content?.q === q) return;
    clearTimeout(this.contentTimer);
    this.contentTimer = setTimeout(async () => {
      try {
        const r = await this.conn.request({ t: 'search', text: q });
        this.content = { q, matches: r.matches || [] };
      } catch {
        this.content = { q, matches: [] };
      }
      if (this.paletteOpen && !this.picker && !this.promptState && $('#palette input').value.trim() === q) this.renderPalette(true);
    }, 150);
  }

  contentItems(q) {
    if (this.content?.q !== q) return [];
    const nums = this.tabNumbers();
    return this.content.matches
      .filter((m) => this.sessions.has(m.id))
      .map((m) => {
        const s = this.sessions.get(m.id);
        return {
          kind: 'In tab',
          label: m.line,
          hint: `${tabLabel(s)} · ${s.workspace}${m.count > 1 ? ` · ${m.count} matches` : ''}`,
          glyph: numGlyph(s, nums.get(m.id)),
          mono: true,
          run: () => this.revealMatch(m.id, q),
        };
      });
  }

  // Switch to a tab and highlight the most recent occurrence of q.
  revealMatch(id, q) {
    this.focusSession(id);
    const p = this.panes.get(id);
    if (!p) return;
    const find = () => p.search.findPrevious(q, { caseSensitive: false });
    // A freshly opened tab is still replaying its history.
    setTimeout(() => { if (!find()) setTimeout(find, 400); }, 150);
  }

  renderPaletteSel() {
    const lis = document.querySelectorAll('#palette li[data-i]');
    lis.forEach((li, n) => li.classList.toggle('sel', n === this.paletteSel));
    lis[this.paletteSel]?.scrollIntoView({ block: 'nearest' });
    this.picker?.onSelect?.(this.paletteItems?.[this.paletteSel]);
  }

  // A one-line text prompt in the palette's place.
  prompt(title, value = '', note = '') {
    return new Promise((resolve) => {
      if (this.promptState) this.promptState.resolve(null);
      this.paletteOpen = true;
      this.promptState = { resolve };
      const el = $('#palette');
      el.hidden = false;
      el.classList.add('prompt');
      const input = $('#palette input');
      input.placeholder = title;
      input.value = value;
      $('#palette .note').textContent = note ? `${title}. ${note}` : title;
      $('#palette ul').innerHTML = '';
      input.focus();
      input.select();
    });
  }

  finishPrompt(v) {
    this.closePalette(v.trim());
  }

  confirm(text, verb) {
    return new Promise((resolve) => {
      const m = $('#modal');
      m.innerHTML = `<div class="card" role="alertdialog" aria-modal="true">
        <p>${md(text)}</p>
        <div class="row"><button data-no>Cancel</button><button class="primary" data-yes>${esc(verb)}</button></div></div>`;
      m.hidden = false;
      const done = (v) => { m.hidden = true; m.innerHTML = ''; resolve(v); this.activePane()?.focus(); };
      m.querySelector('[data-yes]').onclick = () => done(true);
      m.querySelector('[data-no]').onclick = () => done(false);
      m.onkeydown = (ev) => { if (ev.key === 'Escape') done(false); };
      m.querySelector('[data-yes]').focus();
    });
  }

  async openShare() {
    const m = $('#modal');
    let urls = [];
    try {
      const r = await fetch('/api/info');
      urls = (await r.json()).urls || [];
    } catch {}
    const here = location.origin + '/';
    let body;
    if (location.hostname !== '127.0.0.1' && location.hostname !== 'localhost') {
      urls = [here];
      body = '<p>Open this address on your other device. You may need the access token: run <code>gecko url</code> on the machine running Gecko.</p>';
    } else if (!urls.length) {
      body = `<p>Gecko only accepts connections from this computer right now.</p>
        <p>To use it from your phone, restart the daemon listening on your network or tailnet, for example <code>gecko stop && gecko daemon --listen 0.0.0.0:7681</code>, or set <code>"listen"</code> in the config. Over the internet, put it behind Tailscale (<code>tailscale serve 7681</code>) rather than exposing the port.</p>`;
    } else {
      body = '<p>Scan with your phone (same network or tailnet). The link contains your access token, so treat it like a password.</p>';
    }
    const qrs = urls.map((u) => {
      const q = qrcode(0, 'M');
      q.addData(u);
      q.make();
      return `<figure>${q.createSvgTag({ cellSize: 4, margin: 2, scalable: true })}<figcaption><code>${esc(u)}</code></figcaption></figure>`;
    }).join('');
    m.innerHTML = `<div class="card share" role="dialog" aria-modal="true"><h2>Open on another device</h2>${body}<div class="qrs">${qrs}</div>
      <div class="row"><button class="primary" data-no>Done</button></div></div>`;
    m.hidden = false;
    m.querySelector('[data-no]').onclick = () => { m.hidden = true; m.innerHTML = ''; };
    m.querySelector('[data-no]').focus();
  }

  // ---------------------------------------------------------------- text view

  bindTextView() {
    const tv = $('#textview');
    tv.querySelector('[data-close]').onclick = () => this.closeTextView();
    tv.querySelector('[data-copy-sel]').onclick = async () => {
      const sel = window.getSelection().toString();
      if (!sel) return this.toast('Select some text first.');
      this.toast((await copyText(sel)) ? 'Copied' : 'Copy failed');
    };
    tv.querySelector('[data-copy-all]').onclick = async () => {
      this.toast((await copyText(tv.querySelector('pre').textContent)) ? 'Copied everything' : 'Copy failed');
    };
    tv.addEventListener('keydown', (ev) => {
      if (ev.key === 'Escape') this.closeTextView();
    });
    tv.querySelector('input').addEventListener('keydown', (ev) => {
      if (ev.key === 'Enter') {
        ev.preventDefault();
        if (!window.find?.(ev.target.value, false, ev.shiftKey, true)) this.toast('No more matches');
      }
    });
  }

  async openTextView() {
    const id = this.activeId();
    const p = this.activePane();
    if (!id || !p) return;
    let text;
    try {
      text = (await this.conn.request({ t: 'capture', id })).text;
    } catch {
      text = null;
    }
    if (text == null || text === '') {
      const buf = p.term.buffer.active;
      const lines = [];
      for (let i = 0; i < buf.length; i++) lines.push(buf.getLine(i)?.translateToString(true) ?? '');
      while (lines.length && !lines[lines.length - 1]) lines.pop();
      text = lines.join('\n');
    }
    const tv = $('#textview');
    const s = this.sessions.get(id);
    tv.querySelector('h2').textContent = s ? tabLabel(s) : 'Text';
    tv.querySelector('pre').textContent = text;
    tv.hidden = false;
    this.textViewOpen = true;
    const pre = tv.querySelector('pre');
    pre.scrollTop = pre.scrollHeight;
    pre.focus();
  }

  closeTextView() {
    $('#textview').hidden = true;
    this.textViewOpen = false;
    this.activePane()?.focus();
  }

  async copyAll() {
    const id = this.activeId();
    if (!id) return;
    try {
      const { text } = await this.conn.request({ t: 'capture', id });
      this.toast((await copyText(text)) ? 'Copied everything' : 'Copy failed');
    } catch (e) {
      this.toast(e.message);
    }
  }

  // ---------------------------------------------------------------- find

  bindFind() {
    const box = $('#find');
    const input = box.querySelector('input');
    const go = (back) => {
      const p = this.activePane();
      if (!p || !input.value) return;
      const ok = back ? p.search.findPrevious(input.value) : p.search.findNext(input.value);
      box.classList.toggle('miss', !ok);
    };
    input.addEventListener('keydown', (ev) => {
      if (ev.key === 'Enter') { ev.preventDefault(); go(ev.shiftKey); }
      if (ev.key === 'Escape') { box.hidden = true; this.activePane()?.search.clearDecorations(); this.activePane()?.focus(); }
    });
    input.addEventListener('input', () => go(false));
    box.querySelector('[data-close]').onclick = () => { box.hidden = true; this.activePane()?.focus(); };
  }

  openFind() {
    const box = $('#find');
    box.hidden = false;
    const input = box.querySelector('input');
    const sel = this.activePane()?.term.getSelection();
    if (sel && !sel.includes('\n')) input.value = sel;
    input.focus();
    input.select();
  }

  // ---------------------------------------------------------------- render

  toast(text, onClick) {
    const t = document.createElement('div');
    t.className = 'toast';
    t.innerHTML = md(text);
    t.setAttribute('role', 'status');
    if (onClick) {
      t.classList.add('clickable');
      t.onclick = () => { onClick(); t.remove(); };
    }
    $('#toasts').appendChild(t);
    setTimeout(() => t.remove(), onClick ? 8000 : 3500);
  }

  renderSoon() {
    if (this.raf) return;
    this.raf = requestAnimationFrame(() => {
      this.raf = 0;
      this.render();
    });
  }

  render() {
    this.renderSidebar();
    this.renderTabs();
    this.showActive();
    this.renderEmpty();
    this.renderDock();
    this.renderStatus();
    this.updateTitle();
  }

  renderSidebar() {
    const parts = [];
    parts.push('<div class="group"><h3>Workspaces <button class="mini" data-act="new-ws" title="New workspace" aria-label="New workspace">+</button></h3>');
    const nums = this.tabNumbers();
    for (const ws of this.workspaces()) {
      const tabs = this.tabsOf(ws);
      const agents = tabs.filter((t) => t.agent);
      const need = agents.filter((t) => t.agent.status === 'needs-input').length;
      const working = agents.filter((t) => t.agent.status === 'working').length;
      const hosts = [...new Set(tabs.map((t) => t.host))].filter((h) => h && h !== this.hostName);
      const folded = this.collapsed[ws];
      parts.push(`<div class="ws ${ws === this.ws ? 'cur' : ''}">
        <div class="ws-row">
          <button class="ws-name" data-act="ws" data-ws="${esc(ws)}" aria-expanded="${!folded}" title="Click to ${folded ? 'expand' : 'collapse'}, double-click to rename"><span class="fold" aria-hidden="true">${folded ? '▸' : '▾'}</span>${esc(ws)}</button>
          ${need ? `<span class="pill need" title="Agents waiting for you">${need}</span>` : ''}
          ${working ? `<span class="pill work" title="Agents working">${working}</span>` : ''}
          ${hosts.length ? `<span class="hosts">${esc(hosts.join(', '))}</span>` : ''}
          <button class="x" data-act="ws-close" data-ws="${esc(ws)}" aria-label="Close workspace ${esc(ws)}" title="Close workspace and its tabs">×</button>
        </div>`);
      if (!folded) {
        parts.push('<ul>');
        for (const t of tabs) {
          const active = ws === this.ws && t.id === this.activeId();
          const sub = t.agent?.detail || (t.kind === 'shell' ? shortPath(t.cwd) : t.command) || '';
          const tip = [t.host, t.repo && `${t.repo} on ${t.branch || '?'}${t.git ? ' (' + gitStateText(t.git) + ')' : ''}`, sub].filter(Boolean).join(' · ');
          parts.push(`<li><div class="tab-row ${active ? 'cur' : ''} ${t.exited ? 'gone' : ''}" role="button" data-act="tab" data-id="${esc(t.id)}" title="${esc(tip)}">
            ${numGlyph(t, nums.get(t.id))}<span class="nm">${esc(tabLabel(t))} ${gitChip(t)}</span>${t.host !== this.hostName ? `<span class="where">${esc(t.host)}</span>` : '<span></span>'}<button class="x" data-act="tab-close" data-id="${esc(t.id)}" aria-label="Close tab" title="Close tab">×</button></div></li>`);
        }
        parts.push('</ul>');
      }
      parts.push('</div>');
    }
    const open = new Set(this.workspaces());
    for (const t of this.templates) {
      if (open.has(t.name)) continue;
      parts.push(`<div class="ws template"><button class="ws-name" data-act="template" data-name="${esc(t.name)}">${esc(t.name)} <span class="hosts">open</span></button></div>`);
    }
    parts.push('</div><div class="group"><h3>Machines <button class="mini" data-act="add-host" title="Add a machine" aria-label="Add a machine">+</button></h3><ul class="hostlist">');
    for (const h of this.hosts) {
      const tm = this.tmux.filter((t) => t.host === h.name);
      const broken = !h.local && h.status === 'error';
      const outdated = !h.local && h.status === 'connected' && (h.version || 0) < PROTOCOL;
      parts.push(`<li class="host st-${esc(h.status)}">
        <div class="host-row">
          <button class="host-main" data-act="${broken ? 'host-fix' : outdated ? 'host-update' : 'host-new'}" data-host="${esc(h.name)}" title="${esc(broken ? 'Fix the connection to ' + h.name : outdated ? h.name + ' runs an older Gecko; click to update it (its tabs keep running)' : 'New tab on ' + h.name)}">
            <i class="hdot"></i><span class="nm">${esc(h.name)}</span><span class="where">${esc(h.local ? 'this machine' : h.status === 'connected' ? h.os || '' : h.status)}</span><span class="plus${outdated ? ' upd' : ''}">${broken ? 'fix' : outdated ? 'update' : '+'}</span>
          </button>
          ${h.local ? '' : `<button class="x" data-act="host-rm" data-host="${esc(h.name)}" aria-label="Remove ${esc(h.name)}" title="Remove ${esc(h.name)}">×</button>`}
        </div>
        ${h.error && h.status !== 'connected' ? `<div class="err">${esc(h.error)}</div>` : ''}
        ${tm.map((t) => {
          const tab = this.tmuxTab(h.name, t.name);
          const open = !!this.tmuxOpen[h.name + '/' + t.name];
          const wins = (t.wins || []).map((w) => `<button class="tmux-win ${w.active ? 'on' : ''}" data-act="tmux-win" data-host="${esc(h.name)}" data-name="${esc(t.name)}" data-idx="${w.index}" title="${esc((tab ? 'Show this window in tab ' + nums.get(tab.id) : 'Open in a new tab') + '. Double-click to rename.')}"><span class="wi">${w.index}</span>${esc(w.name)}${w.command && w.command !== w.name ? `<span class="where">${esc(w.command)}</span>` : ''}</button>`).join('');
          return `<div class="tmux-row">
            <button class="fold" data-act="tmux-fold" data-host="${esc(h.name)}" data-name="${esc(t.name)}" aria-label="${open ? 'Hide' : 'Show'} windows">${open ? '▾' : '▸'}</button>
            <button class="tmux" data-act="tmux" data-host="${esc(h.name)}" data-name="${esc(t.name)}" title="${esc((tab ? 'Go to tab ' + nums.get(tab.id) + ', which shows this session' : 'Open in a new tab') + '. Double-click to rename.')}">${TMUX_ICON}<b>${esc(t.name)}</b><span class="where">${t.windows} ${t.windows === 1 ? 'window' : 'windows'}</span>${tab ? numGlyph(tab, nums.get(tab.id)) : ''}</button>
          </div>${open ? `<div class="tmux-wins">${wins}</div>` : ''}`;
        }).join('')}
      </li>`);
    }
    parts.push('</ul></div>');
    $('#side-body').innerHTML = parts.join('');
    const st = this.conn.status;
    const conn = $('#conn');
    conn.className = 'conn ' + (st === 'online' && this.outdated ? 'outdated' : st);
    conn.textContent = st === 'online' ? (this.outdated ? 'Restart needed' : '') : st === 'unauthorized' ? 'Not signed in' : st === 'offline' ? 'Reconnecting…' : 'Connecting…';
    conn.title = st === 'online' && this.outdated ? OUTDATED : '';
  }

  renderTabs() {
    const tabs = this.tabsOf(this.ws);
    const act = this.activeId();
    const nums = this.tabNumbers();
    $('#ws-title').textContent = this.ws;
    $('#tabs').innerHTML = tabs.map((t) => {
      const n = nums.get(t.id);
      return `
      <div class="tab ${t.id === act ? 'cur' : ''} ${t.exited ? 'gone' : ''} ${t.attention ? 'bell' : ''}" data-tab="${esc(t.id)}" role="tab" aria-selected="${t.id === act}" title="${esc(`${tabLabel(t)} - ${t.host}${t.cwd ? ' · ' + t.cwd : ''}${n <= 9 ? ` (${prettyCombo((isMac ? 'cmd+' : 'alt+') + n)})` : ''}`)}">
        ${numGlyph(t, n)}<span class="nm">${esc(tabLabel(t))}</span>${gitChip(t)}${t.host !== this.hostName ? `<span class="where">${esc(t.host)}</span>` : ''}
        <button class="x" data-close="${esc(t.id)}" aria-label="Close tab">×</button>
      </div>`;
    }).join('');
    $('#tabs .cur')?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }

  renderEmpty() {
    const el = $('#empty');
    if (el.hidden) return;
    if (this.fitRequest && this.firstSync) {
      // Opened with no tabs: size the window for the first one now.
      const { cols, rows } = this.fitRequest;
      this.fitRequest = null;
      requestAnimationFrame(() => this.fitWindow(cols, rows));
    }
    const hosts = this.hosts.length ? this.hosts : [{ name: this.hostName || 'this machine', local: true, status: 'connected' }];
    const offline = this.conn.status !== 'online';
    el.innerHTML = offline
      ? `<div class="hello"><h1>${this.conn.status === 'unauthorized' ? 'Gecko needs your access link' : 'Connecting to Gecko…'}</h1>
         <p>${this.conn.status === 'unauthorized'
          ? 'Open the link printed by <code>gecko url</code> on the machine running Gecko.'
          : 'If this takes long, start the daemon with <code>gecko</code> on that machine.'}</p></div>`
      : `<div class="hello">
        <h1>${this.sessions.size ? `${esc(this.ws)} has no tabs` : 'No sessions yet'}</h1>
        <p>Open a shell on one of your machines. Sessions keep running when you close this window or lose the connection.</p>
        <div class="choices">${hosts.map((h) => `<button class="primary" data-act="host-new" data-host="${esc(h.name)}" ${h.status !== 'connected' ? 'disabled' : ''}>New tab on ${esc(h.name)}</button>`).join('')}
        ${this.templates.map((t) => `<button data-act="template" data-name="${esc(t.name)}">Open ${esc(t.name)}</button>`).join('')}</div>
        <p class="keys">${prettyCombo(this.keys.first('palette'))} search everything · ${prettyCombo(this.keys.first('newTab'))} new tab · ${prettyCombo(this.keys.first('nextAgent'))} agent that needs you</p>
      </div>`;
  }

  renderDock() {
    const agents = this.allAgents();
    const dock = $('#dock');
    dock.hidden = !agents.length;
    if (!agents.length) {
      dock.innerHTML = '';
      return;
    }
    const act = this.activeId();
    dock.innerHTML = agents.map((a) => {
      const s = a.session;
      const where = tabLabel(s) === a.state.name ? s.workspace : tabLabel(s);
      const replies = a.state.status === 'needs-input'
        ? `<span class="replies">${['1', '2', '3', 'y', 'n'].map((k) => `<button data-reply="${k}" data-id="${esc(s.id)}">${k}</button>`).join('')}<button data-reply="esc" data-id="${esc(s.id)}" aria-label="Escape">esc</button><button data-reply="enter" data-id="${esc(s.id)}" aria-label="Enter">⏎</button></span>`
        : '';
      return `<div class="agent s-${esc(a.state.status)} ${s && s.id === act ? 'cur' : ''}" data-agent="${esc(a.key)}" role="button" tabindex="0"
          title="${esc(`${a.state.name} on ${a.host}: ${STATUS_TEXT[a.state.status] || a.state.status}${a.state.detail ? ' - ' + a.state.detail : ''}`)}">
        <i class="dot s-${esc(a.state.status)}"></i>
        <span class="who"><b>${esc(a.state.name)}</b> ${esc(where)}${a.host !== this.hostName ? ` <span class="where">${esc(a.host)}</span>` : ''}</span>
        <span class="what">${esc(a.state.detail || STATUS_TEXT[a.state.status] || '')}</span>
        <span class="since">${esc(ago(a.state.since))}</span>
        ${replies}
      </div>`;
    }).join('');
  }

  renderStatus() {
    const s = this.activeSession();
    const el = $('#statusline');
    if (!s) {
      el.innerHTML = '';
      return;
    }
    const bits = [`<span class="host">${esc(s.host)}</span>`];
    if (s.kind === 'ssh' || s.kind === 'mosh') {
      bits.push(`<span class="tmux-on" title="${s.kind} to ${esc(s.remote || '?')}">${tag(s.kind.toUpperCase())}${esc(s.remote || '?')}</span>`);
      if (s.remoteTmux) bits.push(`<span class="tmux-on" title="tmux session ${esc(s.remoteTmux)} on ${esc(s.remote || 'the remote machine')}">${TMUX_ICON}${esc(s.remoteTmux)}</span>`);
      const known = this.hostForTarget(s.remote);
      if (known) bits.push(`<button class="sl-action" data-act="host-new" data-host="${esc(known.name)}" title="Gecko tabs on ${esc(known.name)} keep running when the connection drops">New Gecko tab on ${esc(known.name)}</button>`);
      else if (s.remote) bits.push(`<button class="sl-action" data-act="adopt-ssh" data-target="${esc(s.remote)}" title="Install Gecko there so tabs on that machine survive disconnects and show up here">Make it a Gecko machine</button>`);
    }
    if (s.tmuxSession) {
      bits.push(`<span class="tmux-on" title="tmux session ${esc(s.tmuxSession)} (double-click to rename)" data-act="tmux-rename">${TMUX_ICON}${esc(s.tmuxSession)}</span>`);
      const ts = this.tmux.find((t) => t.host === s.host && t.name === s.tmuxSession);
      if (ts?.wins?.length) {
        bits.push(`<span class="tmux-wins-bar">${ts.wins.map((w) => `<button class="${w.active ? 'on' : ''}" data-act="tmux-sel" data-idx="${w.index}" title="${esc(w.command || w.name)} (double-click to rename)">${w.index}:${esc(w.name)}</button>`).join('')}<button data-act="tmux-new" title="New tmux window" aria-label="New tmux window">+</button></span>`);
      }
    }
    // ssh/mosh/tmux tabs: the local folder and client command say nothing
    // useful (the remote shell or tmux pane has its own folder).
    const remoteTab = s.kind === 'ssh' || s.kind === 'mosh' || s.kind === 'tmux';
    if (remoteTab) {
      // nothing more
    } else if (s.repo) {
      const sub = repoPath(s);
      bits.push(`<span class="repo">${gitChip(s, true)}<button class="cwd pick" data-act="pick-cwd" title="${esc(s.cwd)} (click to change folder)">${esc(sub || shortPath(s.cwd))}</button></span>`);
    } else if (s.cwd) {
      bits.push(`<button class="cwd pick" data-act="pick-cwd" title="${esc(s.cwd)} (click to change folder)">${esc(shortPath(s.cwd))}</button>`);
    }
    if (s.command && s.kind !== 'shell' && !remoteTab) bits.push(`<span class="cmd" title="${esc(s.command)}">${esc(s.command)}</span>`);
    if (s.lastExit != null && s.lastExit !== 0 && !s.running) bits.push(`<span class="bad">exit ${s.lastExit}</span>`);
    const p = this.panes.get(s.id);
    const right = [];
    if (s.notice) right.push(`<span class="notice" title="${esc(s.notice)}">${esc(s.notice)}</span>`);
    if (p) right.push(`<span>${p.term.cols}×${p.term.rows}</span>`);
    right.push(`<span class="conn-dot ${this.conn.status}" title="Connection: ${this.conn.status}"></span>`);
    el.innerHTML = `<div class="l">${bits.join('')}</div><div class="r">${right.join('')}</div>`;
  }

  updateTitle() {
    const need = this.allAgents().filter((a) => a.state.status === 'needs-input').length;
    const s = this.activeSession();
    document.title = `${need ? `(${need}) ` : ''}${s ? tabLabel(s) + ' - ' : ''}Gecko`;
    const icon = document.querySelector('link[rel=icon]');
    const want = need ? '/icon-alert.svg' : '/icon.svg';
    if (icon && icon.getAttribute('href') !== want) icon.setAttribute('href', want);
  }
}

// Colors first, so the page never flashes the default theme.
if (window.geckoDesktop) document.documentElement.classList.add('desktop', 'desktop-' + window.geckoDesktop.platform);
paintTheme(
  resolveTheme(initialTheme()),
  window.geckoDesktop?.transparent ? store('opacity', 0.9) : 1,
);

// xterm measures glyphs when a terminal opens, so wait for the font.
Promise.race([
  document.fonts.load('13px "JetBrains Mono Variable"'),
  new Promise((r) => setTimeout(r, 1500)),
]).finally(() => {
  splash.set(0.65); // font ready
  window.gecko = new App();
});

if ('serviceWorker' in navigator && window.isSecureContext) {
  navigator.serviceWorker.register('/sw.js').catch(() => {});
}
