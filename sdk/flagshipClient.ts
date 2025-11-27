// sdk/flagshipClient.ts
import murmur from 'murmurhash-js';
import jsonLogic from 'json-logic-js';

/** Variant definition for A/B testing */
type Variant = {
  name: string;
  weight: number;
  config?: Record<string, unknown>;
};

type FlagView = {
  key: string;
  description?: string;
  enabled: boolean;
  rollout: number;
  expression?: string | null;
  config?: Record<string, unknown>;
  variants?: Variant[];
  env: string;
  updatedAt: string;
};

type Snapshot = {
  etag: string;
  flags: Record<string, FlagView>;
  updatedAt: string;
  rolloutSalt?: string; // Salt for deterministic rollout evaluation
};

type Events = {
  update: (etag: string) => void;
  ready: () => void;
  error: (err: unknown) => void;
};

type ListenerMap = Partial<Record<keyof Events, Function[]>>;

type Listener<T extends keyof Events> = Events[T];

/** User context for rollout evaluation */
export type UserContext = {
  id: string; // Required for rollouts
  attributes?: Record<string, unknown>; // Optional, for future targeting
};

export type ClientOptions = {
  baseUrl?: string; // default http://localhost:8080
  user?: UserContext; // User context for rollout evaluation
  reconnect?: boolean; // SSE reconnect on error (default true)
  reconnectMinMs?: number; // backoff start (default 500ms)
  reconnectMaxMs?: number; // backoff cap (default 10_000ms)
  fetchImpl?: typeof fetch; // optional custom fetch
  eventSourceImpl?: typeof EventSource; // optional custom EventSource (Node)
};

export class FlagshipClient {
  private base = 'http://localhost:8080';
  private fetch: typeof fetch;
  private ES: typeof EventSource | undefined;
  private es: EventSource | null = null;

  private cache: Snapshot | null = null;
  private listeners: ListenerMap = {};
  private reconnect = true;
  private rMin = 500;
  private rMax = 10_000;
  private stopping = false;
  private user: UserContext | undefined;

  constructor(opts: ClientOptions = {}) {
    this.base = opts.baseUrl ?? this.base;
    this.user = opts.user;

    // ✅ bind fetch to the global object to avoid "Illegal invocation"
    const f = opts.fetchImpl ?? (globalThis as any).fetch;
    if (!f) throw new Error('fetch is not available in this environment');
    this.fetch = f.bind(globalThis);

    this.ES = opts.eventSourceImpl ?? (globalThis as any).EventSource;

    this.reconnect = opts.reconnect ?? true;
    this.rMin = opts.reconnectMinMs ?? this.rMin;
    this.rMax = opts.reconnectMaxMs ?? this.rMax;
  }

  // ---- public API ----

  async init(): Promise<void> {
    await this.refresh(); // initial snapshot
    this.openStream(); // subscribe for updates
    this.emit('ready');
  }

  isReady(): boolean {
    return !!this.cache;
  }

  /**
   * Set or update the user context for rollout evaluation.
   * Call this when the user logs in or their identity changes.
   */
  setUser(user: UserContext): void {
    this.user = user;
  }

  /**
   * Get the current user context.
   */
  getUser(): UserContext | undefined {
    return this.user;
  }

  /**
   * Check if a flag is enabled for the current user.
   * Takes into account the flag's enabled state, targeting expression, and rollout percentage.
   * 
   * @param key - The flag key
   * @returns true if the flag is enabled and the user matches targeting rules
   */
  isEnabled(key: string): boolean {
    const flag = this.cache?.flags?.[key];
    if (!flag) return false;
    if (!flag.enabled) return false;

    // Check expression if present
    if (flag.expression) {
      const matches = this.evaluateExpression(flag.expression);
      if (!matches) return false;
    }

    // If rollout is 100, enabled (expression already passed)
    if (flag.rollout >= 100) return true;
    // If rollout is 0, always disabled
    if (flag.rollout <= 0) return false;

    // Need user context for partial rollout
    if (!this.user?.id) return false;

    // Calculate bucket and check rollout
    const bucket = this.bucketUser(this.user.id, key);
    return bucket < flag.rollout;
  }

  /**
   * Get the config for a flag.
   */
  getConfig<T = unknown>(key: string): T | undefined {
    return this.cache?.flags?.[key]?.config as T | undefined;
  }

  /**
   * Get the variant name assigned to the current user for a flag.
   * Returns undefined if no variants are defined or user context is missing.
   * 
   * @param key - The flag key
   * @returns The variant name or undefined
   */
  getVariant(key: string): string | undefined {
    const flag = this.cache?.flags?.[key];
    if (!flag || !flag.variants || flag.variants.length === 0) return undefined;
    if (!this.user?.id) return undefined;

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
  getVariantConfig<T = unknown>(key: string): T | undefined {
    const variantName = this.getVariant(key);
    if (!variantName) return undefined;

    const flag = this.cache?.flags?.[key];
    if (!flag?.variants) return undefined;

    const variant = flag.variants.find((v) => v.name === variantName);
    return variant?.config as T | undefined;
  }

  keys(): string[] {
    return Object.keys(this.cache?.flags ?? {});
  }

  on<T extends keyof Events>(ev: T, fn: Events[T]): () => void {
    // get/create the bucket and cast it to Function[]
    const arr = (this.listeners[ev] ||= [] as Function[]);
    arr.push(fn as unknown as Function);

    return () => {
      const list = this.listeners[ev] as Function[] | undefined;
      if (!list) return;
      this.listeners[ev] = list.filter((f) => f !== (fn as unknown as Function));
    };
  }

  close(): void {
    this.stopping = true;
    this.es?.close();
    this.es = null;
  }

  // ---- internals ----

  /**
   * Evaluate a JSON Logic expression against the current user context.
   * Returns true if the expression matches, false if it doesn't match or is invalid.
   * For security, invalid expressions fail closed (return false).
   * 
   * @param expression - JSON Logic expression string
   * @returns true if user matches the expression
   */
  private evaluateExpression(expression: string): boolean {
    if (!expression || expression.trim() === '') {
      return true; // No expression means no targeting restriction
    }

    // Build context from user attributes
    const context: Record<string, unknown> = {};
    if (this.user) {
      context.id = this.user.id;
      // Merge attributes into the context
      if (this.user.attributes) {
        Object.assign(context, this.user.attributes);
      }
    }

    try {
      const logic = JSON.parse(expression);
      const result = jsonLogic.apply(logic, context);
      // Treat any truthy value as true
      return Boolean(result);
    } catch {
      // For security, fail closed on invalid expressions
      // This prevents accidental access when expressions are malformed
      return false;
    }
  }

  /**
   * Calculate the bucket (0-99) for a user and flag using MurmurHash3.
   * This is deterministic: same user + flag + salt = same bucket.
   */
  private bucketUser(userId: string, flagKey: string): number {
    const salt = this.cache?.rolloutSalt ?? '';
    const key = `${userId}:${flagKey}:${salt}`;
    const hash = murmur.murmur3(key);
    // Use Math.abs to handle potential negative values from modulo
    return Math.abs(hash % 100);
  }

  private async refresh(): Promise<void> {
    await this.fetchSnapshot();
  }

  private openStream(): void {
    if (!this.ES) {
      throw new Error('EventSource is not available. Provide eventSourceImpl in Node.');
    }
    const url = `${this.base}/v1/flags/stream`;
    const es = new this.ES(url);
    this.es = es;

    es.addEventListener('init', async (e: MessageEvent) => {
      try {
        const { etag } = JSON.parse(e.data);
        const changed = await this.refreshWithETag(etag);
        if (changed) {
          this.emit('update', etag);
        }
      } catch (err) {
        this.emit('error', err);
      }
    });

    es.addEventListener('update', async (e: MessageEvent) => {
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
      } catch (err) {
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

  private async refreshWithETag(etag: string): Promise<boolean> {
    if (!etag) return false;

    const changed = await this.fetchSnapshot();
    return changed;
  }

  private snapshotUrl(): string {
    const base = this.base.endsWith('/') ? this.base : `${this.base}/`;
    const url = new URL(`${base}v1/flags/snapshot`);
    url.searchParams.set('ts', Date.now().toString());
    return url.toString();
  }

  private async fetchSnapshot(skipCache = false): Promise<boolean> {
    const url = this.snapshotUrl();
    const headers: Record<string, string> = {};

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

    const next = (await res.json()) as Snapshot;
    const changed = !this.cache || this.cache.etag !== next.etag;
    this.cache = next;
    return changed;
  }

  private scheduleReconnect() {
    let delay = this.rMin + Math.random() * (this.rMin / 2);
    delay = Math.min(delay, this.rMax);
    setTimeout(() => {
      if (!this.stopping && !this.es) this.openStream();
    }, delay);
    // Exponential-ish backoff
    this.rMin = Math.min(this.rMin * 2, this.rMax);
  }

  private emit<T extends keyof Events>(ev: T, ...args: Parameters<Events[T]>) {
    for (const fn of this.listeners[ev] || []) {
      (fn as (...a: Parameters<Events[T]>) => void)(...args);
    }
  }
}
