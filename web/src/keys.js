// Keyboard shortcuts. Browsers reserve some combos (Ctrl+T, Ctrl+W, ...);
// installing Gecko as an app (or `gecko open`, which uses an app window)
// lets it receive more of them. Every action is also in the palette.
import { isMac } from './util.js';

const MAC = {
  palette: ['cmd+k', 'cmd+shift+p'],
  toggleSidebar: ['cmd+b'],
  fullscreen: ['ctrl+cmd+f'],
  newTab: ['cmd+t'],
  closeTab: ['cmd+w'],
  nextTab: ['cmd+shift+]', 'ctrl+tab'],
  prevTab: ['cmd+shift+[', 'ctrl+shift+tab'],
  nextWs: ['cmd+shift+down'],
  prevWs: ['cmd+shift+up'],
  copy: ['cmd+c'],
  find: ['cmd+f'],
  selectText: ['cmd+shift+s'],
  nextAgent: ['cmd+j'],
  fontUp: ['cmd+=', 'cmd+shift+='],
  fontDown: ['cmd+-'],
  fontReset: ['cmd+0'],
  rename: ['f2'],
  help: ['cmd+/'],
};

const OTHER = {
  palette: ['ctrl+k', 'ctrl+shift+k', 'ctrl+shift+p'],
  toggleSidebar: ['ctrl+shift+b'],
  fullscreen: ['f11'],
  newTab: ['ctrl+t', 'ctrl+shift+t'],
  closeTab: ['ctrl+w', 'ctrl+shift+w'],
  nextTab: ['ctrl+pagedown', 'alt+]', 'ctrl+tab'],
  prevTab: ['ctrl+pageup', 'alt+[', 'ctrl+shift+tab'],
  nextWs: ['alt+shift+down', 'ctrl+shift+pagedown'],
  prevWs: ['alt+shift+up', 'ctrl+shift+pageup'],
  copy: ['ctrl+shift+c'],
  find: ['ctrl+shift+f'],
  selectText: ['ctrl+shift+s'],
  nextAgent: ['ctrl+shift+j'],
  fontUp: ['ctrl+=', 'ctrl+shift+='],
  fontDown: ['ctrl+-'],
  fontReset: ['ctrl+0'],
  rename: ['f2'],
  help: ['ctrl+shift+/'],
};

// Defaults for this platform. People can override any action in
// settings.json ("keys": {"newTab": ["ctrl+t"], "tabNumbers": "alt"}),
// usually through the Keyboard shortcuts editor.
export const DEFAULTS = isMac ? MAC : OTHER;
export const DEFAULT_TAB_MODIFIER = isMac ? 'cmd' : 'alt';
export const TAB_MODIFIERS = isMac ? ['cmd', 'ctrl', 'alt'] : ['alt', 'ctrl'];

// What each action is called in the shortcuts editor, in display order.
export const ACTIONS = {
  palette: 'Search tabs, agents and commands',
  newTab: 'New tab',
  closeTab: 'Close tab',
  nextTab: 'Next tab',
  prevTab: 'Previous tab',
  nextWs: 'Next workspace',
  prevWs: 'Previous workspace',
  nextAgent: 'Go to the agent that needs you',
  copy: 'Copy selection',
  find: 'Find in terminal',
  selectText: 'Select text (full scrollback)',
  rename: 'Rename tab',
  toggleSidebar: 'Show or hide the sidebar',
  fullscreen: 'Fullscreen',
  fontUp: 'Bigger text',
  fontDown: 'Smaller text',
  fontReset: 'Default text size',
  help: 'List shortcuts',
};

// Effective bindings: defaults with the person's overrides on top.
export function effectiveBindings(overrides = {}) {
  const out = {};
  for (const action of Object.keys(DEFAULTS)) {
    out[action] = Array.isArray(overrides[action]) ? overrides[action] : DEFAULTS[action];
  }
  return out;
}

const CODE_NAMES = {
  BracketLeft: '[', BracketRight: ']', Equal: '=', Minus: '-', Slash: '/', Backquote: '`',
  PageUp: 'pageup', PageDown: 'pagedown', ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left',
  ArrowRight: 'right', Tab: 'tab', Escape: 'escape', Enter: 'enter', F2: 'f2', Comma: ',',
};

export function comboOf(ev) {
  let key = CODE_NAMES[ev.code];
  if (!key) {
    if (ev.code.startsWith('Key')) key = ev.code.slice(3).toLowerCase();
    else if (ev.code.startsWith('Digit')) key = ev.code.slice(5);
    else key = (ev.key || '').toLowerCase();
  }
  return (ev.ctrlKey ? 'ctrl+' : '') + (ev.altKey ? 'alt+' : '') + (ev.shiftKey ? 'shift+' : '') + (ev.metaKey ? 'cmd+' : '') + key;
}

export function prettyCombo(c) {
  if (!c) return '';
  const map = isMac
    ? { cmd: '⌘', ctrl: '⌃', alt: '⌥', shift: '⇧' }
    : { cmd: 'Win', ctrl: 'Ctrl', alt: 'Alt', shift: 'Shift' };
  const names = { pagedown: 'PgDn', pageup: 'PgUp', escape: 'Esc', enter: 'Enter', tab: 'Tab', up: '↑', down: '↓', left: '←', right: '→' };
  return c.split('+').map((p) => map[p] || names[p] || (p.length === 1 ? p.toUpperCase() : p[0].toUpperCase() + p.slice(1))).join(isMac ? '' : '+');
}

export class Keys {
  constructor(app) {
    this.app = app;
    this.rebuild(app.settings.keys);
    window.addEventListener('keydown', (ev) => {
      // Terminal keys are handled in terminalKey; this catches the rest of the UI.
      if (ev.target.closest?.('.xterm')) return;
      if (this.handle(ev)) ev.preventDefault();
    }, true);
  }

  // Apply key overrides (from settings).
  rebuild(overrides = {}) {
    this.bindings = effectiveBindings(overrides || {});
    this.tabModifier = TAB_MODIFIERS.includes(overrides?.tabNumbers) ? overrides.tabNumbers : DEFAULT_TAB_MODIFIER;
    this.map = new Map();
    for (const [action, combos] of Object.entries(this.bindings)) for (const c of combos) this.map.set(c, action);
  }

  first(action) {
    return this.bindings[action]?.[0];
  }

  tabNumber(ev) {
    const m = /^Digit([1-9])$/.exec(ev.code);
    if (!m || ev.shiftKey) return 0;
    const mods = { cmd: ev.metaKey, ctrl: ev.ctrlKey, alt: ev.altKey };
    const ok = Object.entries(mods).every(([k, on]) => on === (k === this.tabModifier));
    return ok ? +m[1] : 0;
  }

  handle(ev) {
    if (ev.type !== 'keydown' || this.app.capturingKeys) return false;
    const n = this.tabNumber(ev);
    if (n) {
      this.app.selectTabIndex(n - 1);
      return true;
    }
    const action = this.map.get(comboOf(ev));
    if (!action) return false;
    return this.app.action(action, ev) !== false;
  }

  // Called by xterm for every key event; returning false stops xterm.
  terminalKey(ev, pane) {
    if (ev.type !== 'keydown') return true;
    const app = this.app;
    const combo = comboOf(ev);
    // Smart copy: Ctrl+C with a selection copies instead of interrupting.
    if (!isMac && combo === 'ctrl+c' && app.settings.smartCopy && pane.term.hasSelection()) {
      app.copySelection(pane);
      ev.preventDefault();
      return false;
    }
    // Let the browser deliver a paste event to xterm (bracketed paste aware).
    if (combo === (isMac ? 'cmd+v' : 'ctrl+shift+v')) return false;
    if (this.app.capturingKeys) return false;
    if (isMac && ev.metaKey && !this.map.has(combo) && !this.tabNumber(ev)) return false;
    if (this.handle(ev)) {
      ev.preventDefault();
      return false;
    }
    return true;
  }
}
