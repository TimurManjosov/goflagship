import { FlagshipClient } from '../dist/flagshipClient.js';
import { pathToFileURL } from 'node:url';

class MessageEventShim {
  constructor(type, data) {
    this.type = type;
    this.data = data;
  }
}

class HeadlessEventSource {
  #url;
  #listeners = new Map();
  #controller = new AbortController();
  onerror = null;

  constructor(url) {
    this.#url = url;
    this.#start();
  }

  async #start() {
    try {
      const res = await fetch(this.#url, {
        cache: 'no-store',
        headers: {
          Accept: 'text/event-stream',
        },
        signal: this.#controller.signal,
      });

      if (!res.ok || !res.body) {
        throw new Error(`SSE connect failed: ${res.status}`);
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          buffer += decoder.decode();
          if (buffer.trim().length > 0) {
            this.#processEvent(buffer);
          }
          break;
        }
        buffer += decoder.decode(value, { stream: true });

        let idx;
        while ((idx = buffer.indexOf('\n\n')) !== -1) {
          const chunk = buffer.slice(0, idx);
          buffer = buffer.slice(idx + 2);
          if (chunk.trim().length === 0) continue;
          this.#processEvent(chunk);
        }
      }
    } catch (err) {
      if (err && err.name === 'AbortError') {
        return;
      }
      if (typeof this.onerror === 'function') {
        this.onerror(err);
      }
    }
  }

  #processEvent(raw) {
    const lines = raw.split(/\r?\n/);
    let eventName = 'message';
    const payloadLines = [];

    for (const line of lines) {
      if (!line) continue;
      if (line.startsWith('event:')) {
        eventName = line.slice(6).trim() || 'message';
        continue;
      }
      if (line.startsWith('data:')) {
        payloadLines.push(line.slice(5).trim());
        continue;
      }
      payloadLines.push(line.trim());
    }

    const joined = payloadLines.join('\n');
    let parsed = joined;
    try {
      const candidate = JSON.parse(joined);
      parsed = candidate?.data ?? candidate;
    } catch {
      // fall back to raw string
    }

    const data = typeof parsed === 'string' ? parsed : JSON.stringify(parsed);
    this.#dispatch(eventName, new MessageEventShim(eventName, data));
  }

  #dispatch(type, event) {
    const listeners = this.#listeners.get(type);
    if (!listeners) return;
    for (const listener of listeners) {
      listener(event);
    }
  }

  addEventListener(type, listener) {
    const set = this.#listeners.get(type) ?? new Set();
    set.add(listener);
    this.#listeners.set(type, set);
  }

  removeEventListener(type, listener) {
    const set = this.#listeners.get(type);
    if (!set) return;
    set.delete(listener);
    if (set.size === 0) {
      this.#listeners.delete(type);
    }
  }

  close() {
    this.#controller.abort();
    this.#listeners.clear();
  }
}

class HeadlessPage {
  #baseUrl;
  #pendingUpdate = null;
  #pendingTimer = null;
  outText = '';

  constructor(baseUrl) {
    this.#baseUrl = baseUrl;
    this.client = new FlagshipClient({
      baseUrl,
      eventSourceImpl: HeadlessEventSource,
    });

    this.client.on('ready', () => {
      this.#render();
    });

    this.client.on('update', () => {
      this.#render();
      if (this.#pendingUpdate) {
        const resolve = this.#pendingUpdate;
        this.#pendingUpdate = null;
        if (this.#pendingTimer) {
          clearTimeout(this.#pendingTimer);
          this.#pendingTimer = null;
        }
        resolve();
      }
    });

    this.client.on('error', (err) => {
      console.error('[HeadlessPage] SDK error:', err);
    });
  }

  async init() {
    await this.client.init();
    this.#render();
  }

  #render() {
    const flags = this.client.keys().map((key) => ({
      key,
      enabled: this.client.isEnabled(key),
      config: this.client.getConfig(key),
    }));
    const snapshot = {
      etag: this.client['cache']?.etag ?? null,
      flags,
    };
    this.outText = JSON.stringify(snapshot, null, 2);
  }

  currentEtag() {
    return this.client['cache']?.etag ?? null;
  }

  waitForNextUpdate(timeoutMs = 7000) {
    if (this.#pendingUpdate) {
      throw new Error('waitForNextUpdate already pending');
    }
    return new Promise((resolve, reject) => {
      this.#pendingUpdate = resolve;
      const timer = setTimeout(() => {
        this.#pendingUpdate = null;
        this.#pendingTimer = null;
        reject(new Error('Timed out waiting for update'));
      }, timeoutMs);
      if (typeof timer === 'object' && typeof timer.unref === 'function') {
        timer.unref();
      }
      this.#pendingTimer = timer;
    });
  }

  dispose() {
    this.client.close();
    if (this.#pendingTimer) {
      clearTimeout(this.#pendingTimer);
      this.#pendingTimer = null;
    }
  }
}

async function run() {
  const baseUrl = process.env.FLAGSHIP_BASE_URL ?? 'http://localhost:8080';
  const adminKey = process.env.FLAGSHIP_ADMIN_KEY ?? 'admin-123';
  const flagKey = process.env.FLAGSHIP_FLAG_KEY ?? 'checkout_new_ui';

  const page = new HeadlessPage(baseUrl);
  try {
    await page.init();
    const beforeEtag = page.currentEtag();

    const unique = `${Date.now()}`;
    const payload = {
      key: flagKey,
      description: `updated ${unique}`,
      enabled: true,
      rollout: 50,
      config: { variant: unique },
      env: 'prod',
    };

    const updatePromise = page.waitForNextUpdate();

    const res = await fetch(`${baseUrl}/v1/flags`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${adminKey}`,
      },
      body: JSON.stringify(payload),
    });

    if (!res.ok) {
      const text = await res.text();
      throw new Error(`Flag update failed: ${res.status} ${text}`);
    }

    await updatePromise;
    const afterEtag = page.currentEtag();

    if (!afterEtag || afterEtag === beforeEtag) {
      throw new Error(`Expected ETag to change (before=${beforeEtag}, after=${afterEtag})`);
    }

    console.log('✅ Live update detected. New ETag:', afterEtag);
    console.log('Rendered snapshot:\n', page.outText);
  } finally {
    page.dispose();
  }
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  run().catch((err) => {
    console.error('Live update harness failed:', err);
    process.exitCode = 1;
  });
}
