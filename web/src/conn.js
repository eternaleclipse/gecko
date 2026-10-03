// Connection to the Gecko daemon: one WebSocket carrying JSON control
// messages and binary terminal frames. Reconnects forever with backoff and
// lets panes resume their streams from the last byte they saw.

const KIND_OUTPUT = 1;
const KIND_INPUT = 2;
const enc = new TextEncoder();

export class Conn {
  constructor(handlers) {
    this.h = handlers;
    this.ws = null;
    this.rid = 0;
    this.pending = new Map();
    this.backoff = 500;
    this.status = 'connecting';
    this.timer = null;
    window.addEventListener('online', () => this.kick());
    document.addEventListener('visibilitychange', () => {
      if (!document.hidden) this.kick();
    });
  }

  connect() {
    clearTimeout(this.timer);
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${location.host}/ws`);
    ws.binaryType = 'arraybuffer';
    this.ws = ws;
    this.setStatus('connecting');
    ws.onopen = () => {
      this.backoff = 500;
      this.send({ t: 'hello', client: 'web' });
      this.setStatus('online');
      this.h.onOpen?.();
    };
    ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') this.onJSON(JSON.parse(ev.data));
      else this.onBinary(new Uint8Array(ev.data));
    };
    ws.onclose = (ev) => {
      if (this.ws !== ws) return;
      this.ws = null;
      for (const [, p] of this.pending) p.reject(new Error('disconnected'));
      this.pending.clear();
      this.setStatus(ev.code === 1008 || ev.code === 4001 ? 'unauthorized' : 'offline');
      this.timer = setTimeout(() => this.connect(), this.backoff);
      this.backoff = Math.min(this.backoff * 2, 8000);
    };
  }

  // Reconnect immediately (network came back, tab became visible).
  kick() {
    if (this.ws && this.ws.readyState <= 1) return;
    this.backoff = 300;
    this.connect();
  }

  setStatus(s) {
    this.status = s;
    this.h.onStatus?.(s);
  }

  send(msg) {
    if (this.ws?.readyState === 1) this.ws.send(JSON.stringify(msg));
  }

  request(msg) {
    return new Promise((resolve, reject) => {
      if (this.ws?.readyState !== 1) return reject(new Error('Not connected to the Gecko daemon'));
      msg.rid = ++this.rid;
      this.pending.set(msg.rid, { resolve, reject });
      this.send(msg);
      setTimeout(() => {
        if (this.pending.delete(msg.rid)) reject(new Error('The daemon did not answer in time'));
      }, 30000);
    });
  }

  input(id, data) {
    if (this.ws?.readyState !== 1) return false;
    const idb = enc.encode(id);
    const body = typeof data === 'string' ? enc.encode(data) : data;
    const frame = new Uint8Array(2 + idb.length + 8 + body.length);
    frame[0] = KIND_INPUT;
    frame[1] = idb.length;
    frame.set(idb, 2);
    frame.set(body, 2 + idb.length + 8);
    this.ws.send(frame);
    return true;
  }

  onBinary(b) {
    if (b[0] !== KIND_OUTPUT) return;
    const n = b[1];
    const sub = new TextDecoder().decode(b.subarray(2, 2 + n));
    const dv = new DataView(b.buffer, b.byteOffset + 2 + n, 8);
    const off = Number(dv.getBigUint64(0));
    this.h.onData?.(sub, off, b.subarray(2 + n + 8));
  }

  onJSON(m) {
    if (m.t === 'reply' || (m.rid && this.pending.has(m.rid))) {
      const p = this.pending.get(m.rid);
      if (!p) return;
      this.pending.delete(m.rid);
      if (m.error) {
        // Services from before protocol 2 answer unknown requests with a bare
        // "bad request"; say what that actually means.
        const msg = m.error === 'bad request'
          ? 'The Gecko background service is too old for this. Restart it to update: gecko stop && gecko'
          : m.error;
        p.reject(new Error(msg));
      }
      else p.resolve(m);
      return;
    }
    this.h.onMessage?.(m);
  }
}
