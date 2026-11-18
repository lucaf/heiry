package heiry

import (
	"os/exec"
	"strings"
	"testing"
)

func TestM8ExternalComponentFixtures(t *testing.T) {
	positive := exec.Command("go", "test", "./testdata/m8positive")
	if output, err := positive.CombinedOutput(); err != nil {
		t.Fatalf("positive component fixture: %v\n%s", err, output)
	}
	for _, tc := range []struct {
		path, expected string
	}{{"./testdata/m8negativepointer", "does not match *Provider"}, {"./testdata/m8negativebind", "does not implement"}} {
		negative := exec.Command("go", "test", tc.path)
		output, err := negative.CombinedOutput()
		if err == nil || !strings.Contains(string(output), tc.expected) {
			t.Fatalf("%s diagnostic: %v\n%s", tc.path, err, output)
		}
	}
}
