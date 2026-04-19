package main

import (
	"testing"
)

func TestParseS3Offset(t *testing.T) {
	// Create mock 64-bit C struct
	// Sector: 12345, Object: 99
	var fakePackedNumber uint64 = (uint64(99) << 48) | uint64(12345)

	// Run Go translation
	result := ParseS3Offset(fakePackedNumber)

	// Verify mask logic
	if result.Sector != 12345 {
		t.Errorf("Expected Sector 12345, but got %d", result.Sector)
	}

	// Verify shift logic
	if result.Object != 99 {
		t.Errorf("Expected Object 99, but got %d", result.Object)
	}
}
