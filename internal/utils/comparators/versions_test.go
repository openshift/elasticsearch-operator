package comparators

import (
	"reflect"
	"testing"
)

func TestVersionsEqual(t *testing.T) {
	lhs := []int{5, 6, 16}
	rhs := []int{5, 6, 16}

	comparison := CompareVersionArrays(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != 0 {
		t.Errorf("Expected %q to be same as %q", rhs, lhs)
	}
}

func TestLhsVersionNewer(t *testing.T) {
	lhs := []int{6, 8, 1}
	rhs := []int{5, 6, 16}

	comparison := CompareVersionArrays(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestRhsVersionNewer(t *testing.T) {
	lhs := []int{5, 6, 16}
	rhs := []int{6, 8, 1}

	comparison := CompareVersionArrays(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != 1 {
		t.Errorf("Expected %q to be newer than %q", rhs, lhs)
	}
}

func TestRhsReleaseVersionMissing(t *testing.T) {
	lhs := []int{6, 8, 1}
	rhs := []int{6, 8}

	comparison := CompareVersionArrays(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestRhsMinorVersionMissing(t *testing.T) {
	lhs := []int{6, 8, 1}
	rhs := []int{6}

	comparison := CompareVersionArrays(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestLhsReleaseVersionNewer(t *testing.T) {
	lhs := []int{6, 8, 1}
	rhs := []int{6, 8, 0}

	comparison := CompareVersionArrays(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestLhsMinorVersionNewer(t *testing.T) {
	lhs := []int{6, 8, 1}
	rhs := []int{6, 0}

	comparison := CompareVersionArrays(lhs, rhs)

	// if lhs is newer we expect -1
	// 0 if they're the same
	// 1 if rhs is newer
	if comparison != -1 {
		t.Errorf("Expected %q to be newer than %q", lhs, rhs)
	}
}

func TestValidVersionArray(t *testing.T) {
	version := Version("6.8.1")
	expectedArray := []int{6, 8, 1}

	actualArray, err := version.ToArray()
	if err != nil {
		t.Errorf("Expected no error. Got %v", err)
	}

	if !reflect.DeepEqual(expectedArray, actualArray) {
		t.Errorf("Expected: %v \nActual: %v", expectedArray, actualArray)
	}
}

func TestInvalidVersionArray(t *testing.T) {
	version := Version("6.8.a.0")

	actualArray, err := version.ToArray()

	if err == nil {
		t.Error("Expected error. Got no error.")
	}

	if len(actualArray) != 0 {
		t.Errorf("Expected empty array. Got: %v", actualArray)
	}
}
