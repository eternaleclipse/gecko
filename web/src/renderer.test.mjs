// The desktop window reloads its page when the renderer dies (the OOM
// killer, a crash), so it never sits on a frozen frame: desktop/main.js,
// rendererDelay.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import Module, { createRequire } from 'node:module';

// desktop/main.js with a stand-in for Electron (as in transparency.test.mjs).
function loadMain() {
  const app = new Proxy({}, { get: (_, k) => (k === 'requestSingleInstanceLock' ? () => false : k === 'commandLine' ? { appendSwitch() {} } : () => {}) });
  const electron = { app, BrowserWindow: {}, Menu: {}, dialog: {}, ipcMain: { handle() {}, on() {} }, shell: {}, screen: {} };
  const load = Module._load;
  Module._load = function (req, ...rest) { return req === 'electron' ? electron : load.call(this, req, ...rest); };
  try {
    return createRequire(import.meta.url)('../../desktop/main.js');
  } finally {
    Module._load = load;
  }
}
const { rendererDelay } = loadMain();

function run(times) {
  let state = { n: 0 };
  return times.map((t) => {
    let delay;
    ({ delay, state } = rendererDelay(state, t));
    return delay;
  });
}

test('the first crash reloads at once', () => {
  assert.deepEqual(run([1000]), [0]);
});

test('crashes in a row back off and stay at the longest wait', () => {
  // Each crash comes right after the previous reload.
  assert.deepEqual(run([0, 100, 1200, 6300, 36400, 66500]), [0, 1000, 5000, 30000, 30000, 30000]);
});

test('a page that stayed up for a minute starts over', () => {
  assert.deepEqual(run([0, 100, 1200 + 60000]), [0, 1000, 0]);
});
