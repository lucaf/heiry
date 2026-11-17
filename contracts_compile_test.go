package heiry

import (
	"os/exec"
	"strings"
	"testing"
)

func TestStage4ExternalCompilationBoundary(t *testing.T) {
	positive := exec.Command("go", "test", "./testdata/stage4positive")
	if output, err := positive.CombinedOutput(); err != nil {
		t.Fatalf("positive external consumer failed: %v\n%s", err, output)
	}
	negative := exec.Command("go", "test", "./testdata/stage4negative")
	output, err := negative.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "has no field or method Secret") {
		t.Fatalf("negative compilation = %v\n%s", err, output)
	}
}
