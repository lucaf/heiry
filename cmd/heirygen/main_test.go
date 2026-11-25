package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const spec = "package main\n\ntype (\n\tleaf struct{}\n\tP    interface{}\n\tQ    interface{}\n)\n\n//heiry:spec\nconst s = `\nhierarchy shape leaf\nlevel leaf public=P private=Q\n`\n"

func dirWith(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func heirygen(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	exit := run(args, &stdout, &stderr)
	return exit, stdout.String(), stderr.String()
}

// The exit statuses of specification S10: 0 success, 1 outputs not up to
// date in check mode, 2 usage, 3 generation or checking failed.
func TestExitStatuses(t *testing.T) {
	broken := map[string]string{"main.go": strings.Replace(spec, "level leaf", "levle leaf", 1)}
	for _, tc := range []struct {
		name   string
		files  map[string]string
		args   func(dir string) []string
		exit   int
		stderr string
	}{
		{"generate", map[string]string{"main.go": spec}, func(d string) []string { return []string{d} }, 0, ""},
		{"check, not generated", map[string]string{"main.go": spec}, func(d string) []string { return []string{"-check", d} }, 1, "main.heirygen.go: missing; run go generate"},
		{"generate, invalid specification", broken, func(d string) []string { return []string{d} }, 3, `heirygen: `},
		{"check, invalid specification", broken, func(d string) []string { return []string{"-check", d} }, 3, `unknown keyword "levle"`},
		{"no such directory", nil, func(d string) []string { return []string{filepath.Join(d, "absent")} }, 3, "heirygen: "},
		{"two directories", nil, func(d string) []string { return []string{d, d} }, 2, "usage: heirygen"},
		{"unknown flag", nil, func(d string) []string { return []string{"-x", d} }, 2, "flag provided but not defined"},
		// Asking for help is not a usage error.
		{"help", nil, func(d string) []string { return []string{"-h"} }, 0, "usage: heirygen"},
		{"help, long form", nil, func(d string) []string { return []string{"-help", d} }, 0, "usage: heirygen"},
		{"comment specifications are not a mode", nil, func(d string) []string { return []string{"-comments", d} }, 2, "flag provided but not defined: -comments"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := dirWith(t, tc.files)
			exit, stdout, stderr := heirygen(tc.args(dir)...)
			if exit != tc.exit || !strings.Contains(stderr, tc.stderr) {
				t.Fatalf("exit %d, want %d; stderr %q, want it to contain %q", exit, tc.exit, stderr, tc.stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout %q, want nothing", stdout)
			}
		})
	}
}

func TestVerboseAndCheckAfterGeneration(t *testing.T) {
	dir := dirWith(t, map[string]string{"main.go": spec})
	if exit, stdout, stderr := heirygen("-v", dir); exit != 0 || stdout != "wrote main.heirygen.go\n" || stderr != "" {
		t.Fatalf("exit %d stdout %q stderr %q", exit, stdout, stderr)
	}
	if exit, stdout, stderr := heirygen("-check", dir); exit != 0 || stdout+stderr != "" {
		t.Fatalf("check: exit %d stdout %q stderr %q", exit, stdout, stderr)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if exit, stdout, _ := heirygen("-v", dir); exit != 0 || stdout != "removed main.heirygen.go\n" {
		t.Fatalf("exit %d stdout %q", exit, stdout)
	}
}

// -version names the module the command was built from, which is how a
// consumer's go.mod selects the generator version.
func TestVersion(t *testing.T) {
	exit, stdout, stderr := heirygen("-version")
	if exit != 0 || !strings.HasPrefix(stdout, "heirygen github.com/lucaf/heiry ") || stderr != "" {
		t.Fatalf("exit %d stdout %q stderr %q", exit, stdout, stderr)
	}
}
