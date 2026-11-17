package heiry

import (
	"os/exec"
	"strings"
	"testing"
)

func TestM6ExternalFluentCompilationFixtures(t *testing.T) {
	positive := exec.Command("go", "test", "./testdata/m6positive")
	if output, err := positive.CombinedOutput(); err != nil {
		t.Fatalf("positive grouped public lifecycle: %v\n%s", err, output)
	}
	for _, tt := range []struct {
		name, path  string
		diagnostics []string
	}{
		{"typed individual binding", "./testdata/m6negativebind", []string{"does not implement", "missing method Read"}},
		{"group requires pointer", "./testdata/m6negativepointer", []string{"does not match *Provider", "cannot infer Provider"}},
		{"group requires tokens", "./testdata/m6negativetoken", []string{"cannot use 7", "OperationToken"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command("go", "test", tt.path)
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("expected compilation failure:\n%s", output)
			}
			for _, diagnostic := range tt.diagnostics {
				if !strings.Contains(string(output), diagnostic) {
					t.Fatalf("expected %q compilation diagnostic: %v\n%s", diagnostic, err, output)
				}
			}
		})
	}
}
