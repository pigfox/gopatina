// site-overflow.mjs: checks that a page never scrolls sideways on a phone.
//
// usage: node scripts/site-overflow.mjs CHROME URL
//
// It starts headless Chrome (or Chromium) with its DevTools protocol, loads
// URL at the phone widths 320, 360, 390, 412 and 430 CSS pixels and the
// tablet and desktop widths 768, 1024 and 1440, and fails if the page is
// wider than the viewport at any of them, printing the
// elements that stick out. Content inside its own scroll container (a code
// block or a wide table with overflow-x: auto) does not count: it scrolls
// on its own, not the page. Node 22 or later (for the built-in WebSocket);
// no packages. CI runs it on gopatina.org's page (.github/workflows/ci.yml).
import { spawn } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const [chrome, url] = process.argv.slice(2);
if (!chrome || !url) {
  console.error('usage: node scripts/site-overflow.mjs CHROME URL');
  process.exit(2);
}
const widths = [320, 360, 390, 412, 430, 768, 1024, 1440];
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// start launches Chrome with a new profile and returns it with a DevTools
// connection to its page, or with no connection if Chrome did not come up
// within 20 seconds or exited first. Chrome picks a free port itself
// (--remote-debugging-port=0) and writes it to DevToolsActivePort in its
// profile, so two checks, or a Chrome still exiting, never meet on a port.
const start = async () => {
  const profile = mkdtempSync(join(tmpdir(), 'site-overflow-'));
  const proc = spawn(chrome, [
    '--headless', '--no-sandbox', '--disable-gpu', '--no-first-run',
    '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank',
  ], { stdio: 'ignore' });
  let exited = false;
  proc.once('exit', () => { exited = true; });
  let ws;
  for (let i = 0; i < 100 && !ws && !exited; i++) {
    await sleep(200);
    try {
      const port = readFileSync(join(profile, 'DevToolsActivePort'), 'utf8').split('\n')[0];
      const targets = await (await fetch('http://127.0.0.1:' + port + '/json/list')).json();
      const page = targets.find((t) => t.type === 'page');
      if (page) ws = new WebSocket(page.webSocketDebuggerUrl);
    } catch {
      // Chrome is not listening yet.
    }
  }
  return { proc, profile, ws };
};

// stop stops Chrome, waits for it to exit so it no longer writes to its
// profile, and removes the profile; a failed clean-up of a temporary
// directory does not change the result.
const stop = async ({ proc, profile }) => {
  if (proc.exitCode === null && proc.signalCode === null) {
    const exited = new Promise((resolve) => proc.once('exit', resolve));
    proc.kill();
    await Promise.race([exited, sleep(5000)]);
  }
  try {
    rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  } catch {
    // Left for the system's temporary-file clean-up.
  }
};

// A Chrome that does not come up is tried again, up to three times: a slow
// or failed start on a busy machine says nothing about the page.
let browser;
for (let attempt = 1; attempt <= 3; attempt++) {
  browser = await start();
  if (browser.ws) break;
  await stop(browser);
  console.error('site-overflow: Chrome did not start (attempt ' + attempt + ' of 3)');
}
const done = async (code) => {
  await stop(browser);
  process.exit(code);
};
const { ws } = browser;
if (!ws) {
  console.error('site-overflow: Chrome did not start');
  await done(2);
}
await new Promise((r) => ws.addEventListener('open', r));
let next = 0;
const pending = new Map();
ws.addEventListener('message', (e) => {
  const m = JSON.parse(e.data);
  if (m.id && pending.has(m.id)) {
    pending.get(m.id)(m);
    pending.delete(m.id);
  }
});
const send = (method, params = {}) => new Promise((resolve) => {
  const id = ++next;
  pending.set(id, resolve);
  ws.send(JSON.stringify({ id, method, params }));
});

// measure runs in the page: the page's width, and every element that sticks
// out past the viewport without a scrolling ancestor to hold it.
const measure = `(() => {
  const w = document.documentElement.clientWidth;
  const sw = Math.max(document.documentElement.scrollWidth, document.body.scrollWidth);
  // clips reports whether el's own overflow holds its content. Table rows
  // and row groups ignore overflow, so they never do.
  const clips = (el) => {
    const cs = getComputedStyle(el);
    if (/^table-(row|row-group|header-group|footer-group|column|column-group)$/.test(cs.display)) return false;
    return ['auto', 'scroll', 'hidden', 'clip'].includes(cs.overflowX);
  };
  const scrolls = (el) => {
    for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
      if (clips(p)) return true;
    }
    return false;
  };
  const name = (el) => el.tagName.toLowerCase() + (el.id ? '#' + el.id : '');
  const text = (el) => (el.textContent || '').trim().replace(/\\s+/g, ' ').slice(0, 70);
  const out = [];
  for (const el of document.querySelectorAll('body *')) {
    if (scrolls(el)) continue;
    const b = el.getBoundingClientRect();
    if (b.right > w + 0.5) {
      out.push(name(el) + ' (right edge ' + Math.round(b.right) + 'px): ' + text(el));
    } else if (!clips(el) && b.left + el.scrollWidth > w + 0.5) {
      out.push(name(el) + ' (its content reaches ' + Math.round(b.left + el.scrollWidth) + 'px): ' + text(el));
    }
  }
  return { w, sw, out: out.slice(0, 20) };
})()`;

let failed = false;
await send('Page.enable');
for (const width of widths) {
  await send('Emulation.setDeviceMetricsOverride', { width, height: 800, deviceScaleFactor: 2, mobile: width < 768 });
  const loaded = new Promise((resolve) => {
    const on = (e) => {
      if (JSON.parse(e.data).method === 'Page.loadEventFired') {
        ws.removeEventListener('message', on);
        resolve();
      }
    };
    ws.addEventListener('message', on);
  });
  await send('Page.navigate', { url });
  await loaded;
  const { result } = await send('Runtime.evaluate', { expression: measure, returnByValue: true });
  const v = result.result.value;
  const ok = v.sw <= v.w;
  console.log(width + 'px: page ' + v.sw + 'px wide in a ' + v.w + 'px viewport: ' + (ok ? 'ok' : 'scrolls sideways'));
  if (!ok) {
    failed = true;
    for (const o of v.out) console.log('  ' + o);
  }
}
ws.close();
await done(failed ? 1 : 0);
