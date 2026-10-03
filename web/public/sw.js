// Gecko's service worker exists so the app can be installed (desktop app
// window, phone home screen). It deliberately caches nothing: the daemon
// is the source of truth and the bundle is tiny.
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (e) => e.waitUntil(self.clients.claim()));
self.addEventListener('fetch', () => {});
