package comparators

import (
	"reflect"
	"testing"
)

func TestVersionsEqual(t *testing.T) {
	lhs := "5.6.16"
	rhs := "5.6.16"

	comparison := CompareVersions(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != 0 {
		t.Errorf("Expected %q to be same as %q", rhs, lhs)
	}
}

func TestLhsVersionNewer(t *testing.T) {
	lhs := "6.8.1"
	rhs := "5.6.16"

	comparison := CompareVersions(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestRhsVersionNewer(t *testing.T) {
	lhs := "5.6.16"
	rhs := "6.8.1"

	comparison := CompareVersions(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != 1 {
		t.Errorf("Expected %q to be newer than %q", rhs, lhs)
	}
}

func TestRhsReleaseVersionMissing(t *testing.T) {
	lhs := "6.8.1"
	rhs := "6.8"

	comparison := CompareVersions(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestRhsMinorVersionMissing(t *testing.T) {
	lhs := "6.8.1"
	rhs := "6"

	comparison := CompareVersions(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestLhsReleaseVersionNewer(t *testing.T) {
	lhs := "6.8.1"
	rhs := "6.8.0"

	comparison := CompareVersions(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestLhsMinorVersionNewer(t *testing.T) {
	lhs := "6.8.1"
	rhs := "6.0"

	comparison := CompareVersions(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestValidVersionArray(t *testing.T) {
	version := "6.8.1"
	expectedArray := []int{6, 8, 1}

	actualArray := buildVersionArray(version)

	if !reflect.DeepEqual(expectedArray, actualArray) {
		t.Errorf("Expected: %v \nActual: %v", expectedArray, actualArray)
	}
}

func TestInvalidVersionArray(t *testing.T) {
	version := "6.8.a.0"
	expectedArray := []int{6, 8}

	actualArray := buildVersionArray(version)

	if !reflect.DeepEqual(expectedArray, actualArray) {
		t.Errorf("Expected: %v \nActual: %v", expectedArray, actualArray)
	}
}
