package heiry

import (
	"os/exec"
	"strings"
	"testing"
)

// The positive fixture is the control: it proves the new forms compile and
// run from an external package, so a negative fixture cannot pass because of
// a missing module, a cache problem, or a syntax accident.
func TestErgonomicsExternalFixtures(t *testing.T) {
	positive := exec.Command("go", "test", "./testdata/e1positive")
	if output, err := positive.CombinedOutput(); err != nil {
		t.Fatalf("positive fixture: %v\n%s", err, output)
	}
	for _, tc := range []struct {
		path, expected string
	}{
		{"./testdata/e1negativeshadow", "cannot index t.Parent"},
	} {
		negative := exec.Command("go", "test", tc.path)
		output, err := negative.CombinedOutput()
		if err == nil || !strings.Contains(string(output), tc.expected) {
			t.Fatalf("%s diagnostic: %v\n%s", tc.path, err, output)
		}
	}
}
