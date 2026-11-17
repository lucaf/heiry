package heiry

import (
	"os/exec"
	"strings"
	"testing"
)

func TestM4ExternalTypedDeclarationCompilation(t *testing.T) {
	positive := exec.Command("go", "test", "./testdata/m4positive")
	if output, err := positive.CombinedOutput(); err != nil {
		t.Fatalf("positive external declaration: %v\n%s", err, output)
	}
	negative := exec.Command("go", "test", "./testdata/m4negative")
	output, err := negative.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "does not implement") || !strings.Contains(string(output), "Read") {
		t.Fatalf("wrong-provider compilation: %v\n%s", err, output)
	}
}
