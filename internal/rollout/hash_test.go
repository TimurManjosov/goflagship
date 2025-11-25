package rollout

import (
	"testing"
)

func TestBucketUser_Deterministic(t *testing.T) {
	// Same inputs should always return the same bucket
	userID := "user-123"
	flagKey := "feature_x"
	salt := "test-salt"

	bucket1 := BucketUser(userID, flagKey, salt)
	bucket2 := BucketUser(userID, flagKey, salt)

	if bucket1 != bucket2 {
		t.Errorf("BucketUser is not deterministic: got %d and %d", bucket1, bucket2)
	}

	// Should be in valid range
	if bucket1 < 0 || bucket1 >= 100 {
		t.Errorf("Bucket out of range: %d", bucket1)
	}
}

func TestBucketUser_DifferentUsersDistribution(t *testing.T) {
	// Test that different users get distributed across buckets
	flagKey := "feature_x"
	salt := "test-salt"
	bucketCounts := make([]int, 100)

	// Generate 10000 users to test distribution
	for i := 0; i < 10000; i++ {
		userID := "user-" + string(rune(i/1000)) + string(rune(i%1000))
		userID = "user-" + itoa(i)
		bucket := BucketUser(userID, flagKey, salt)
		if bucket >= 0 && bucket < 100 {
			bucketCounts[bucket]++
		}
	}

	// Check that distribution is roughly even (each bucket should have ~100 users)
	// Allow 50% variance (50-150 users per bucket)
	for i, count := range bucketCounts {
		if count < 50 || count > 150 {
			t.Errorf("Bucket %d has %d users, expected ~100", i, count)
		}
	}
}

func TestBucketUser_EmptyUserID(t *testing.T) {
	bucket := BucketUser("", "feature_x", "salt")
	if bucket != -1 {
		t.Errorf("Expected -1 for empty userID, got %d", bucket)
	}
}

func TestBucketUser_DifferentFlags(t *testing.T) {
	userID := "user-123"
	salt := "test-salt"

	bucket1 := BucketUser(userID, "feature_a", salt)
	bucket2 := BucketUser(userID, "feature_b", salt)

	// Different flags should generally give different buckets
	// (not guaranteed, but very likely)
	// We just verify both are valid
	if bucket1 < 0 || bucket1 >= 100 {
		t.Errorf("Bucket1 out of range: %d", bucket1)
	}
	if bucket2 < 0 || bucket2 >= 100 {
		t.Errorf("Bucket2 out of range: %d", bucket2)
	}
}

func TestBucketUser_DifferentSalts(t *testing.T) {
	userID := "user-123"
	flagKey := "feature_x"

	bucket1 := BucketUser(userID, flagKey, "salt1")
	bucket2 := BucketUser(userID, flagKey, "salt2")

	// Different salts should give different buckets
	// (not guaranteed but very likely for xxhash)
	if bucket1 < 0 || bucket1 >= 100 {
		t.Errorf("Bucket1 out of range: %d", bucket1)
	}
	if bucket2 < 0 || bucket2 >= 100 {
		t.Errorf("Bucket2 out of range: %d", bucket2)
	}
}

// Helper function for integer to string conversion
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
