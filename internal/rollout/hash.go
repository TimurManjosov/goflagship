// Package rollout provides deterministic user bucketing for feature flag rollouts.
package rollout

import (
	"strings"

	"github.com/cespare/xxhash/v2"
)

// BucketUser returns a deterministic bucket (0-99) for the given user and flag.
//
// Preconditions:
//   - userID, flagKey, salt may be empty strings (see edge cases)
//
// Postconditions:
//   - Returns integer in range [0, 99] for valid inputs
//   - Returns -1 for invalid inputs (empty userID)
//   - Same inputs always produce same output (deterministic)
//
// Algorithm:
//   1. Concatenate userID:flagKey:salt with ':' delimiters
//   2. Compute xxHash64 of the concatenated string
//   3. Return hash % 100 to get bucket in range [0, 99]
//
// Edge Cases:
//   - userID="": Returns -1 (invalid, no user context)
//   - flagKey="": Valid (uses "::" pattern, unusual but deterministic)
//   - salt="": Valid (reduces hash quality but still deterministic)
//   - All empty: Returns -1 (userID is empty)
//
// Deterministic Behavior:
//   The same (userID, flagKey, salt) combination will always return the same bucket.
//   This ensures consistent user experience across requests and server instances.
//   Uses xxHash64 for high-quality, evenly-distributed hashing.
//
// Performance:
//   Uses strings.Builder to avoid allocations in hot path.
//   Pre-grows builder to exact size needed.
func BucketUser(userID, flagKey, salt string) int {
	if userID == "" {
		return -1 // Invalid: no user context
	}
	// Combine userID, flagKey, and salt with delimiters for uniqueness
	// Use strings.Builder to avoid intermediate string allocations in hot path
	var builder strings.Builder
	const delimiterCount = 2 // Two ':' delimiters between the three components
	builder.Grow(len(userID) + len(flagKey) + len(salt) + delimiterCount)
	builder.WriteString(userID)
	builder.WriteByte(':')
	builder.WriteString(flagKey)
	builder.WriteByte(':')
	builder.WriteString(salt)
	
	hash := xxhash.Sum64String(builder.String())
	return int(hash % 100) // Returns 0-99
}
