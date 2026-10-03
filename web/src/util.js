export const isMac = /Mac|iPhone|iPad/.test(navigator.platform) || /Mac OS/.test(navigator.userAgent);
export const isTouch = matchMedia('(pointer: coarse)').matches;

export const $ = (sel, root = document) => root.querySelector(sel);

export function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
}

export function ago(ms) {
  if (!ms) return '';
  const s = Math.max(0, Math.round((Date.now() - ms) / 1000));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 172800) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

export function shortPath(p) {
  if (!p) return '';
  const home = p.match(/^\/(home|Users)\/[^/]+/);
  if (home) p = '~' + p.slice(home[0].length);
  const parts = p.split('/');
  return parts.length > 4 ? '…/' + parts.slice(-2).join('/') : p;
}

// Copy text, falling back to execCommand where the async clipboard API is
// unavailable (plain-http LAN access from a phone).
export async function copyText(text) {
  try {
    if (window.isSecureContext && navigator.clipboard) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {}
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.setAttribute('readonly', '');
  ta.style.cssText = 'position:fixed;top:0;left:0;opacity:0';
  document.body.appendChild(ta);
  ta.select();
  ta.setSelectionRange(0, text.length);
  let ok = false;
  try { ok = document.execCommand('copy'); } catch {}
  ta.remove();
  return ok;
}

export async function readClipboard() {
  if (window.isSecureContext && navigator.clipboard?.readText) {
    try { return await navigator.clipboard.readText(); } catch {}
  }
  return null;
}

// Subsequence fuzzy match; returns a score or -1.
export function fuzzy(query, text) {
  query = query.toLowerCase();
  text = text.toLowerCase();
  if (!query) return 0;
  let score = 0, ti = 0, streak = 0;
  for (const ch of query) {
    const i = text.indexOf(ch, ti);
    if (i < 0) return -1;
    streak = i === ti ? streak + 1 : 0;
    score += 1 + streak * 2 + (i === 0 || /[\s/._-]/.test(text[i - 1]) ? 3 : 0);
    ti = i + 1;
  }
  return score - text.length * 0.01;
}

export function store(key, fallback) {
  try {
    const v = localStorage.getItem('gecko.' + key);
    return v == null ? fallback : JSON.parse(v);
  } catch { return fallback; }
}

export function save(key, value) {
  try { localStorage.setItem('gecko.' + key, JSON.stringify(value)); } catch {}
}
