// sdk/flagshipClient.ts
import murmur from 'murmurhash-js';
import jsonLogic from 'json-logic-js';
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
        this.user = opts.user;
        // ✅ bind fetch to the global object to avoid "Illegal invocation"
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
    /**
     * Set or update the user context for rollout evaluation.
     * Call this when the user logs in or their identity changes.
     */
    setUser(user) {
        this.user = user;
    }
    /**
     * Get the current user context.
     */
    getUser() {
        return this.user;
    }
    /**
     * Check if a flag is enabled for the current user.
     * Takes into account the flag's enabled state, targeting expression, and rollout percentage.
     *
     * @param key - The flag key
     * @returns true if the flag is enabled and the user matches targeting rules
     */
    isEnabled(key) {
        const flag = this.cache?.flags?.[key];
        if (!flag)
            return false;
        if (!flag.enabled)
            return false;
        // Check expression targeting if present
        if (flag.expression) {
            const matches = this.evaluateExpression(flag.expression);
            if (!matches)
                return false;
        }
        // If rollout is 100, always enabled (after expression check)
        if (flag.rollout >= 100)
            return true;
        // If rollout is 0, always disabled
        if (flag.rollout <= 0)
            return false;
        // Need user context for partial rollout
        if (!this.user?.id)
            return false;
        // Calculate bucket and check rollout
        const bucket = this.bucketUser(this.user.id, key);
        return bucket < flag.rollout;
    }
    /**
     * Get the config for a flag.
     */
    getConfig(key) {
        return this.cache?.flags?.[key]?.config;
    }
    /**
     * Get the variant name assigned to the current user for a flag.
     * Returns undefined if no variants are defined or user context is missing.
     *
     * @param key - The flag key
     * @returns The variant name or undefined
     */
    getVariant(key) {
        const flag = this.cache?.flags?.[key];
        if (!flag || !flag.variants || flag.variants.length === 0)
            return undefined;
        if (!this.user?.id)
            return undefined;
        const bucket = this.bucketUser(this.user.id, key);
        // Find variant based on cumulative weights
        let cumulative = 0;
        for (const variant of flag.variants) {
            cumulative += variant.weight;
            if (bucket < cumulative) {
                return variant.name;
            }
        }
        // Fallback to last variant (should not happen if weights sum to 100)
        return flag.variants[flag.variants.length - 1]?.name;
    }
    /**
     * Get the config for the variant assigned to the current user.
     * Returns undefined if no variants or user context is missing.
     *
     * @param key - The flag key
     * @returns The variant config or undefined
     */
    getVariantConfig(key) {
        const variantName = this.getVariant(key);
        if (!variantName)
            return undefined;
        const flag = this.cache?.flags?.[key];
        if (!flag?.variants)
            return undefined;
        const variant = flag.variants.find((v) => v.name === variantName);
        return variant?.config;
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
    /**
     * Evaluate a JSON Logic expression against the user context.
     * Returns true if the expression matches, false otherwise.
     * Returns false if expression parsing fails or user context is missing.
     */
    evaluateExpression(expression) {
        try {
            // Parse the expression (it's stored as a JSON string)
            const rule = JSON.parse(expression);
            // Build the data context from user attributes
            const data = {
                ...this.user?.attributes,
                id: this.user?.id,
            };
            // Apply JSON Logic rule
            const result = jsonLogic.apply(rule, data);
            // Convert result to boolean (JSON Logic can return various types)
            return Boolean(result);
        }
        catch {
            // If expression parsing fails, treat as not matching
            return false;
        }
    }
    /**
     * Calculate the bucket (0-99) for a user and flag using MurmurHash3.
     * This is deterministic: same user + flag + salt = same bucket.
     */
    bucketUser(userId, flagKey) {
        const salt = this.cache?.rolloutSalt ?? '';
        const key = `${userId}:${flagKey}:${salt}`;
        const hash = murmur.murmur3(key);
        // Use Math.abs to handle potential negative values from modulo
        return Math.abs(hash % 100);
    }
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
