// Themes. Each one colors both the terminal (16 ANSI colors) and the UI
// (the UI tokens are mixed from the theme's background and foreground),
// plus an accent (cursor, current tab, selection) and a "needs you" color.

// Gecko: Nord's calm, low-contrast palette turned green. Same lightness
// and (reduced) saturation as Nord's polar night / snow storm, hue rotated
// to green-grey; Nord's sage as the accent and its orange for attention.
const geckoDark = {
  id: 'gecko-dark', name: 'Gecko Dark', dark: true, accent: '#a3be8c', need: '#d08770', brand: '#a3be8c',
  ui: { panel: '#2c352f', raise: '#404d44', line: '#46564b', muted: '#90a095', faint: '#6b7d70', idle: '#90a095' },
  term: {
    background: '#323c35', foreground: '#dbe6df',
    black: '#404d44', red: '#bf616a', green: '#a3be8c', yellow: '#ebcb8b',
    blue: '#81a1c1', magenta: '#b48ead', cyan: '#88c0b0', white: '#e3ebe5',
    brightBlack: '#6b7d70', brightRed: '#d07a83', brightGreen: '#b6cfa1', brightYellow: '#f0d7a3',
    brightBlue: '#9ab6d1', brightMagenta: '#c5a3c0', brightCyan: '#9fd0c2', brightWhite: '#eef2ef',
  },
};

const geckoLight = {
  id: 'gecko-light', name: 'Gecko Light', dark: false, accent: '#5d8a4a', need: '#b5653f', brand: '#5d8a4a',
  ui: { panel: '#e5ebe4', raise: '#dae3d8', line: '#c8d2c6', muted: '#627267', faint: '#809186', idle: '#627267' },
  term: {
    background: '#eef2ee', foreground: '#323c35',
    black: '#323c35', red: '#b5545e', green: '#5d8a4a', yellow: '#a07f2e',
    blue: '#5e81ac', magenta: '#9a6a92', cyan: '#4c8a86', white: '#b8c4bb',
    brightBlack: '#627267', brightRed: '#c4656f', brightGreen: '#6f9c5b', brightYellow: '#b08f3e',
    brightBlue: '#6f92bd', brightMagenta: '#ab7ba3', brightCyan: '#5d9b97', brightWhite: '#ffffff',
  },
};

// ansi(background, foreground, 8 normal colors, 8 bright colors)
function ansi(background, foreground, normal, bright) {
  const names = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white'];
  const t = { background, foreground };
  names.forEach((n, i) => {
    t[n] = normal[i];
    t['bright' + n[0].toUpperCase() + n.slice(1)] = bright[i];
  });
  return t;
}

export const THEMES = [
  geckoDark,
  geckoLight,
  {
    // Mission Control: launch-webcast black, off-white, cool greys, white as
    // the only accent and a signal orange for "needs you". It is a skin, not
    // just colors: DIN-style type, square corners, hairlines (see style.css).
    id: 'mission-control', name: 'Mission Control', dark: true, skin: 'mission',
    accent: '#ffffff', need: '#ff5a1f', brand: '#ffffff',
    ui: { panel: '#060606', raise: '#161616', line: '#242424', muted: '#8b9099', faint: '#5b6069', idle: '#8b9099' },
    term: ansi('#000000', '#e6e8eb',
      ['#1c1d1f', '#ff4d3d', '#8fd19e', '#f2c14e', '#4da3ff', '#b88cff', '#5fd4e8', '#c9cdd3'],
      ['#4a4e55', '#ff7a6b', '#b2e3bd', '#f7d27f', '#80bdff', '#d0b2ff', '#8fe3f0', '#ffffff']),
  },
  {
    // Picture themes: the image fills the window; the terminal sits on a
    // veil of the background color (veil = its opacity) so text stays crisp.
    id: 'mt-fuji', name: 'Mt. Fuji', dark: true, image: '/themes/fuji.svg', veil: 0.72,
    accent: '#f2a07b', need: '#ffd27a', brand: '#f6cf9f',
    ui: { panel: '#1b1f3d', raise: '#2a2f55', line: '#363c68', muted: '#a8a3c2', faint: '#726d93', idle: '#a8a3c2' },
    term: ansi('#141832', '#ece8f3',
      ['#2a2f55', '#ef7a85', '#a6d6a0', '#f4c88a', '#8aa7f0', '#c79ae8', '#84d0d8', '#d9d5e6'],
      ['#5a5f8a', '#f799a2', '#c0e5bb', '#f8d9a8', '#a9bff5', '#d8b5ef', '#a5dfe5', '#ffffff']),
  },
  {
    id: 'jellyfish', name: 'Jellyfish', dark: true, image: '/themes/jellyfish.svg', veil: 0.7,
    accent: '#6fe3ea', need: '#ff86c8', brand: '#ffb0e6',
    ui: { panel: '#062033', raise: '#0d3048', line: '#153d57', muted: '#8fb6c2', faint: '#5a7f8d', idle: '#8fb6c2' },
    term: ansi('#041627', '#dff6f8',
      ['#0d3048', '#ff7a90', '#7ee0b0', '#f2d98a', '#6fb6ff', '#e59af0', '#6fe3ea', '#cfe6ec'],
      ['#3f6a80', '#ff9db0', '#a4ecc9', '#f7e5ae', '#98caff', '#efbaf5', '#9ceff3', '#ffffff']),
  },
  {
    id: 'dracula', name: 'Dracula', dark: true, accent: '#bd93f9', need: '#ffb86c',
    term: ansi('#282a36', '#f8f8f2',
      ['#21222c', '#ff5555', '#50fa7b', '#f1fa8c', '#bd93f9', '#ff79c6', '#8be9fd', '#f8f8f2'],
      ['#6272a4', '#ff6e6e', '#69ff94', '#ffffa5', '#d6acff', '#ff92df', '#a4ffff', '#ffffff']),
  },
  {
    id: 'nord', name: 'Nord', dark: true, accent: '#88c0d0', need: '#d08770',
    term: ansi('#2e3440', '#d8dee9',
      ['#3b4252', '#bf616a', '#a3be8c', '#ebcb8b', '#81a1c1', '#b48ead', '#88c0d0', '#e5e9f0'],
      ['#4c566a', '#bf616a', '#a3be8c', '#ebcb8b', '#81a1c1', '#b48ead', '#8fbcbb', '#eceff4']),
  },
  {
    id: 'tokyo-night', name: 'Tokyo Night', dark: true, accent: '#7aa2f7', need: '#ff9e64',
    term: ansi('#1a1b26', '#c0caf5',
      ['#15161e', '#f7768e', '#9ece6a', '#e0af68', '#7aa2f7', '#bb9af7', '#7dcfff', '#a9b1d6'],
      ['#414868', '#f7768e', '#9ece6a', '#e0af68', '#7aa2f7', '#bb9af7', '#7dcfff', '#c0caf5']),
  },
  {
    id: 'catppuccin-mocha', name: 'Catppuccin Mocha', dark: true, accent: '#cba6f7', need: '#fab387',
    term: ansi('#1e1e2e', '#cdd6f4',
      ['#45475a', '#f38ba8', '#a6e3a1', '#f9e2af', '#89b4fa', '#f5c2e7', '#94e2d5', '#bac2de'],
      ['#585b70', '#f38ba8', '#a6e3a1', '#f9e2af', '#89b4fa', '#f5c2e7', '#94e2d5', '#a6adc8']),
  },
  {
    id: 'one-dark', name: 'One Dark', dark: true, accent: '#61afef', need: '#d19a66',
    term: ansi('#282c34', '#abb2bf',
      ['#282c34', '#e06c75', '#98c379', '#e5c07b', '#61afef', '#c678dd', '#56b6c2', '#abb2bf'],
      ['#5c6370', '#e06c75', '#98c379', '#e5c07b', '#61afef', '#c678dd', '#56b6c2', '#ffffff']),
  },
  {
    id: 'gruvbox-dark', name: 'Gruvbox Dark', dark: true, accent: '#fabd2f', need: '#fe8019',
    term: ansi('#282828', '#ebdbb2',
      ['#282828', '#cc241d', '#98971a', '#d79921', '#458588', '#b16286', '#689d6a', '#a89984'],
      ['#928374', '#fb4934', '#b8bb26', '#fabd2f', '#83a598', '#d3869b', '#8ec07c', '#ebdbb2']),
  },
  {
    id: 'solarized-dark', name: 'Solarized Dark', dark: true, accent: '#268bd2', need: '#cb4b16',
    term: ansi('#002b36', '#839496',
      ['#073642', '#dc322f', '#859900', '#b58900', '#268bd2', '#d33682', '#2aa198', '#eee8d5'],
      ['#586e75', '#cb4b16', '#859900', '#b58900', '#268bd2', '#6c71c4', '#2aa198', '#fdf6e3']),
  },
  {
    id: 'catppuccin-latte', name: 'Catppuccin Latte', dark: false, accent: '#8839ef', need: '#fe640b',
    term: ansi('#eff1f5', '#4c4f69',
      ['#5c5f77', '#d20f39', '#40a02b', '#df8e1d', '#1e66f5', '#ea76cb', '#179299', '#acb0be'],
      ['#6c6f85', '#d20f39', '#40a02b', '#df8e1d', '#1e66f5', '#ea76cb', '#179299', '#bcc0cc']),
  },
  {
    id: 'solarized-light', name: 'Solarized Light', dark: false, accent: '#268bd2', need: '#cb4b16',
    term: ansi('#fdf6e3', '#657b83',
      ['#073642', '#dc322f', '#859900', '#b58900', '#268bd2', '#d33682', '#2aa198', '#eee8d5'],
      ['#586e75', '#cb4b16', '#859900', '#b58900', '#268bd2', '#6c71c4', '#2aa198', '#002b36']),
  },
  {
    id: 'github-light', name: 'GitHub Light', dark: false, accent: '#0969da', need: '#bc4c00',
    term: ansi('#ffffff', '#1f2328',
      ['#24292f', '#cf222e', '#116329', '#4d2d00', '#0969da', '#8250df', '#1b7c83', '#6e7781'],
      ['#57606a', '#a40e26', '#1a7f37', '#633c01', '#218bff', '#a475f9', '#3192aa', '#8c959f']),
  },
];

// "gecko" follows the system's light/dark preference.
export const AUTO = 'gecko';

export function resolveTheme(id) {
  if (!id || id === AUTO) return matchMedia('(prefers-color-theme: dark)').matches ? geckoDark : geckoLight;
  return THEMES.find((s) => s.id === id) || geckoDark;
}

function hex(c) {
  const n = parseInt(c.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

// mix(a, b, t): a blended toward b by t (0..1).
function mix(a, b, t) {
  const x = hex(a), y = hex(b);
  return '#' + x.map((v, i) => Math.round(v + (y[i] - v) * t).toString(16).padStart(2, '0')).join('');
}

// UI tokens for a theme, derived where the theme doesn't set them.
export function uiTokens(s) {
  const bg = s.term.background, fg = s.term.foreground;
  const k = s.dark ? 1 : 1.3;
  return {
    bg,
    panel: s.ui?.panel ?? mix(bg, fg, 0.04 * k),
    raise: s.ui?.raise ?? mix(bg, fg, 0.09 * k),
    line: s.ui?.line ?? mix(bg, fg, 0.15 * k),
    text: fg,
    muted: s.ui?.muted ?? mix(bg, fg, 0.62),
    faint: s.ui?.faint ?? mix(bg, fg, 0.42),
    accent: s.accent,
    need: s.need,
    idle: s.ui?.idle ?? mix(bg, fg, 0.55),
    red: s.term.red,
    'on-accent': s.dark ? bg : '#ffffff',
  };
}

function alpha(c, a) {
  return a >= 1 ? c : c + Math.round(a * 255).toString(16).padStart(2, '0');
}

// Applies a theme to the page and returns the matching xterm.js theme.
// opacity < 1 (desktop app only) makes the big surfaces see-through.
export function paintTheme(s, opacity = 1) {
  const root = document.documentElement;
  const ui = uiTokens(s);
  for (const [k, v] of Object.entries(ui)) root.style.setProperty('--' + k, v);
  // Each region gets exactly one translucent layer (stacked layers would
  // add up to opaque): the sidebar and the main area. Everything inside the
  // main area, the terminal included, is transparent; highlights are light
  // overlays.
  const see = opacity < 1;
  root.style.setProperty('--surface', alpha(ui.bg, opacity));
  root.style.setProperty('--surface-term', see ? 'transparent' : ui.bg);
  root.style.setProperty('--surface-panel', alpha(ui.panel, opacity));
  root.style.setProperty('--surface-raise', see ? alpha(ui.raise, 0.55) : ui.raise);
  root.style.colorScheme = s.dark ? 'dark' : 'light';
  // Skins change type and shape too, through a class on <html>.
  for (const c of [...root.classList]) if (c.startsWith('skin-')) root.classList.remove(c);
  if (s.skin) root.classList.add('skin-' + s.skin);
  // Picture themes: the image behind everything, a veil under the terminal.
  root.classList.toggle('has-image', !!s.image);
  if (s.image) {
    root.style.setProperty('--bg-image', `url("${s.image}")`);
    root.style.setProperty('--veil', alpha(ui.bg, s.veil ?? 0.72));
    root.style.setProperty('--surface-term', 'transparent');
  } else {
    root.style.removeProperty('--bg-image');
  }
  root.style.setProperty('--brand', s.brand || (s.dark ? '#7ccb3a' : '#3d8c1c')); // the foot in the start animation
  document.querySelector('meta[name=theme-color]')?.setAttribute('content', s.term.background);
  const fg = s.term.foreground;
  return {
    ...s.term,
    background: opacity < 1 || s.image ? '#00000000' : s.term.background,
    cursor: s.accent,
    cursorAccent: s.term.background,
    selectionBackground: s.accent + '4d',
    selectionInactiveBackground: s.accent + '26',
    scrollbarSliderBackground: fg + '22',
    scrollbarSliderHoverBackground: fg + '44',
  };
}

// A few color chips to recognise a theme in the picker.
export function swatch(s) {
  if (s.image) return `<span class="swatch pic" aria-hidden="true" style="background-image:url('${s.image}')"></span>`;
  const t = s.term;
  const chips = [t.background, s.accent, t.red, t.green, t.yellow, t.blue, t.magenta, t.foreground];
  return `<span class="swatch" aria-hidden="true">${chips.map((c) => `<i style="background:${c}"></i>`).join('')}</span>`;
}
