// Gecko desktop: a native window around the Gecko web client. It exists for
// what a browser window can't do: a see-through background and no browser
// chrome in the way. Everything else is the
// same client the daemon serves.
//
//   gecko-desktop --url=http://127.0.0.1:7681/?token=... [--size=W,H] [--position=X,Y]
//
// `gecko open` starts it with the right URL, size and position. The packaged
// apps (dmg, AppImage, Windows setup) carry their own gecko binary and are
// started without --url: then this finds the user's running daemon, or
// starts the bundled one (detached, so sessions outlive the window), and
// opens it. See docs/desktop-packages.md.

const { app, BrowserWindow, Menu, dialog, ipcMain, shell, screen } = require('electron');
const { execFile } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const arg = (name) => {
  const a = process.argv.find((x) => x.startsWith(`--${name}=`));
  return a ? a.slice(name.length + 3) : '';
};
const pair = (s) => (/^-?\d+,-?\d+$/.test(s) ? s.split(',').map(Number) : null);

const mac = process.platform === 'darwin';
const win32 = process.platform === 'win32';
const linux = process.platform === 'linux';
let url = arg('url');
// The web UI switches to its phone layout below 500px of height (a phone in
// landscape), so the window's content never gets that short here.
const MIN_HEIGHT = 540;
let size = pair(arg('size')) || [900, 600];
let pos = pair(arg('position'));

// Which build this is, like the Android app's: "a1b2c3d · Oct 17 02:15"
// (commit, -dirty with uncommitted changes, and build time in local time).
// build-id.json is written when the app is packaged.
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
function formatBuildId(commit, time) {
  const c = commit || 'unknown';
  const t = new Date(time || '');
  if (Number.isNaN(t.getTime())) return c;
  const two = (n) => String(n).padStart(2, '0');
  return `${c} · ${MONTHS[t.getMonth()]} ${t.getDate()} ${two(t.getHours())}:${two(t.getMinutes())}`;
}
const build = (() => {
  try {
    return JSON.parse(fs.readFileSync(path.join(__dirname, 'build-id.json'), 'utf8'));
  } catch {
    return {};
  }
})();
const buildId = formatBuildId(build.commit, build.time);
process.env.GECKO_DESKTOP_BUILD = buildId; // for preload.js

// The package is named "gecko" so the X11 WM_CLASS is "gecko" and the dock
// matches the window to gecko.desktop.

// Linux needs an ARGB visual for a see-through window.
if (linux) app.commandLine.appendSwitch('enable-transparent-visuals');

const geckoHome = () => process.env.GECKO_HOME || path.join(os.homedir(), '.gecko-terminal');
// Another GECKO_HOME is another daemon, so its window is a separate app
// instance too (its own single-instance lock and browser profile).
if (process.env.GECKO_HOME) app.setPath('userData', path.join(process.env.GECKO_HOME, 'state', 'desktop'));

// The gecko binary inside the app: Contents/Resources on macOS, resources\
// on Windows, usr/bin in the AppImage (whose launcher passes a copy that
// outlives the AppImage's mount as GECKO_BIN). Unpackaged: gecko on PATH.
function bundledGecko() {
  if (process.env.GECKO_BIN) return process.env.GECKO_BIN;
  if (!app.isPackaged) return 'gecko';
  if (mac) return path.join(process.resourcesPath, 'gecko');
  if (win32) return path.join(process.resourcesPath, 'gecko.exe');
  return path.join(process.resourcesPath, '..', 'usr', 'bin', 'gecko');
}

// A daemon outlives the app, so it must not run from somewhere that goes
// away or gets replaced under it: a dmg or a translocated copy on macOS, or
// the install dir on Windows (an update would have to stop it to replace
// gecko.exe). Those run a copy in GECKO_HOME/state/bin, one per build.
function stableGecko(p) {
  const moving = win32 || (mac && (p.startsWith('/Volumes/') || p.includes('/AppTranslocation/')));
  if (!app.isPackaged || !moving) return p;
  try {
    const data = fs.readFileSync(p);
    const sum = crypto.createHash('sha256').update(data).digest('hex').slice(0, 12);
    const dir = path.join(geckoHome(), 'state', 'bin');
    const dst = path.join(dir, `gecko-${sum}${win32 ? '.exe' : ''}`);
    if (!fs.existsSync(dst)) {
      fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
      const tmp = `${dst}.${process.pid}.tmp`;
      fs.writeFileSync(tmp, data, { mode: 0o755 });
      fs.renameSync(tmp, dst);
      // Older copies: deleting one that still runs a daemon fails on
      // Windows, which is what we want; elsewhere keep the last few.
      const old = fs
        .readdirSync(dir)
        .filter((f) => /^gecko-[0-9a-f]{12}(\.exe)?$/.test(f) && path.join(dir, f) !== dst)
        .map((f) => ({ f: path.join(dir, f), t: fs.statSync(path.join(dir, f)).mtimeMs }))
        .sort((a, b) => b.t - a.t)
        .slice(win32 ? 0 : 2);
      for (const o of old) fs.rmSync(o.f, { force: true });
    }
    return dst;
  } catch (e) {
    console.error('gecko-desktop: could not copy gecko out of the app:', e.message);
    return p;
  }
}

// The environment for gecko and the daemon it may start: what the user's
// shells should see, without what the AppImage launcher and Electron add.
function cleanEnv() {
  const env = { ...process.env };
  const appdir = env.APPDIR;
  for (const k of Object.keys(env)) {
    if (/^(ELECTRON_|CHROME_|GECKO_DESKTOP_|GECKO_SANDBOX_)/.test(k) || k === 'GECKO_BIN') delete env[k];
  }
  if (env.ORIGINAL_XDG_CURRENT_DESKTOP) env.XDG_CURRENT_DESKTOP = env.ORIGINAL_XDG_CURRENT_DESKTOP;
  delete env.ORIGINAL_XDG_CURRENT_DESKTOP;
  if (appdir) {
    for (const k of ['APPDIR', 'APPIMAGE', 'ARGV0', 'OWD']) delete env[k];
    for (const k of ['PATH', 'LD_LIBRARY_PATH', 'XDG_DATA_DIRS', 'GSETTINGS_SCHEMA_DIR', 'PYTHONPATH', 'PERLLIB', 'QT_PLUGIN_PATH']) {
      if (env[k] === undefined) continue;
      const v = env[k].split(':').filter((x) => x && !x.startsWith(appdir)).join(':');
      if (v) env[k] = v;
      else delete env[k];
    }
  }
  return env;
}

// `gecko url` does what `gecko open` does: connects to this user's daemon
// (GECKO_HOME's socket, token and web port), starting it detached when none
// is running, and prints the URL with the token.
function daemonURL(gecko) {
  return new Promise((resolve, reject) => {
    execFile(gecko, ['url'], { env: cleanEnv(), timeout: 30000, windowsHide: true, cwd: os.homedir() }, (err, stdout, stderr) => {
      const u = String(stdout).split(/\r?\n/)[0].trim();
      if (!err && /^https?:\/\//.test(u)) return resolve(u);
      reject(new Error((String(stderr).trim() || (err && err.message) || 'no URL from gecko url') + `\n\n(${gecko})`));
    });
  });
}

// The theme in GECKO_HOME/settings.json, so a window whose browser profile
// has none cached yet (the first launch) paints it from the start.
function savedTheme() {
  try {
    const t = JSON.parse(fs.readFileSync(path.join(geckoHome(), 'settings.json'), 'utf8')).theme;
    return typeof t === 'string' && /^[\w-]+$/.test(t) ? t : '';
  } catch {
    return '';
  }
}

// Where the last fitted window ended up (as `gecko open` does).
function savedWindow() {
  try {
    const b = JSON.parse(fs.readFileSync(path.join(geckoHome(), 'state', 'window.json'), 'utf8'));
    if (!(b.W > 0 && b.H > 0 && b.SW > 0 && b.SH > 0)) return null;
    return { size: [b.W, b.H], pos: [b.SX + Math.max(0, Math.floor((b.SW - b.W) / 2)), b.SY + Math.max(0, Math.floor((b.SH - b.H) / 2))] };
  } catch {
    return null;
  }
}

// The AppImage launcher falls back to --no-sandbox where the system blocks
// the user namespaces Chromium's sandbox needs; say so once.
function sandboxNote(w) {
  if (process.env.GECKO_SANDBOX_FALLBACK !== '1') return;
  const msg =
    'This system restricts unprivileged user namespaces (Ubuntu 23.10 and later do by default), ' +
    "so Gecko's window runs without Chromium's sandbox. It only ever shows your own Gecko. " +
    'To turn the sandbox on, see "Linux sandbox" in docs/desktop-packages.md.';
  console.error('gecko-desktop: ' + msg);
  const mark = path.join(geckoHome(), 'state', 'desktop-sandbox-note');
  if (fs.existsSync(mark)) return;
  try {
    fs.mkdirSync(path.dirname(mark), { recursive: true, mode: 0o700 });
    fs.writeFileSync(mark, msg + '\n');
  } catch {}
  dialog.showMessageBox(w, { type: 'info', title: 'Gecko', message: 'Running without the Chromium sandbox', detail: msg, buttons: ['OK'] });
}

// macOS: puts the bundled gecko on PATH as /usr/local/bin/gecko.
async function installCLI() {
  const target = bundledGecko();
  if (target.startsWith('/Volumes/') || target.includes('/AppTranslocation/')) {
    await dialog.showMessageBox({
      type: 'warning',
      message: 'Move Gecko to Applications first',
      detail: 'The command line tool links to the gecko inside the app, so the app needs to stay where it is. Drag Gecko into Applications, open it from there, and try again.',
    });
    return;
  }
  const q = (s) => `'${s.replace(/'/g, `'\\''`)}'`;
  const sh = `mkdir -p /usr/local/bin && ln -sf ${q(target)} /usr/local/bin/gecko`;
  const script = `do shell script ${JSON.stringify(sh)} with administrator privileges`;
  execFile('/usr/bin/osascript', ['-e', script], (err, _out, stderr) => {
    if (err && /User canceled|-128/.test(String(stderr))) return;
    dialog.showMessageBox(
      err
        ? { type: 'error', message: 'Could not install the command line tool', detail: String(stderr || err.message) }
        : { type: 'info', message: 'Installed /usr/local/bin/gecko', detail: `It runs ${target}. Try \`gecko ls\` in a terminal.` },
    );
  });
}

function menu() {
  if (!mac) return null; // Linux/Windows: no menu, so every shortcut reaches Gecko
  // macOS needs a menu for Cmd+V and Cmd+Q. Copy, Cmd+W etc. are left to
  // Gecko, which knows about terminal selections and tabs.
  const geckoMenu = [{ role: 'about' }, { type: 'separator' }];
  if (app.isPackaged) geckoMenu.push({ label: 'Install command line tool', click: installCLI }, { type: 'separator' });
  geckoMenu.push({ role: 'hide' }, { role: 'hideOthers' }, { type: 'separator' }, { role: 'quit' });
  return Menu.buildFromTemplate([
    { label: 'Gecko', submenu: geckoMenu },
    { label: 'Edit', submenu: [{ label: 'Paste', accelerator: 'Cmd+V', click: (_, w) => w?.webContents.paste() }] },
    { label: 'Window', submenu: [{ role: 'minimize' }, { role: 'zoom' }, { role: 'togglefullscreen' }] },
  ]);
}

// The Windows caption buttons sit over the right end of the tab bar, as
// tall as it is (style.css, #tabbar).
const TITLE_BAR_HEIGHT = 38;

// See-through background: a transparent window on every platform, and the
// page decides how opaque each surface is (the opacity setting, see
// paintTheme in web/src/themes.js). Not macOS vibrancy or Windows acrylic:
// those put a blurred system material between the page and the desktop, so
// nothing behind the window showed through, at any opacity. They are
// plain gray in a VM, with Reduce transparency or transparency effects
// off, and acrylic goes solid whenever the window isn't focused.
// A transparent window on Windows can't have the native frame, so its
// title bar is hidden and Windows draws the caption buttons over the page
// (titleBarOverlay), in the theme's text color (gecko:title-bar).
// transparency.test.mjs keeps this as it is.
function windowStyle(platform) {
  const style = { transparent: true, backgroundColor: '#00000000' };
  if (platform === 'darwin') style.titleBarStyle = 'hiddenInset';
  if (platform === 'win32') {
    style.titleBarStyle = 'hidden';
    style.titleBarOverlay = { color: '#00000000', symbolColor: '#c0caf5', height: TITLE_BAR_HEIGHT };
  }
  return style;
}

function createWindow() {
  const w = new BrowserWindow({
    width: size[0],
    height: Math.max(size[1], MIN_HEIGHT),
    minHeight: MIN_HEIGHT,
    useContentSize: true,
    ...(pos ? { x: pos[0], y: pos[1] } : { center: true }),
    show: false,
    title: 'Gecko',
    icon: path.join(__dirname, 'icon.png'),
    autoHideMenuBar: true,
    ...windowStyle(process.platform),
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      sandbox: true,
      spellcheck: false,
      autoplayPolicy: 'no-user-gesture-required', // the startup chime
      additionalArguments: savedTheme() ? [`--gecko-theme=${savedTheme()}`] : [],
    },
  });
  // Show the window once the page has painted its theme (the preload says
  // so), not at its first paint: that one still has the default colors, a
  // light flash on a light system. Gecko's own page always reports; the
  // timeout covers anything else (an error page, a remote that's down).
  let ready = false, painted = false, shown = false;
  const show = () => {
    if (shown || !ready || !painted || w.isDestroyed()) return;
    shown = true;
    w.show();
  };
  w.webContents.ipc.once('gecko:painted', () => { painted = true; show(); });
  w.once('ready-to-show', () => {
    ready = true;
    show();
    setTimeout(() => { painted = true; show(); }, 2000);
  });
  // Links open in the real browser; the window only ever shows Gecko.
  w.webContents.setWindowOpenHandler(({ url: u }) => {
    if (/^https?:/.test(u)) shell.openExternal(u);
    return { action: 'deny' };
  });
  const origin = new URL(url).origin;
  w.webContents.on('will-navigate', (ev, u) => {
    if (new URL(u).origin !== origin) {
      ev.preventDefault();
      shell.openExternal(u);
    }
  });
  watchRenderer(w);
  w.loadURL(url);
  return w;
}

// The page can die under the window: the kernel's OOM killer picks the
// renderer first (Chromium gives it a high oom_score_adj), and a GPU or
// renderer crash does the same. The window then keeps its last frame, tabs
// that look alive but never change, until Gecko is restarted. Reload the
// page instead: the sessions live in the daemon, so nothing is lost. Each
// reload in a row waits longer (rendererDelay), so a page that crashes as
// soon as it loads can't spin.
const RELOAD_DELAYS = [0, 1000, 5000, 30000];
const STABLE_MS = 60000;
// state: { n: reloads in a row, at: when the last one happened }. A page
// that stayed up for STABLE_MS starts the count again.
function rendererDelay(state, now) {
  const n = state.at !== undefined && now - state.at < STABLE_MS ? state.n : 0;
  const delay = RELOAD_DELAYS[Math.min(n, RELOAD_DELAYS.length - 1)];
  return { delay, state: { n: n + 1, at: now + delay } };
}

function watchRenderer(w) {
  let state = { n: 0 };
  let timer = null;
  w.webContents.on('render-process-gone', (_ev, details) => {
    if (details.reason === 'clean-exit' || w.isDestroyed() || timer) return;
    let delay;
    ({ delay, state } = rendererDelay(state, Date.now()));
    console.error(`gecko: the page stopped (${details.reason}, exit ${details.exitCode}); reloading in ${delay / 1000}s`);
    timer = setTimeout(() => {
      timer = null;
      if (w.isDestroyed()) return;
      w.loadURL(url);
      if (!w.isVisible()) w.show();
    }, delay);
  });
}

// The page sizes the window so the terminal is exactly 80x24 (see the
// client's fitWindow), through this instead of window.resizeTo.
ipcMain.handle('gecko:set-bounds', (ev, b) => {
  const w = BrowserWindow.fromWebContents(ev.sender);
  if (!w || w.isMaximized() || w.isFullScreen()) return;
  const area = screen.getDisplayMatching(w.getBounds()).workArea;
  const width = Math.min(Math.max(320, Math.round(b.width)), area.width);
  const height = Math.min(Math.max(200, Math.round(b.height)), area.height);
  const x = Number.isFinite(b.x) ? Math.round(b.x) : area.x + Math.round((area.width - width) / 2);
  const y = Number.isFinite(b.y) ? Math.round(b.y) : area.y + Math.round((area.height - height) / 2);
  w.setBounds({ x, y, width, height });
});

// The anchor the window chose (Make anchor in the sidebar, see
// web/src/anchor.js): kept in the app's settings, per daemon (GECKO_HOME
// has its own profile), so the next launch opens on it.
const anchorFile = () => path.join(app.getPath('userData'), 'anchor.json');
ipcMain.on('gecko:anchor-get', (ev) => {
  try {
    ev.returnValue = JSON.parse(fs.readFileSync(anchorFile(), 'utf8')).path || '';
  } catch {
    ev.returnValue = '';
  }
});
ipcMain.on('gecko:anchor-set', (ev, s) => {
  try {
    const p = JSON.parse(String(s));
    const ok = Array.isArray(p) && p.length > 0 && p.length <= 4 && p.every((x) => typeof x === 'string' && /^[^\s/]{1,128}$/.test(x));
    if (ok) fs.writeFileSync(anchorFile(), JSON.stringify({ path: JSON.stringify(p) }));
    else fs.rmSync(anchorFile(), { force: true });
  } catch { /* not a path */ }
});

// Windows: the caption buttons follow the theme (preload's setTitleBar).
ipcMain.on('gecko:title-bar', (ev, c) => {
  const w = BrowserWindow.fromWebContents(ev.sender);
  if (!win32 || !w || !/^#[0-9a-f]{6}$/i.test(c?.symbolColor || '')) return;
  w.setTitleBarOverlay({ color: '#00000000', symbolColor: c.symbolColor, height: TITLE_BAR_HEIGHT });
});

// Without --url (the packaged app, double-clicked): find or start the daemon.
async function resolveURL() {
  if (url) return true;
  try {
    url = (await daemonURL(stableGecko(bundledGecko()))) + '&fit=80x24';
    const saved = savedWindow();
    if (saved) ({ size, pos } = saved);
    return true;
  } catch (e) {
    dialog.showErrorBox(
      'Gecko could not start',
      `Starting the Gecko daemon failed:\n\n${e.message}\n\nIts log is ${path.join(geckoHome(), 'state', 'daemon.log')}.`,
    );
    app.exit(1);
    return false;
  }
}

if (!app.requestSingleInstanceLock()) {
  app.quit();
} else {
  app.on('second-instance', () => {
    const w = BrowserWindow.getAllWindows()[0];
    if (w) {
      if (w.isMinimized()) w.restore();
      w.focus();
    }
  });
  app.setAboutPanelOptions({
    applicationName: 'Gecko',
    applicationVersion: buildId,
    version: app.getVersion(),
    copyright: 'Gecko contributors',
  });
  app.whenReady().then(async () => {
    Menu.setApplicationMenu(menu());
    if (!(await resolveURL())) return;
    // Linux compositors need a moment before a transparent window maps.
    setTimeout(() => {
      const w = createWindow();
      w.once('show', () => sandboxNote(w));
    }, linux ? 150 : 0);
  });
  app.on('window-all-closed', () => app.quit());
  // macOS: clicking the dock icon with no window open (after Cmd+W).
  app.on('activate', () => {
    if (url && BrowserWindow.getAllWindows().length === 0) createWindow();
  });
}

module.exports = { windowStyle, rendererDelay }; // for transparency.test.mjs and renderer.test.mjs
