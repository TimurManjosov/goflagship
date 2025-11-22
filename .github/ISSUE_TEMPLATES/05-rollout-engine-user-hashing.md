# Implement Rollout Engine with Percentage Splits and User ID Hashing

## Problem / Motivation

Currently, the `rollout` field exists in the database but is not used for actual feature rollout logic. The system needs:

1. **Gradual Rollouts**: Enable features for X% of users (e.g., 25% rollout)
2. **Deterministic Assignment**: Same user always gets same result for a flag
3. **Even Distribution**: Users should be evenly distributed across rollout buckets
4. **User Context**: Support targeting based on user ID or other context
5. **Variant Splits**: Support A/B/C testing with multiple variants (e.g., 50% control, 30% variantA, 20% variantB)

Without these capabilities, teams can't:
- Safely roll out features incrementally
- Run A/B tests
- Reduce blast radius of bugs
- Gradually migrate users to new features

## Proposed Solution

Implement a deterministic rollout engine that:

1. Uses **consistent hashing** (e.g., MurmurHash3) on user ID + flag key
2. Maps hash to 0-100 range to determine bucket assignment
3. Compares bucket to rollout percentage to decide enablement
4. Supports multi-variant splits for A/B/C testing
5. Works client-side (SDK) and server-side (future evaluation endpoint)

## Concrete Tasks

### Phase 1: Core Hashing Algorithm
- [ ] Create `internal/rollout/hash.go` package
- [ ] Implement deterministic hash function:
  ```go
  // BucketUser returns 0-99 bucket for user+flag
  func BucketUser(userID, flagKey string, salt string) int
  ```
- [ ] Use MurmurHash3 (or xxhash) for speed and distribution
- [ ] Add salt parameter for hash stability across deployments
- [ ] Add unit tests verifying:
  - Same input → same bucket
  - Different users → even distribution across buckets
  - 1M samples → ~1% in each bucket (chi-squared test)

### Phase 2: Rollout Evaluation Logic
- [ ] Create rollout evaluation function:
  ```go
  // IsRolledOut returns true if user is in rollout
  func IsRolledOut(userID, flagKey string, rollout int32, salt string) bool {
    bucket := BucketUser(userID, flagKey, salt)
    return bucket < int(rollout)
  }
  ```
- [ ] Add validation:
  - Empty userID → return `false` (or random?)
  - Invalid rollout (not 0-100) → error
- [ ] Add unit tests for boundary cases:
  - rollout=0 → always false
  - rollout=100 → always true
  - rollout=50 → ~50% of users

### Phase 3: Multi-Variant Splits
- [ ] Extend flag schema to support variants:
  ```json
  {
    "key": "checkout_flow",
    "enabled": true,
    "rollout": 100,
    "variants": [
      {"name": "control", "weight": 50},
      {"name": "express", "weight": 30},
      {"name": "onepage", "weight": 20}
    ],
    "config": {...}
  }
  ```
- [ ] Add `variants` column to database (JSONB)
- [ ] Implement variant selection:
  ```go
  // GetVariant returns variant name for user
  func GetVariant(userID, flagKey string, variants []Variant, salt string) string
  ```
- [ ] Validate weights sum to 100
- [ ] Add unit tests for variant distribution

### Phase 4: SDK Integration (TypeScript)
- [ ] Update SDK to accept user context:
  ```typescript
  const client = new FlagshipClient({
    baseUrl: 'http://localhost:8080',
    user: { id: 'user-123' }
  });
  ```
- [ ] Implement client-side rollout evaluation:
  ```typescript
  isEnabled(key: string): boolean {
    const flag = this.flags[key];
    if (!flag) return false;
    if (!flag.enabled) return false;
    if (flag.rollout === 100) return true;
    if (flag.rollout === 0) return false;
    
    const bucket = this.bucketUser(this.user.id, key);
    return bucket < flag.rollout;
  }
  ```
- [ ] Port hashing algorithm to TypeScript (use `murmurhash-js`)
- [ ] Add variant evaluation in SDK
- [ ] Add tests for SDK rollout logic

### Phase 5: Rollout Salt Configuration
- [ ] Add environment variable:
  ```bash
  ROLLOUT_SALT=production-stable-salt
  ```
- [ ] Default to random salt on startup if not provided
- [ ] Warn if salt is not configured (non-deterministic across restarts)
- [ ] Document importance of stable salt for consistent assignments

### Phase 6: Backend Changes
- [ ] Update snapshot to include rollout and variants:
  ```go
  type FlagView struct {
    // ... existing fields
    Rollout  int32     `json:"rollout"`
    Variants []Variant `json:"variants,omitempty"`
  }
  ```
- [ ] Update upsert handler to validate variants
- [ ] Update migration to add variants column
- [ ] Ensure backward compatibility (variants are optional)

### Phase 7: Testing & Documentation
- [ ] Add end-to-end test:
  - Create flag with rollout=30
  - Test 1000 unique user IDs
  - Verify ~30% return true
- [ ] Document rollout feature in README
- [ ] Add examples:
  - Basic percentage rollout
  - A/B test with variants
  - Gradual rollout (0% → 10% → 50% → 100%)
- [ ] Document salt configuration and implications

## API Changes

### Flag Schema Changes (Backward Compatible)

**Basic Rollout** (existing field, now functional)
```json
{
  "key": "new_feature",
  "enabled": true,
  "rollout": 25,  // 25% of users
  "env": "prod",
  "config": {}
}
```

**Multi-Variant A/B Test** (new optional field)
```json
{
  "key": "checkout_flow",
  "enabled": true,
  "rollout": 100,
  "variants": [
    {"name": "control", "weight": 50, "config": {"layout": "standard"}},
    {"name": "express", "weight": 30, "config": {"layout": "single-page"}},
    {"name": "premium", "weight": 20, "config": {"layout": "vip"}}
  ],
  "env": "prod",
  "config": {}  // default config if no variant matched
}
```

### SDK API Changes

**Constructor with User Context**
```typescript
const client = new FlagshipClient({
  baseUrl: 'http://localhost:8080',
  user: {
    id: 'user-123',              // Required for rollouts
    attributes?: {               // Optional, for future targeting
      plan: 'premium',
      country: 'US'
    }
  }
});
```

**New Methods**
```typescript
// Existing method now uses rollout logic
client.isEnabled('feature_x');  // Uses rollout if configured

// New method for variant testing
client.getVariant('checkout_flow');  // Returns 'control' | 'express' | 'premium'

// Get variant config
client.getVariantConfig('checkout_flow');  // Returns config object for assigned variant
```

### Environment Variables
```bash
ROLLOUT_SALT=prod-stable-v1  # Important: must be stable across deployments
```

## Acceptance Criteria

### Hashing & Distribution
- [ ] Hash function is deterministic (same input → same output)
- [ ] Hash function is fast (<1μs per call)
- [ ] Distribution test: 100k users → each bucket ~1% ± 0.2%
- [ ] Same user+flag → same bucket across different processes
- [ ] Different users → evenly distributed
- [ ] Salt changes → different distribution (avoid this in production)

### Rollout Logic
- [ ] rollout=0 → no users enabled
- [ ] rollout=100 → all users enabled
- [ ] rollout=25 → ~25% of users enabled (±1% in large samples)
- [ ] Empty userID → consistent behavior (document choice)
- [ ] Same user checked twice → same result

### Variants
- [ ] Variant weights sum to 100 (validation)
- [ ] Variant assignment is deterministic per user
- [ ] Variant distribution matches weights (±2% in large samples)
- [ ] Variants work with enabled=true and rollout=100

### SDK Integration
- [ ] SDK evaluates rollout client-side
- [ ] SDK requires user context for rollout-enabled flags
- [ ] SDK methods work with and without rollout
- [ ] Backward compatible with existing flags (no variants)

### Documentation
- [ ] Rollout feature documented with examples
- [ ] Salt configuration explained
- [ ] Variant testing guide with A/B test example
- [ ] Migration guide for existing flags

## Notes / Risks / Edge Cases

### Risks
- **Salt Instability**: Changing salt redistributes users
  - Mitigation: Strongly document salt importance, warn if not set
- **User ID Requirements**: Rollouts need user context
  - Mitigation: Graceful degradation (no user → treat as disabled? or random?)
- **Backward Compatibility**: Existing flags have rollout=0 by default
  - Mitigation: Keep rollout as opt-in, default 0 means "ignore rollout logic" OR 100?

### Edge Cases
- What if user ID is null/empty/undefined?
  - Option A: Treat as disabled
  - Option B: Use random bucket (not deterministic)
  - Option C: Use default bucket 0
  - **Decision**: Document expected behavior
- Concurrent variant weight updates (validation on write)
- Variant weights don't sum to 100 (reject on write)
- Same variant name appears twice (validation)
- User ID with special characters (hash should handle any string)
- Very long user IDs (>1MB) (should reject or hash)

### Hash Algorithm Choice
- **MurmurHash3**: Fast, good distribution, widely used
- **xxHash**: Faster than Murmur, also good distribution
- **MD5/SHA**: Slower, overkill for this use case
- **Recommendation**: xxHash for speed, MurmurHash3 for ecosystem compatibility

### Performance Considerations
- Hash calculation happens on every flag check (SDK side)
- SDK should cache bucket assignments per user+flag
- Consider batch evaluation API in future (evaluate all flags for user at once)

### Future Enhancements
- Sticky bucketing (ensure user stays in bucket even if weight changes)
- Time-based rollouts (gradual increase over time)
- Server-side evaluation endpoint (for server-to-server)
- Rollout override per user ID (whitelist/blacklist)
- Canary deployments (route % to specific instances)

## Implementation Hints

- Current rollout field is in `internal/db/queries/flags.sql` (UpsertFlag)
- Snapshot includes rollout in `internal/snapshot/snapshot.go` (FlagView struct)
- SDK is in `sdk/flagshipClient.ts`
- Hash libraries:
  - Go: `github.com/spaolacci/murmur3` or `github.com/cespare/xxhash`
  - TypeScript: `murmurhash-js` npm package
- Example bucket calculation:
  ```go
  hash := murmur3.Sum32([]byte(userID + ":" + flagKey + ":" + salt))
  bucket := int(hash % 100)  // 0-99
  ```
- Variant selection similar but use cumulative weights

## Labels

`feature`, `backend`, `sdk`, `algorithm`, `good-first-issue` (for hash unit tests)

## Estimated Effort

**3 days**
- Day 1: Core hashing + rollout evaluation + unit tests
- Day 2: Multi-variant support + database schema + backend integration
- Day 3: SDK integration + end-to-end tests + documentation
