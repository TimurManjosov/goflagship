// Package rollout provides deterministic user bucketing for feature flag rollouts.
package rollout

import (
	"strings"

	"github.com/cespare/xxhash/v2"
)

// BucketUser returns a deterministic bucket (0-99) for the given user and flag.
// The same userID + flagKey + salt combination will always return the same bucket.
// This is used to determine if a user should see a feature at a given rollout percentage.
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
