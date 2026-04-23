package main

import (
	"testing"
)

// TestParseS3Offset validates the bitwise extraction logic used to bypass
// memory alignment padding on legacy C structs.
func TestParseS3Offset(t *testing.T) {
	// Simulate a packed 64-bit chunk.
	// Target Sector: 12345. Target Object ID: 99.
	var fakePackedNumber uint64 = (uint64(99) << 48) | uint64(12345)

	result := ParseS3Offset(fakePackedNumber)

	// Verify the bitwise AND mask logic.
	if result.Sector != 12345 {
		t.Errorf("Expected Sector 12345, but got %d", result.Sector)
	}

	// Verify the bitwise right-shift logic.
	if result.Object != 99 {
		t.Errorf("Expected Object 99, but got %d", result.Object)
	}
}
