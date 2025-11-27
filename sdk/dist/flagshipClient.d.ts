type Events = {
    update: (etag: string) => void;
    ready: () => void;
    error: (err: unknown) => void;
};
/** User context for rollout evaluation */
export type UserContext = {
    id: string;
    attributes?: Record<string, unknown>;
};
export type ClientOptions = {
    baseUrl?: string;
    user?: UserContext;
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
    private user;
    constructor(opts?: ClientOptions);
    init(): Promise<void>;
    isReady(): boolean;
    /**
     * Set or update the user context for rollout evaluation.
     * Call this when the user logs in or their identity changes.
     */
    setUser(user: UserContext): void;
    /**
     * Get the current user context.
     */
    getUser(): UserContext | undefined;
    /**
     * Check if a flag is enabled for the current user.
     * Takes into account the flag's enabled state, targeting expression, and rollout percentage.
     *
     * @param key - The flag key
     * @returns true if the flag is enabled and the user matches targeting rules
     */
    isEnabled(key: string): boolean;
    /**
     * Get the config for a flag.
     */
    getConfig<T = unknown>(key: string): T | undefined;
    /**
     * Get the variant name assigned to the current user for a flag.
     * Returns undefined if no variants are defined or user context is missing.
     *
     * @param key - The flag key
     * @returns The variant name or undefined
     */
    getVariant(key: string): string | undefined;
    /**
     * Get the config for the variant assigned to the current user.
     * Returns undefined if no variants or user context is missing.
     *
     * @param key - The flag key
     * @returns The variant config or undefined
     */
    getVariantConfig<T = unknown>(key: string): T | undefined;
    keys(): string[];
    on<T extends keyof Events>(ev: T, fn: Events[T]): () => void;
    close(): void;
    /**
     * Evaluate a JSON Logic expression against the user context.
     * Returns true if the expression matches, false otherwise.
     * Returns false if expression parsing fails or user context is missing.
     */
    private evaluateExpression;
    /**
     * Calculate the bucket (0-99) for a user and flag using MurmurHash3.
     * This is deterministic: same user + flag + salt = same bucket.
     */
    private bucketUser;
    private refresh;
    private openStream;
    private refreshWithETag;
    private snapshotUrl;
    private fetchSnapshot;
    private scheduleReconnect;
    private emit;
}
export {};
