// sdk/flagshipClient.ts
type FlagView = {
  key: string;
  description?: string;
  enabled: boolean;
  rollout: number;
  expression?: string | null;
  config?: Record<string, unknown>;
  env: string;
  updatedAt: string;
};

type Snapshot = {
  etag: string;
  flags: Record<string, FlagView>;
  updatedAt: string;
};

type Events = {
  update: (etag: string) => void;
  ready: () => void;
  error: (err: unknown) => void;
};

type ListenerMap = Partial<Record<keyof Events, Function[]>>;

type Listener<T extends keyof Events> = Events[T];

export type ClientOptions = {
  baseUrl?: string; // default http://localhost:8080
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

  constructor(opts: ClientOptions = {}) {
    this.base = opts.baseUrl ?? this.base;

    // ✅ bind fetch to the global object to avoid “Illegal invocation”
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

  isEnabled(key: string): boolean {
    return !!this.cache?.flags?.[key]?.enabled;
  }

  getConfig<T = unknown>(key: string): T | undefined {
    return this.cache?.flags?.[key]?.config as T | undefined;
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

        if (etag === this.cache?.etag) {
          const currentEtag = this.cache?.etag ?? etag;
          this.emit('update', currentEtag);
          return;
        }

        const changed = await this.refreshWithETag(etag);
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

  private async fetchSnapshot(): Promise<boolean> {
    const url = this.snapshotUrl();
    const headers: Record<string, string> = {};
    if (this.cache?.etag) {
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
    this.cache = next;
    return true;
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
