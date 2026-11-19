package generator

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// A consumer runs the command through go generate, either with go run and
// the package path, or with go tool after recording it as a tool of the
// module. Both select the heiry version that go.mod requires.
func TestGoGenerateWorkflow(t *testing.T) {
	for _, tc := range []struct {
		name, directive, goMod string
		tool                   []string
	}{
		{name: "go run", directive: "//go:generate go run github.com/lucaf/heiry/cmd/heirygen", tool: []string{"run", "github.com/lucaf/heiry/cmd/heirygen"}},
		{name: "go tool", directive: "//go:generate go tool heirygen", goMod: "\ntool github.com/lucaf/heiry/cmd/heirygen\n", tool: []string{"tool", "heirygen"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := strings.Replace(fullApp, "package main\n", "package main\n\n"+tc.directive+"\n", 1)
			dir := tempModule(t, map[string]string{"main.go": app, "probe.go": fullProbe})
			if tc.goMod != "" {
				write(t, dir, "go.mod", read(t, dir, "go.mod")+tc.goMod)
			}

			// Ordinary commands do not run the generator.
			buildFails(t, dir, "undefined: recordsInit")
			if output, err := goTool(t, dir, nil, "generate", "./..."); err != nil {
				t.Fatalf("go generate:\n%s", output)
			}
			mustBuild(t, dir)
			if got := mustRun(t, dir); !strings.Contains(got, "exposes Read: true Audit: true Seal: false") {
				t.Fatalf("output = %q", got)
			}

			check := append(tc.tool, "-check", ".")
			if output, err := goTool(t, dir, nil, check...); err != nil || output != "" {
				t.Fatalf("check: %v\n%s", err, output)
			}
			edit(t, dir, "main.go", "wrapper view\n", "wrapper view field=object\n")
			// go run reports the command's status itself; go tool passes it on.
			output, err := goTool(t, dir, nil, check...)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 && !strings.Contains(output, "exit status 1") || !strings.Contains(output, "main.heirygen.go: differs from the expected output; run go generate") {
				t.Fatalf("check on a stale output: %v\n%s", err, output)
			}

			version := append(tc.tool, "-version")
			if output, err := goTool(t, dir, nil, version...); err != nil || !strings.HasPrefix(output, "heirygen github.com/lucaf/heiry v0.0.0 => ") {
				t.Fatalf("version: %v\n%s", err, output)
			}
		})
	}
}
