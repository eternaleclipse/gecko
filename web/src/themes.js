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
    // Picture themes: a CC0 photo fills the window (see public/themes/CREDITS.md).
    // `veil` is how strongly the theme color tints the photo under the
    // terminal, so text stays readable.
    id: 'mt-fuji', name: 'Mt. Fuji', dark: true, image: '/themes/fuji.jpg', position: 'center 38%', veil: 0.6,
    accent: '#f5a8a0', need: '#ffd27a', brand: '#f5c4b0',
    ui: { panel: '#1a2140', raise: '#2a3258', line: '#384170', muted: '#aab0cc', faint: '#737a9c', idle: '#aab0cc' },
    term: ansi('#121935', '#eef0f7',
      ['#2a3258', '#f2838c', '#a9d8a8', '#f6cb8f', '#8fb0f2', '#d0a2e6', '#8ad3dc', '#dcdfee'],
      ['#5b6390', '#f8a2a9', '#c3e7c1', '#f9dcaf', '#aec6f6', '#e0bff0', '#abe2e8', '#ffffff']),
  },
  {
    id: 'jellyfish', name: 'Jellyfish', dark: true, image: '/themes/jellyfish.jpg', position: '30% center', veil: 0.55,
    accent: '#f5b45e', need: '#ff7f73', brand: '#f5b45e',
    ui: { panel: '#06222c', raise: '#0d3442', line: '#164252', muted: '#93b9c2', faint: '#5d828d', idle: '#93b9c2' },
    term: ansi('#041b24', '#e2f3f5',
      ['#0d3442', '#ff7f73', '#7fdcb0', '#f5c56e', '#6fb3e8', '#d99ae6', '#68d8de', '#cfe4e8'],
      ['#3f6a78', '#ffa197', '#a5eacb', '#f8d696', '#98c8ef', '#e7b9ef', '#95e6ea', '#ffffff']),
  },
  {
    // Light: petal white, plum text, blossom pink. Rounded and soft.
    id: 'sakura', name: 'Sakura', dark: false, skin: 'sakura', image: '/themes/sakura.jpg', position: 'center 35%', veil: 0.87,
    accent: '#d93a78', need: '#d9781f', brand: '#f47ba8',
    ui: { panel: '#ffeef3', raise: '#ffdce8', line: '#f2c2d3', muted: '#86576b', faint: '#b38799', idle: '#86576b' },
    term: ansi('#fff6f9', '#3a1f2c',
      ['#3a1f2c', '#b82a45', '#3d7a2c', '#8a5a00', '#3f66b0', '#b0408a', '#2c8585', '#c9b0bb'],
      ['#7a5466', '#d9536b', '#629c4c', '#b8851f', '#5880c4', '#c45a9e', '#3f9a9a', '#ffffff']),
  },
  {
    // Neon on night violet: hot magenta, electric cyan, warning yellow.
    id: 'cyberpunk', name: 'Cyberpunk', dark: true, skin: 'cyberpunk', image: '/themes/cyberpunk.jpg', position: 'center', veil: 0.62,
    accent: '#ff2a6d', need: '#f9f002', brand: '#ff2a6d',
    termFont: '"Share Tech Mono", ui-monospace, monospace',
    ui: { panel: '#120424', raise: '#22093d', line: '#3a0f5e', muted: '#b494d8', faint: '#7a5a9c', idle: '#b494d8' },
    term: ansi('#0b0215', '#f2e9ff',
      ['#1a0833', '#ff2a6d', '#3cff9e', '#f9f002', '#3d7bff', '#d16bff', '#05d9e8', '#e0d4f5'],
      ['#4a2a6e', '#ff6b98', '#7dffbf', '#fbf66b', '#7aa4ff', '#e3a0ff', '#6ff0fa', '#ffffff']),
  },
  {
    // Shinjuku at night: signal red on deep navy, heavy display type.
    id: 'neo-tokyo', name: 'Neo-Tokyo', dark: true, skin: 'neotokyo', image: '/themes/neo-tokyo.jpg', position: 'center 60%', veil: 0.64,
    accent: '#ff3b3b', need: '#ffd166', brand: '#ff3b3b',
    ui: { panel: '#0b1124', raise: '#172042', line: '#232f57', muted: '#9aa6c8', faint: '#5d6890', idle: '#9aa6c8' },
    term: ansi('#070b18', '#eaf0ff',
      ['#172042', '#ff3b3b', '#56e39f', '#ffd166', '#4d8dff', '#ff6bd6', '#4dd9ff', '#d5dcf0'],
      ['#3d4a78', '#ff7070', '#8aefbf', '#ffe09a', '#83b0ff', '#ff9be3', '#86e6ff', '#ffffff']),
  },
  {
    // Race orange on mud, lime for attention, number-plate tabs.
    id: 'dirt-bike', name: 'Dirt Bike', dark: true, skin: 'dirtbike', image: '/themes/dirt-bike.jpg', position: 'center 40%', veil: 0.66,
    accent: '#ff6a00', need: '#c6ff00', brand: '#ff6a00',
    ui: { panel: '#1c1612', raise: '#2c231b', line: '#3b2f24', muted: '#b8a48f', faint: '#7d6a57', idle: '#b8a48f' },
    term: ansi('#15110d', '#f4ece2',
      ['#2c231b', '#ff4d2e', '#9be564', '#ffc21a', '#4fa3ff', '#e06bd8', '#3fd6c6', '#e2d6c6'],
      ['#5a4a3b', '#ff7d63', '#bdf08f', '#ffd55c', '#83beff', '#ec97e6', '#76e4d8', '#ffffff']),
  },
  {
    // Alpenglow on night slate, glacier blue, classic outdoor-poster type.
    id: 'summit', name: 'Summit', dark: true, skin: 'summit', image: '/themes/summit.jpg', position: '70% 40%', veil: 0.6,
    accent: '#ff9a52', need: '#ffcf5c', brand: '#ff9a52',
    ui: { panel: '#101a24', raise: '#1c2a38', line: '#2a3b4c', muted: '#a3b3c2', faint: '#687a8c', idle: '#a3b3c2' },
    term: ansi('#0c141c', '#eef3f7',
      ['#1c2a38', '#ef6f6c', '#8fd19e', '#ffcf5c', '#7cc4ff', '#c7a0e8', '#79d6d3', '#d9e2ea'],
      ['#4a5c6e', '#f59492', '#b0e0bb', '#ffdd8a', '#a6d6ff', '#d8bdf0', '#a0e3e1', '#ffffff']),
  },
  {
    // Walnut and oak, safety red, slab serif, a workshop mono.
    id: 'wood-shop', name: 'Wood Shop', dark: true, skin: 'woodshop', image: '/themes/wood-shop.jpg', position: 'center 45%', veil: 0.66,
    accent: '#e8a24a', need: '#e4572e', brand: '#e8a24a',
    termFont: '"IBM Plex Mono", ui-monospace, monospace',
    ui: { panel: '#21170f', raise: '#33241a', line: '#4a3424', muted: '#c2a68a', faint: '#8a6e55', idle: '#c2a68a' },
    term: ansi('#1a120c', '#f3e7d7',
      ['#33241a', '#e4572e', '#9cc069', '#e8b04a', '#7fa6c9', '#c98bb0', '#7fbfb1', '#e0d2bf'],
      ['#5e4634', '#ee8060', '#b8d590', '#f0c878', '#a3c0dc', '#dcaecb', '#a3d4c9', '#ffffff']),
  },
  {
    // Lantern red and broth gold on lacquer, playful rounded type, a noren tab.
    id: 'ramen', name: 'Ramen', dark: true, skin: 'ramen', image: '/themes/ramen.jpg', position: 'center', veil: 0.72, sideVeil: 0.8,
    accent: '#f2b33d', need: '#e63946', brand: '#e63946',
    ui: { panel: '#1d0f0a', raise: '#2e1811', line: '#47241a', muted: '#d0a98a', faint: '#8e6a55', idle: '#d0a98a' },
    term: ansi('#170c08', '#fbefe0',
      ['#2e1811', '#e63946', '#8ccf6e', '#f2b33d', '#6fa8dc', '#e07aa8', '#6cc7c2', '#ead9c6'],
      ['#5e3a2c', '#f06a75', '#afe095', '#f7cb72', '#98c1e8', '#eca2c4', '#95dad6', '#ffffff']),
  },
  {
    // Digital Watch: a total UI overhaul in the spirit of a classic resin
    // digital watch (skin-casio in style.css): black resin case, brushed
    // band, a grey-green LCD terminal with pixel type, a 7-segment watch face
    // with LIGHT / MODE / ALARM pushers, and the watch's printed label colors.
    // `lcd`: the terminal is drawn on a CSS LCD (it can light up).
    id: 'digital-watch', name: 'Digital Watch', dark: true, skin: 'casio', lcd: true,
    accent: '#f2c230', need: '#e0473c', brand: '#f2c230',
    termFont: '"VT323", ui-monospace, monospace',
    termSize: 1.45, // VT323 draws small; scale the terminal font up
    ui: { bg: '#141414', text: '#e8e8e8', panel: '#181818', raise: '#2a2a2a', line: '#3a3a3a', muted: '#a8a8a8', faint: '#6c6c6c', idle: '#a8a8a8' },
    term: ansi('#b4bea0', '#121a10',
      ['#121a10', '#8a1f14', '#2a5418', '#6b5200', '#1d3f7a', '#6b2a6b', '#1d5a5a', '#3d4637'],
      ['#3d4637', '#a8281b', '#367020', '#8a6a00', '#2a54a0', '#8a3a8a', '#267575', '#000000']),
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
    bg: s.ui?.bg ?? bg,
    panel: s.ui?.panel ?? mix(bg, fg, 0.04 * k),
    raise: s.ui?.raise ?? mix(bg, fg, 0.09 * k),
    line: s.ui?.line ?? mix(bg, fg, 0.15 * k),
    text: s.ui?.text ?? fg,
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
    root.style.setProperty('--bg-position', s.position || 'center');
    root.style.setProperty('--veil', alpha(ui.bg, s.veil ?? 0.7));
    root.style.setProperty('--side-tint', alpha(ui.panel, s.sideVeil ?? 0.5));
    // The window's opacity over the desktop applies to the photo too.
    root.style.setProperty('--win-opacity', String(opacity));
    root.style.setProperty('--surface-term', 'transparent');
  } else {
    root.style.removeProperty('--bg-image');
  }
  root.style.setProperty('--brand', s.brand || (s.dark ? '#7ccb3a' : '#3d8c1c')); // the foot in the start animation
  document.querySelector('meta[name=theme-color]')?.setAttribute('content', s.term.background);
  const fg = s.term.foreground;
  return {
    ...s.term,
    background: opacity < 1 || s.image || s.lcd ? '#00000000' : s.term.background,
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
