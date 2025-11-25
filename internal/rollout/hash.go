// Package rollout provides deterministic user bucketing for feature flag rollouts.
package rollout

import (
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
	key := userID + ":" + flagKey + ":" + salt
	hash := xxhash.Sum64String(key)
	return int(hash % 100) // Returns 0-99
}
