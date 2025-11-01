export class FlagshipClient {
    constructor(opts = {}) {
        this.base = 'http://localhost:8080';
        this.es = null;
        this.cache = null;
        this.listeners = {};
        this.reconnect = true;
        this.rMin = 500;
        this.rMax = 10000;
        this.stopping = false;
        this.base = opts.baseUrl ?? this.base;
        // ✅ bind fetch to the global object to avoid “Illegal invocation”
        const f = opts.fetchImpl ?? globalThis.fetch;
        if (!f)
            throw new Error('fetch is not available in this environment');
        this.fetch = f.bind(globalThis);
        this.ES = opts.eventSourceImpl ?? globalThis.EventSource;
        this.reconnect = opts.reconnect ?? true;
        this.rMin = opts.reconnectMinMs ?? this.rMin;
        this.rMax = opts.reconnectMaxMs ?? this.rMax;
    }
    // ---- public API ----
    async init() {
        await this.refresh(); // initial snapshot
        this.openStream(); // subscribe for updates
        this.emit('ready');
    }
    isReady() {
        return !!this.cache;
    }
    isEnabled(key) {
        return !!this.cache?.flags?.[key]?.enabled;
    }
    getConfig(key) {
        return this.cache?.flags?.[key]?.config;
    }
    keys() {
        return Object.keys(this.cache?.flags ?? {});
    }
    on(ev, fn) {
        var _a;
        // get/create the bucket and cast it to Function[]
        const arr = ((_a = this.listeners)[ev] || (_a[ev] = []));
        arr.push(fn);
        return () => {
            const list = this.listeners[ev];
            if (!list)
                return;
            this.listeners[ev] = list.filter((f) => f !== fn);
        };
    }
    close() {
        this.stopping = true;
        this.es?.close();
        this.es = null;
    }
    // ---- internals ----
    async refresh() {
        await this.fetchSnapshot();
    }
    openStream() {
        if (!this.ES) {
            throw new Error('EventSource is not available. Provide eventSourceImpl in Node.');
        }
        const url = `${this.base}/v1/flags/stream`;
        const es = new this.ES(url);
        this.es = es;
        es.addEventListener('init', async (e) => {
            try {
                const { etag } = JSON.parse(e.data);
                const changed = await this.refreshWithETag(etag);
                if (changed) {
                    this.emit('update', etag);
                }
            }
            catch (err) {
                this.emit('error', err);
            }
        });
        es.addEventListener('update', async (e) => {
            try {
                const { etag } = JSON.parse(e.data);
                if (!etag) {
                    return;
                }
                // Skip if we already have this version
                if (etag === this.cache?.etag) {
                    return;
                }
                // Force fetch without If-None-Match since we know there's an update
                await this.fetchSnapshot(true);
                // Always emit update after successful fetch
                this.emit('update', this.cache?.etag ?? etag);
            }
            catch (err) {
                this.emit('error', err);
            }
        });
        es.onerror = () => {
            this.es?.close();
            this.es = null;
            if (this.reconnect && !this.stopping) {
                this.scheduleReconnect();
            }
        };
    }
    async refreshWithETag(etag) {
        if (!etag)
            return false;
        const changed = await this.fetchSnapshot();
        return changed;
    }
    snapshotUrl() {
        const base = this.base.endsWith('/') ? this.base : `${this.base}/`;
        const url = new URL(`${base}v1/flags/snapshot`);
        url.searchParams.set('ts', Date.now().toString());
        return url.toString();
    }
    async fetchSnapshot(skipCache = false) {
        const url = this.snapshotUrl();
        const headers = {};
        // Only send If-None-Match if we're not forcing a refresh
        if (!skipCache && this.cache?.etag) {
            headers['If-None-Match'] = this.cache.etag;
        }
        const res = await this.fetch(url, {
            headers,
            cache: 'no-store',
        });
        if (res.status === 304) {
            return false;
        }
        if (!res.ok) {
            throw new Error(`snapshot ${res.status}`);
        }
        const next = (await res.json());
        const changed = !this.cache || this.cache.etag !== next.etag;
        this.cache = next;
        return changed;
    }
    scheduleReconnect() {
        let delay = this.rMin + Math.random() * (this.rMin / 2);
        delay = Math.min(delay, this.rMax);
        setTimeout(() => {
            if (!this.stopping && !this.es)
                this.openStream();
        }, delay);
        // Exponential-ish backoff
        this.rMin = Math.min(this.rMin * 2, this.rMax);
    }
    emit(ev, ...args) {
        for (const fn of this.listeners[ev] || []) {
            fn(...args);
        }
    }
}
