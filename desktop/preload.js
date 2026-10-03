// The only bridge between the page and the native shell.
const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('geckoDesktop', {
  platform: process.platform,
  // Background transparency: real on macOS/Linux, acrylic on Windows.
  transparent: true,
  setBounds: (b) => ipcRenderer.invoke('gecko:set-bounds', b),
});
