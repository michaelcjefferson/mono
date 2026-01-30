package assert

import (
	"strings"
	"testing"
)

// Equal is declared as a generic function so that it is capable of processing whichever data types "actual" and "expected" are, as long as both are of the same type.
func Equal[T comparable](t *testing.T, actual, expected T) {
	// This line tells go that this is a test helper function, so the error message that prints out will include the filename and line number not of this t.Errorf(), but of the actual testing function that called Equal().
	t.Helper()

	if actual != expected {
		t.Errorf("got: %v; want: %v", actual, expected)
	}
}

func StringContains(t *testing.T, actual, expectedSubstring string) {
	t.Helper()

	if !strings.Contains(actual, expectedSubstring) {
		t.Errorf("got: %q; expected to contain: %q", actual, expectedSubstring)
	}
}

func NilError(t *testing.T, actual error) {
	t.Helper()

	if actual != nil {
		t.Errorf("got: %v; expected: nil", actual)
	}
}
