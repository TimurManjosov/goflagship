type Events = {
    update: (etag: string) => void;
    ready: () => void;
    error: (err: unknown) => void;
};
export type ClientOptions = {
    baseUrl?: string;
    reconnect?: boolean;
    reconnectMinMs?: number;
    reconnectMaxMs?: number;
    fetchImpl?: typeof fetch;
    eventSourceImpl?: typeof EventSource;
};
export declare class FlagshipClient {
    private base;
    private fetch;
    private ES;
    private es;
    private cache;
    private listeners;
    private reconnect;
    private rMin;
    private rMax;
    private stopping;
    constructor(opts?: ClientOptions);
    init(): Promise<void>;
    isReady(): boolean;
    isEnabled(key: string): boolean;
    getConfig<T = unknown>(key: string): T | undefined;
    keys(): string[];
    on<T extends keyof Events>(ev: T, fn: Events[T]): () => void;
    close(): void;
    private refresh;
    private openStream;
    private refreshWithETag;
    private snapshotUrl;
    private fetchSnapshot;
    private scheduleReconnect;
    private emit;
}
export {};
