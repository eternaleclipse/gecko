// Gecko desktop: a native window around the Gecko web client. It exists for
// what a browser window can't do: a see-through background (with blur on
// macOS/Windows) and no browser shortcuts in the way. Everything else is the
// same client the daemon serves.
//
//   gecko-desktop --url=http://127.0.0.1:7681/?token=... [--size=W,H] [--position=X,Y]
//
// `gecko open` starts it with the right URL, size and position.

const { app, BrowserWindow, Menu, ipcMain, shell, screen } = require('electron');
const path = require('node:path');

const arg = (name) => {
  const a = process.argv.find((x) => x.startsWith(`--${name}=`));
  return a ? a.slice(name.length + 3) : '';
};
const pair = (s) => (/^-?\d+,-?\d+$/.test(s) ? s.split(',').map(Number) : null);

const url = arg('url');
const size = pair(arg('size')) || [900, 500];
const pos = pair(arg('position'));
const mac = process.platform === 'darwin';
const win32 = process.platform === 'win32';

// The package is named "gecko" so the X11 WM_CLASS is "gecko" and the dock
// matches the window to gecko.desktop.

// Linux needs an ARGB visual for a see-through window.
if (process.platform === 'linux') app.commandLine.appendSwitch('enable-transparent-visuals');

if (!url) {
  console.error('usage: gecko-desktop --url=<gecko url>   (normally started by `gecko open`)');
  app.exit(2);
}

function menu() {
  if (!mac) return null; // Linux/Windows: no menu, so every shortcut reaches Gecko
  // macOS needs a menu for Cmd+V and Cmd+Q. Copy, Cmd+W etc. are left to
  // Gecko, which knows about terminal selections and tabs.
  return Menu.buildFromTemplate([
    { label: 'Gecko', submenu: [{ role: 'about' }, { type: 'separator' }, { role: 'hide' }, { role: 'hideOthers' }, { type: 'separator' }, { role: 'quit' }] },
    { label: 'Edit', submenu: [{ label: 'Paste', accelerator: 'Cmd+V', click: (_, w) => w?.webContents.paste() }] },
    { label: 'Window', submenu: [{ role: 'minimize' }, { role: 'zoom' }, { role: 'togglefullscreen' }] },
  ]);
}

function createWindow() {
  const w = new BrowserWindow({
    width: size[0],
    height: size[1],
    ...(pos ? { x: pos[0], y: pos[1] } : { center: true }),
    show: false,
    title: 'Gecko',
    icon: path.join(__dirname, 'icon.png'),
    autoHideMenuBar: true,
    // See-through background: real transparency on macOS/Linux, acrylic on
    // Windows 11 (which needs its native frame). The page decides how
    // opaque each surface is.
    transparent: !win32,
    backgroundColor: '#00000000',
    ...(mac ? { vibrancy: 'under-window', visualEffectState: 'active', titleBarStyle: 'hiddenInset' } : {}),
    ...(win32 ? { backgroundMaterial: 'acrylic' } : {}),
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      sandbox: true,
      spellcheck: false,
    },
  });
  w.once('ready-to-show', () => w.show());
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
  w.loadURL(url);
  return w;
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
  app.whenReady().then(() => {
    Menu.setApplicationMenu(menu());
    // Linux compositors need a moment before a transparent window maps.
    setTimeout(createWindow, process.platform === 'linux' ? 150 : 0);
  });
  app.on('window-all-closed', () => app.quit());
}
