package heiry

import (
	"os/exec"
	"strings"
	"testing"
)

func TestStage5NodeCopyVetFixture(t *testing.T) {
	cmd := exec.Command("go", "vet", "./testdata/stage5vet")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "copies lock value") {
		t.Fatalf("negative vet = %v\n%s", err, output)
	}
}
